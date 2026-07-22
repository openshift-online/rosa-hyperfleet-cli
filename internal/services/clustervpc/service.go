package clustervpc

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws/cloudformation"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/cloudformation/templates"
)

type CreateVPCRequest struct {
	ClusterName        string
	VpcCidr            string
	PublicSubnetCidrs  []string
	PrivateSubnetCidrs []string
	AvailabilityZones  []string
	SingleNatGateway   bool
	AWSConfig          aws.Config
}

type CreateVPCResponse struct {
	StackID string
	Outputs map[string]string
}

type DeleteVPCRequest struct {
	ClusterName string
	AWSConfig   aws.Config
}

var azPattern = regexp.MustCompile(`^[a-z]{2}-[a-z]+-\d[a-z]$`)

// CreateVPC creates cluster VPC resources via CloudFormation
func CreateVPC(ctx context.Context, req *CreateVPCRequest) (*CreateVPCResponse, error) {
	if len(req.AvailabilityZones) < 1 {
		return nil, fmt.Errorf("at least 1 availability zone is required")
	}
	for _, az := range req.AvailabilityZones {
		if !azPattern.MatchString(az) {
			return nil, fmt.Errorf("invalid availability zone %q: expected format like \"us-east-1a\" (region prefix + letter suffix)", az)
		}
	}

	// Read CloudFormation template
	templateBody, err := templates.Read("cluster-vpc.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read template: %w", err)
	}

	// Create CloudFormation client
	cfnClient := cloudformation.NewClient(req.AWSConfig)

	// Prepare stack parameters
	stackName := fmt.Sprintf("rosa-%s-vpc", req.ClusterName)
	params := map[string]string{
		"ClusterName":      req.ClusterName,
		"SingleNatGateway": fmt.Sprintf("%t", req.SingleNatGateway),
	}

	// Add optional parameters
	if req.VpcCidr != "" {
		params["VpcCidr"] = req.VpcCidr
	}
	// Split subnet CIDRs into individual parameters (template expects individual params, not lists)
	if len(req.PublicSubnetCidrs) > 0 {
		params["PublicSubnetCidr1"] = req.PublicSubnetCidrs[0]
	}
	if len(req.PublicSubnetCidrs) > 1 {
		params["PublicSubnetCidr2"] = req.PublicSubnetCidrs[1]
	}
	if len(req.PublicSubnetCidrs) > 2 {
		params["PublicSubnetCidr3"] = req.PublicSubnetCidrs[2]
	}
	if len(req.PrivateSubnetCidrs) > 0 {
		params["PrivateSubnetCidr1"] = req.PrivateSubnetCidrs[0]
	}
	if len(req.PrivateSubnetCidrs) > 1 {
		params["PrivateSubnetCidr2"] = req.PrivateSubnetCidrs[1]
	}
	if len(req.PrivateSubnetCidrs) > 2 {
		params["PrivateSubnetCidr3"] = req.PrivateSubnetCidrs[2]
	}
	params["AvailabilityZone1"] = req.AvailabilityZones[0]
	if len(req.AvailabilityZones) > 1 {
		params["AvailabilityZone2"] = req.AvailabilityZones[1]
	}
	if len(req.AvailabilityZones) > 2 {
		params["AvailabilityZone3"] = req.AvailabilityZones[2]
	}

	createParams := &cloudformation.CreateStackParams{
		StackName:    stackName,
		TemplateBody: templateBody,
		Parameters:   params,
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
	}

	// Create stack
	output, err := cfnClient.CreateStack(ctx, createParams)
	if err != nil {
		// Check if stack already exists, try update instead
		var alreadyExistsErr *cloudformation.StackAlreadyExistsError
		if errors.As(err, &alreadyExistsErr) {
			return updateVPC(ctx, cfnClient, req, stackName, templateBody)
		}
		return nil, fmt.Errorf("failed to create stack: %w", err)
	}

	return &CreateVPCResponse{
		StackID: output.StackID,
		Outputs: output.Outputs,
	}, nil
}

func updateVPC(ctx context.Context, cfnClient *cloudformation.Client, req *CreateVPCRequest, stackName, templateBody string) (*CreateVPCResponse, error) {
	params := map[string]string{
		"ClusterName":      req.ClusterName,
		"SingleNatGateway": fmt.Sprintf("%t", req.SingleNatGateway),
	}

	// Add optional parameters
	if req.VpcCidr != "" {
		params["VpcCidr"] = req.VpcCidr
	}
	// Split subnet CIDRs into individual parameters (template expects individual params, not lists)
	if len(req.PublicSubnetCidrs) > 0 {
		params["PublicSubnetCidr1"] = req.PublicSubnetCidrs[0]
	}
	if len(req.PublicSubnetCidrs) > 1 {
		params["PublicSubnetCidr2"] = req.PublicSubnetCidrs[1]
	}
	if len(req.PublicSubnetCidrs) > 2 {
		params["PublicSubnetCidr3"] = req.PublicSubnetCidrs[2]
	}
	if len(req.PrivateSubnetCidrs) > 0 {
		params["PrivateSubnetCidr1"] = req.PrivateSubnetCidrs[0]
	}
	if len(req.PrivateSubnetCidrs) > 1 {
		params["PrivateSubnetCidr2"] = req.PrivateSubnetCidrs[1]
	}
	if len(req.PrivateSubnetCidrs) > 2 {
		params["PrivateSubnetCidr3"] = req.PrivateSubnetCidrs[2]
	}
	params["AvailabilityZone1"] = req.AvailabilityZones[0]
	if len(req.AvailabilityZones) > 1 {
		params["AvailabilityZone2"] = req.AvailabilityZones[1]
	}
	if len(req.AvailabilityZones) > 2 {
		params["AvailabilityZone3"] = req.AvailabilityZones[2]
	}

	updateParams := &cloudformation.UpdateStackParams{
		StackName:    stackName,
		TemplateBody: templateBody,
		Parameters:   params,
		WaitTimeout:  15 * time.Minute,
	}

	output, err := cfnClient.UpdateStack(ctx, updateParams)
	if err != nil {
		var noChanges *cloudformation.NoChangesError
		if errors.As(err, &noChanges) {
			current, descErr := cfnClient.GetStackOutputs(ctx, stackName)
			if descErr != nil {
				return nil, descErr
			}
			return &CreateVPCResponse{
				StackID: current.StackID,
				Outputs: current.Outputs,
			}, nil
		}
		return nil, fmt.Errorf("failed to update stack: %w", err)
	}

	return &CreateVPCResponse{
		StackID: output.StackID,
		Outputs: output.Outputs,
	}, nil
}

// DeleteVPC deletes cluster VPC resources
func DeleteVPC(ctx context.Context, req *DeleteVPCRequest) error {
	// Create CloudFormation client
	cfnClient := cloudformation.NewClient(req.AWSConfig)

	// Delete stack
	stackName := fmt.Sprintf("rosa-%s-vpc", req.ClusterName)
	err := cfnClient.DeleteStack(ctx, stackName, 15*time.Minute)
	if err != nil {
		var notFound *cloudformation.StackNotFoundError
		if errors.As(err, &notFound) {
			return nil
		}
		return fmt.Errorf("failed to delete stack: %w", err)
	}

	return nil
}
