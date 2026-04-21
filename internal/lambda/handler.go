package lambda

import (
	"context"
	"fmt"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/crypto"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/services/clusteriam"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/services/clustervpc"
)

// Event represents the Lambda function input
type Event struct {
	Action         string `json:"action"`
	ClusterName    string `json:"cluster_name"`
	OIDCIssuerURL  string `json:"oidc_issuer_url"`
	OIDCThumbprint string `json:"oidc_thumbprint"`
	// VPC parameters
	VpcCidr            string   `json:"vpc_cidr,omitempty"`
	PublicSubnetCidrs  []string `json:"public_subnet_cidrs,omitempty"`
	PrivateSubnetCidrs []string `json:"private_subnet_cidrs,omitempty"`
	AvailabilityZones  []string `json:"availability_zones,omitempty"`
	SingleNatGateway   bool     `json:"single_nat_gateway,omitempty"`
}

// Response represents the Lambda function output
type Response struct {
	Action  string            `json:"action"`
	StackID string            `json:"stack_id,omitempty"`
	Outputs map[string]string `json:"outputs"`
	Error   string            `json:"error,omitempty"`
}

// Handler is the Lambda function handler
func Handler(ctx context.Context, event Event) (Response, error) {
	fmt.Printf("Received event: action=%s cluster_name=%s\n", event.Action, event.ClusterName)

	switch event.Action {
	case "apply-cluster-iam":
		return applyClusterIAM(ctx, event)
	case "delete-cluster-iam":
		return deleteClusterIAM(ctx, event)
	case "apply-cluster-vpc":
		return applyClusterVPC(ctx, event)
	case "delete-cluster-vpc":
		return deleteClusterVPC(ctx, event)
	default:
		return Response{}, fmt.Errorf("unknown action: %s", event.Action)
	}
}

// applyClusterIAM applies the cluster IAM CloudFormation template
func applyClusterIAM(ctx context.Context, event Event) (Response, error) {
	if event.ClusterName == "" {
		return Response{}, fmt.Errorf("cluster_name is required")
	}
	fmt.Println("Applying cluster IAM CloudFormation template...")

	// Derive OIDC issuer domain if URL is provided
	var oidcIssuerDomain string
	if event.OIDCIssuerURL != "" {
		var err error
		oidcIssuerDomain, err = crypto.GetOIDCIssuerDomain(event.OIDCIssuerURL)
		if err != nil {
			return Response{}, fmt.Errorf("failed to parse oidc_issuer_url: %w", err)
		}
	}

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return Response{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Call service layer
	result, err := clusteriam.CreateIAM(ctx, &clusteriam.CreateIAMRequest{
		ClusterName:      event.ClusterName,
		OIDCIssuerDomain: oidcIssuerDomain,
		AWSConfig:        cfg,
	})
	if err != nil {
		return Response{}, fmt.Errorf("failed to create IAM: %w", err)
	}

	fmt.Printf("Stack created successfully: %s\n", result.StackID)

	return Response{
		Action:  "apply-cluster-iam",
		StackID: result.StackID,
		Outputs: result.Outputs,
	}, nil
}

// deleteClusterIAM deletes the cluster IAM CloudFormation stack
func deleteClusterIAM(ctx context.Context, event Event) (Response, error) {
	if event.ClusterName == "" {
		return Response{}, fmt.Errorf("cluster_name is required")
	}

	fmt.Printf("Deleting cluster IAM CloudFormation stack for: %s\n", event.ClusterName)

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return Response{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Call service layer
	err = clusteriam.DeleteIAM(ctx, &clusteriam.DeleteIAMRequest{
		ClusterName: event.ClusterName,
		AWSConfig:   cfg,
	})
	if err != nil {
		return Response{}, fmt.Errorf("failed to delete IAM: %w", err)
	}

	fmt.Printf("Stack deleted successfully: rosa-%s-iam\n", event.ClusterName)

	return Response{
		Action:  "delete-cluster-iam",
		Outputs: map[string]string{"status": "deleted"},
	}, nil
}

// applyClusterVPC applies the cluster VPC CloudFormation template
func applyClusterVPC(ctx context.Context, event Event) (Response, error) {
	if event.ClusterName == "" {
		return Response{}, fmt.Errorf("cluster_name is required")
	}

	fmt.Println("Applying cluster VPC CloudFormation template...")

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return Response{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Call service layer
	result, err := clustervpc.CreateVPC(ctx, &clustervpc.CreateVPCRequest{
		ClusterName:        event.ClusterName,
		VpcCidr:            event.VpcCidr,
		PublicSubnetCidrs:  event.PublicSubnetCidrs,
		PrivateSubnetCidrs: event.PrivateSubnetCidrs,
		AvailabilityZones:  event.AvailabilityZones,
		SingleNatGateway:   event.SingleNatGateway,
		AWSConfig:          cfg,
	})
	if err != nil {
		return Response{}, fmt.Errorf("failed to create VPC: %w", err)
	}

	fmt.Printf("Stack created successfully: %s\n", result.StackID)

	return Response{
		Action:  "apply-cluster-vpc",
		StackID: result.StackID,
		Outputs: result.Outputs,
	}, nil
}

// deleteClusterVPC deletes the cluster VPC CloudFormation stack
func deleteClusterVPC(ctx context.Context, event Event) (Response, error) {
	if event.ClusterName == "" {
		return Response{}, fmt.Errorf("cluster_name is required")
	}

	fmt.Printf("Deleting cluster VPC CloudFormation stack for: %s\n", event.ClusterName)

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return Response{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Call service layer
	err = clustervpc.DeleteVPC(ctx, &clustervpc.DeleteVPCRequest{
		ClusterName: event.ClusterName,
		AWSConfig:   cfg,
	})
	if err != nil {
		return Response{}, fmt.Errorf("failed to delete VPC: %w", err)
	}

	fmt.Printf("Stack deleted successfully: rosa-%s-vpc\n", event.ClusterName)

	return Response{
		Action:  "delete-cluster-vpc",
		Outputs: map[string]string{"status": "deleted"},
	}, nil
}

// Start starts the Lambda handler
func Start() {
	lambda.Start(Handler)
}
