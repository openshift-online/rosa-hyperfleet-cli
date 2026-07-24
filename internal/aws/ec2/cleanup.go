package ec2

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// CleanVPCForDeletion removes orphaned ENIs and non-default security groups
// from a VPC so that its CloudFormation stack can be deleted cleanly.
// Resources left behind by hosted cluster teardown (load balancer controllers,
// CNI plugin, VPC endpoints) cause CloudFormation delete-stack to fail.
//
// This is a temporary workaround for OCPBUGS-74960 where HyperShift's CPO
// fails to clean up VPC endpoint resources during HCP deletion.
func CleanVPCForDeletion(ctx context.Context, cfg aws.Config, vpcID string) error {
	client := ec2.NewFromConfig(cfg)

	if err := deleteOrphanedVPCEndpoints(ctx, client, vpcID); err != nil {
		return fmt.Errorf("cleaning VPC endpoints: %w", err)
	}

	if err := deleteNonDefaultSecurityGroups(ctx, client, vpcID); err != nil {
		return fmt.Errorf("cleaning security groups: %w", err)
	}

	return nil
}

func deleteNonDefaultSecurityGroups(ctx context.Context, client *ec2.Client, vpcID string) error {
	var securityGroups []types.SecurityGroup

	paginator := ec2.NewDescribeSecurityGroupsPaginator(client, &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{Name: aws.String("vpc-id"), Values: []string{vpcID}},
		},
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("describing security groups: %w", err)
		}
		securityGroups = append(securityGroups, page.SecurityGroups...)
	}

	var cleanupErr error
	for _, sg := range securityGroups {
		if aws.ToString(sg.GroupName) == "default" {
			continue
		}
		sgID := aws.ToString(sg.GroupId)
		log.Printf("  deleting security group %s (%s)", sgID, aws.ToString(sg.GroupName))
		_, err := client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
			GroupId: aws.String(sgID),
		})
		if err != nil {
			log.Printf("  warning: failed to delete SG %s: %v", sgID, err)
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("delete SG %s: %w", sgID, err))
		}
	}

	return cleanupErr
}

func deleteOrphanedVPCEndpoints(ctx context.Context, client *ec2.Client, vpcID string) error {
	out, err := client.DescribeVpcEndpoints(ctx, &ec2.DescribeVpcEndpointsInput{
		Filters: []types.Filter{
			{Name: aws.String("vpc-id"), Values: []string{vpcID}},
		},
	})
	if err != nil {
		return fmt.Errorf("describing VPC endpoints: %w", err)
	}

	var endpointIDs []string
	for _, ep := range out.VpcEndpoints {
		endpointIDs = append(endpointIDs, aws.ToString(ep.VpcEndpointId))
	}
	if len(endpointIDs) == 0 {
		return nil
	}

	log.Printf("  deleting %d VPC endpoint(s): %v", len(endpointIDs), endpointIDs)
	_, err = client.DeleteVpcEndpoints(ctx, &ec2.DeleteVpcEndpointsInput{
		VpcEndpointIds: endpointIDs,
	})
	if err != nil {
		return fmt.Errorf("deleting VPC endpoints: %w", err)
	}

	// Wait for endpoints to be fully deleted so their ENIs and SGs are released.
	log.Printf("  waiting for VPC endpoints to be fully deleted...")
	for attempt := 0; attempt < 60; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
		remaining, err := client.DescribeVpcEndpoints(ctx, &ec2.DescribeVpcEndpointsInput{
			VpcEndpointIds: endpointIDs,
		})
		if err != nil {
			// InvalidVpcEndpointId.NotFound means they're all gone.
			log.Printf("  VPC endpoints deleted")
			return nil
		}
		stillActive := 0
		for _, ep := range remaining.VpcEndpoints {
			if ep.State != types.StateDeleted {
				stillActive++
			}
		}
		if stillActive == 0 {
			log.Printf("  VPC endpoints deleted")
			return nil
		}
		log.Printf("  still waiting for %d VPC endpoint(s)...", stillActive)
	}

	return fmt.Errorf("timed out waiting for VPC endpoints to be deleted")
}

