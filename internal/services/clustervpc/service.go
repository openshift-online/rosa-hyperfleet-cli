package clustervpc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws/cloudformation"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws/ec2"
	route53cleanup "github.com/openshift-online/rosa-regional-platform-cli/internal/aws/route53"
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

// CreateVPC creates cluster VPC resources via CloudFormation
func CreateVPC(ctx context.Context, req *CreateVPCRequest) (*CreateVPCResponse, error) {
	// Validate required parameters
	if len(req.AvailabilityZones) < 1 {
		return nil, fmt.Errorf("at least 1 availability zone is required")
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

// DeleteVPC deletes cluster VPC resources. It pre-cleans orphaned ENIs and
// security groups left behind by hosted cluster teardown so that the
// CloudFormation stack delete succeeds on the first attempt.
func DeleteVPC(ctx context.Context, req *DeleteVPCRequest) error {
	cfnClient := cloudformation.NewClient(req.AWSConfig)
	stackName := fmt.Sprintf("rosa-%s-vpc", req.ClusterName)

	// Get the VPC ID from stack outputs so we can clean up orphaned resources.
	outputs, err := cfnClient.GetStackOutputs(ctx, stackName)
	if err != nil {
		var notFound *cloudformation.StackNotFoundError
		if errors.As(err, &notFound) {
			return nil
		}
		// Stack exists but we can't read outputs (e.g. rollback state).
		// Fall through to DeleteStack without cleanup -- no worse than before.
		log.Printf("warning: could not read stack outputs for %s: %v (skipping pre-cleanup)", stackName, err)
	} else {
		// Pre-clean orphaned resources left by HCP teardown (workaround for OCPBUGS-74960).
		if vpcID := outputs.Outputs["VpcId"]; vpcID != "" {
			log.Printf("pre-cleaning VPC %s before stack deletion (workaround for OCPBUGS-74960)", vpcID)
			if cleanErr := ec2.CleanVPCForDeletion(ctx, req.AWSConfig, vpcID); cleanErr != nil {
				log.Printf("warning: VPC pre-cleanup failed: %v (proceeding with stack delete)", cleanErr)
			}
		}
		if zoneID := outputs.Outputs["HypershiftLocalZoneId"]; zoneID != "" {
			log.Printf("pre-cleaning hosted zone %s before stack deletion (workaround for OCPBUGS-74960)", zoneID)
			if cleanErr := route53cleanup.CleanHostedZoneForDeletion(ctx, req.AWSConfig, zoneID); cleanErr != nil {
				log.Printf("warning: hosted zone pre-cleanup failed: %v (proceeding with stack delete)", cleanErr)
			}
		}
	}

	log.Printf("deleting CloudFormation stack %s", stackName)
	err = cfnClient.DeleteStack(ctx, stackName, 15*time.Minute)
	if err != nil {
		var notFound *cloudformation.StackNotFoundError
		if errors.As(err, &notFound) {
			return nil
		}
		return fmt.Errorf("failed to delete stack: %w", err)
	}

	return nil
}
