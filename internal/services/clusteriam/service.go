package clusteriam

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws/cloudformation"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/cloudformation/templates"
)

type CreateIAMRequest struct {
	ClusterName      string
	OIDCIssuerDomain string // optional — empty uses template default PENDING
	NoWait           bool
	AWSConfig        aws.Config
}

type CreateIAMResponse struct {
	StackID string
	Outputs map[string]string
}

type DeleteIAMRequest struct {
	ClusterName string
	NoWait      bool
	AWSConfig   aws.Config
}

// CreateIAM creates cluster IAM resources via CloudFormation
func CreateIAM(ctx context.Context, req *CreateIAMRequest) (*CreateIAMResponse, error) {
	// Read CloudFormation template
	templateBody, err := templates.Read("cluster-iam.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read template: %w", err)
	}

	// Create CloudFormation client
	cfnClient := cloudformation.NewClient(req.AWSConfig)

	// Prepare stack parameters
	stackName := fmt.Sprintf("rosa-%s-iam", req.ClusterName)
	params := &cloudformation.CreateStackParams{
		StackName:    stackName,
		TemplateBody: templateBody,
		Parameters:   map[string]string{"ClusterName": req.ClusterName},
		Capabilities: []types.Capability{
			types.CapabilityCapabilityIam,
			types.CapabilityCapabilityNamedIam,
		},
		Tags: []types.Tag{
			{
				Key:   aws.String("Cluster"),
				Value: aws.String(req.ClusterName),
			},
			{
				Key:   aws.String("ManagedBy"),
				Value: aws.String("rosactl"),
			},
			{
				Key:   aws.String("red-hat-managed"),
				Value: aws.String("true"),
			},
		},
		WaitTimeout: 15 * time.Minute,
		NoWait:      req.NoWait,
	}

	// Only pass OIDCIssuerDomain if provided — otherwise let the template default (PENDING) apply
	if req.OIDCIssuerDomain != "" {
		params.Parameters["OIDCIssuerDomain"] = req.OIDCIssuerDomain
	}

	// Create stack
	output, err := cfnClient.CreateStack(ctx, params)
	if err != nil {
		// Check if stack already exists, try update instead
		var alreadyExistsErr *cloudformation.StackAlreadyExistsError
		if errors.As(err, &alreadyExistsErr) {
			return updateIAM(ctx, cfnClient, req, stackName, templateBody)
		}
		return nil, fmt.Errorf("failed to create stack: %w", err)
	}

	return &CreateIAMResponse{
		StackID: output.StackID,
		Outputs: output.Outputs,
	}, nil
}

func updateIAM(ctx context.Context, cfnClient *cloudformation.Client, req *CreateIAMRequest, stackName, templateBody string) (*CreateIAMResponse, error) {
	params := &cloudformation.UpdateStackParams{
		StackName:    stackName,
		TemplateBody: templateBody,
		Parameters:   map[string]string{"ClusterName": req.ClusterName},
		Capabilities: []types.Capability{
			types.CapabilityCapabilityIam,
			types.CapabilityCapabilityNamedIam,
		},
		WaitTimeout: 15 * time.Minute,
		NoWait:      req.NoWait,
	}

	if req.OIDCIssuerDomain != "" {
		params.Parameters["OIDCIssuerDomain"] = req.OIDCIssuerDomain
	}

	output, err := cfnClient.UpdateStack(ctx, params)
	if err != nil {
		var noChanges *cloudformation.NoChangesError
		if errors.As(err, &noChanges) {
			current, descErr := cfnClient.GetStackOutputs(ctx, stackName)
			if descErr != nil {
				return nil, descErr
			}
			return &CreateIAMResponse{
				StackID: current.StackID,
				Outputs: current.Outputs,
			}, nil
		}
		return nil, fmt.Errorf("failed to update stack: %w", err)
	}

	return &CreateIAMResponse{
		StackID: output.StackID,
		Outputs: output.Outputs,
	}, nil
}

// DeleteIAM deletes cluster IAM resources
func DeleteIAM(ctx context.Context, req *DeleteIAMRequest) error {
	// Create CloudFormation client
	cfnClient := cloudformation.NewClient(req.AWSConfig)

	// Delete stack
	stackName := fmt.Sprintf("rosa-%s-iam", req.ClusterName)
	err := cfnClient.DeleteStack(ctx, stackName, 15*time.Minute, req.NoWait)
	if err != nil {
		var notFound *cloudformation.StackNotFoundError
		if errors.As(err, &notFound) {
			return nil
		}
		return fmt.Errorf("failed to delete stack: %w", err)
	}

	return nil
}
