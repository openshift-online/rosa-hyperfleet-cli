package clusteroidc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws/cloudformation"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/cloudformation/templates"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/crypto"
)

type CreateOIDCRequest struct {
	ClusterName    string
	OIDCIssuerURL  string
	OIDCThumbprint string // optional — fetched automatically if empty
	NoWait         bool
	AWSConfig      aws.Config
}

type CreateOIDCResponse struct {
	StackID string
	Outputs map[string]string
}

type DeleteOIDCRequest struct {
	ClusterName string
	NoWait      bool
	AWSConfig   aws.Config
}

// CreateOIDC creates the OIDC provider stack and updates the IAM stack trust policies.
func CreateOIDC(ctx context.Context, req *CreateOIDCRequest) (*CreateOIDCResponse, error) {
	if !strings.HasPrefix(req.OIDCIssuerURL, "https://") {
		return nil, fmt.Errorf("OIDC issuer URL must start with https://")
	}

	// Fetch thumbprint if not provided
	thumbprint := req.OIDCThumbprint
	if thumbprint == "" {
		var err error
		thumbprint, err = crypto.GetOIDCThumbprint(ctx, req.OIDCIssuerURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch TLS thumbprint: %w", err)
		}
	}

	// Derive issuer domain for IAM stack update
	oidcIssuerDomain, err := crypto.GetOIDCIssuerDomain(req.OIDCIssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OIDC issuer URL: %w", err)
	}

	templateBody, err := templates.Read("cluster-oidc.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read template: %w", err)
	}

	cfnClient := cloudformation.NewClient(req.AWSConfig)
	oidcStackName := fmt.Sprintf("rosa-%s-oidc", req.ClusterName)

	cfParams := &cloudformation.CreateStackParams{
		StackName:    oidcStackName,
		TemplateBody: templateBody,
		Parameters: map[string]string{
			"ClusterName":    req.ClusterName,
			"OIDCIssuerURL":  req.OIDCIssuerURL,
			"OIDCThumbprint": thumbprint,
		},
		Tags: []types.Tag{
			{Key: aws.String("Cluster"), Value: aws.String(req.ClusterName)},
			{Key: aws.String("ManagedBy"), Value: aws.String("rosactl")},
			{Key: aws.String("red-hat-managed"), Value: aws.String("true")},
		},
		WaitTimeout: 5 * time.Minute,
		NoWait:      req.NoWait,
	}

	// Create (or update) the OIDC provider stack
	output, err := cfnClient.CreateStack(ctx, cfParams)
	if err != nil {
		var alreadyExists *cloudformation.StackAlreadyExistsError
		if errors.As(err, &alreadyExists) {
			output, err = updateOIDCStack(ctx, cfnClient, req, oidcStackName, templateBody, thumbprint)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("failed to create OIDC stack: %w", err)
		}
	}

	// Update the IAM stack trust policies with the real issuer domain
	iamStackName := fmt.Sprintf("rosa-%s-iam", req.ClusterName)
	if err := updateIAMTrustPolicies(ctx, cfnClient, req.ClusterName, iamStackName, oidcIssuerDomain, req.NoWait); err != nil {
		return nil, fmt.Errorf("OIDC provider created but failed to update IAM trust policies: %w", err)
	}

	return &CreateOIDCResponse{
		StackID: output.StackID,
		Outputs: output.Outputs,
	}, nil
}

func updateOIDCStack(ctx context.Context, cfnClient *cloudformation.Client, req *CreateOIDCRequest, stackName, templateBody, thumbprint string) (*cloudformation.StackOutput, error) {
	params := &cloudformation.UpdateStackParams{
		StackName:    stackName,
		TemplateBody: templateBody,
		Parameters: map[string]string{
			"ClusterName":    req.ClusterName,
			"OIDCIssuerURL":  req.OIDCIssuerURL,
			"OIDCThumbprint": thumbprint,
		},
		WaitTimeout: 5 * time.Minute,
		NoWait:      req.NoWait,
	}

	output, err := cfnClient.UpdateStack(ctx, params)
	if err != nil {
		var noChanges *cloudformation.NoChangesError
		if errors.As(err, &noChanges) {
			return cfnClient.GetStackOutputs(ctx, stackName)
		}
		return nil, fmt.Errorf("failed to update OIDC stack: %w", err)
	}

	return output, nil
}

// updateIAMTrustPolicies updates the IAM stack with the real OIDC issuer domain.
// If the IAM stack does not exist yet (e.g. when using the deferred-IAM flow where
// cluster-iam is created after cluster-oidc), the update is skipped.
func updateIAMTrustPolicies(ctx context.Context, cfnClient *cloudformation.Client, clusterName, iamStackName, oidcIssuerDomain string, noWait bool) error {
	params := &cloudformation.UpdateStackParams{
		StackName:           iamStackName,
		UsePreviousTemplate: true,
		Parameters: map[string]string{
			"ClusterName":      clusterName,
			"OIDCIssuerDomain": oidcIssuerDomain,
		},
		Capabilities: []types.Capability{
			types.CapabilityCapabilityIam,
			types.CapabilityCapabilityNamedIam,
		},
		WaitTimeout: 15 * time.Minute,
		NoWait:      noWait,
	}

	_, err := cfnClient.UpdateStack(ctx, params)
	if err != nil {
		var noChanges *cloudformation.NoChangesError
		if errors.As(err, &noChanges) {
			return nil // domain was already set correctly
		}
		var notFound *cloudformation.StackNotFoundError
		if errors.As(err, &notFound) {
			fmt.Printf("IAM stack %s not found — skipping trust policy update (create it with 'rosactl cluster-iam create --oidc-issuer-url')\n", iamStackName)
			return nil
		}
		return err
	}

	return nil
}

// DeleteOIDC deletes the OIDC provider stack. The IAM stack is left unchanged.
func DeleteOIDC(ctx context.Context, req *DeleteOIDCRequest) error {
	cfnClient := cloudformation.NewClient(req.AWSConfig)
	stackName := fmt.Sprintf("rosa-%s-oidc", req.ClusterName)

	err := cfnClient.DeleteStack(ctx, stackName, 5*time.Minute, req.NoWait)
	if err != nil {
		var notFound *cloudformation.StackNotFoundError
		if errors.As(err, &notFound) {
			return nil
		}
		return fmt.Errorf("failed to delete OIDC stack: %w", err)
	}

	return nil
}
