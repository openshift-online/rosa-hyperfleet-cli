package ec2

import (
	"context"
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
// CNI plugin) cause CloudFormation delete-stack to fail on the first attempt.
func CleanVPCForDeletion(ctx context.Context, cfg aws.Config, vpcID string) error {
	client := ec2.NewFromConfig(cfg)

	if err := deleteOrphanedENIs(ctx, client, vpcID); err != nil {
		return fmt.Errorf("cleaning ENIs: %w", err)
	}

	if err := deleteNonDefaultSecurityGroups(ctx, client, vpcID); err != nil {
		return fmt.Errorf("cleaning security groups: %w", err)
	}

	return nil
}

func deleteOrphanedENIs(ctx context.Context, client *ec2.Client, vpcID string) error {
	out, err := client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
		Filters: []types.Filter{
			{Name: aws.String("vpc-id"), Values: []string{vpcID}},
		},
	})
	if err != nil {
		return fmt.Errorf("describing ENIs: %w", err)
	}

	for _, eni := range out.NetworkInterfaces {
		eniID := aws.ToString(eni.NetworkInterfaceId)

		if eni.Attachment != nil && eni.Attachment.AttachmentId != nil {
			// Only attempt detach for ENIs we can force-detach.
			// ELB/Lambda-managed attachments will be cleaned up by their
			// owning service; skip them to avoid API errors.
			if eni.Attachment.DeleteOnTermination != nil && *eni.Attachment.DeleteOnTermination {
				log.Printf("  skipping ENI %s (managed attachment, will be cleaned by owner)", eniID)
				continue
			}

			log.Printf("  detaching ENI %s", eniID)
			_, err := client.DetachNetworkInterface(ctx, &ec2.DetachNetworkInterfaceInput{
				AttachmentId: eni.Attachment.AttachmentId,
				Force:        aws.Bool(true),
			})
			if err != nil {
				log.Printf("  warning: failed to detach ENI %s: %v (will still try delete)", eniID, err)
			} else {
				waitForENIAvailable(ctx, client, eniID)
			}
		}

		log.Printf("  deleting ENI %s", eniID)
		_, err := client.DeleteNetworkInterface(ctx, &ec2.DeleteNetworkInterfaceInput{
			NetworkInterfaceId: aws.String(eniID),
		})
		if err != nil {
			log.Printf("  warning: failed to delete ENI %s: %v", eniID, err)
		}
	}

	return nil
}

func waitForENIAvailable(ctx context.Context, client *ec2.Client, eniID string) {
	for i := 0; i < 12; i++ {
		time.Sleep(5 * time.Second)
		out, err := client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
			NetworkInterfaceIds: []string{eniID},
		})
		if err != nil {
			return
		}
		if len(out.NetworkInterfaces) > 0 && out.NetworkInterfaces[0].Status == types.NetworkInterfaceStatusAvailable {
			return
		}
	}
}

func deleteNonDefaultSecurityGroups(ctx context.Context, client *ec2.Client, vpcID string) error {
	out, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{Name: aws.String("vpc-id"), Values: []string{vpcID}},
		},
	})
	if err != nil {
		return fmt.Errorf("describing security groups: %w", err)
	}

	// First pass: revoke all rules so cross-references don't block deletion.
	for _, sg := range out.SecurityGroups {
		if aws.ToString(sg.GroupName) == "default" {
			continue
		}
		sgID := aws.ToString(sg.GroupId)

		if len(sg.IpPermissions) > 0 {
			_, err := client.RevokeSecurityGroupIngress(ctx, &ec2.RevokeSecurityGroupIngressInput{
				GroupId:       aws.String(sgID),
				IpPermissions: sg.IpPermissions,
			})
			if err != nil {
				log.Printf("  warning: failed to revoke ingress on SG %s: %v", sgID, err)
			}
		}
		if len(sg.IpPermissionsEgress) > 0 {
			_, err := client.RevokeSecurityGroupEgress(ctx, &ec2.RevokeSecurityGroupEgressInput{
				GroupId:       aws.String(sgID),
				IpPermissions: sg.IpPermissionsEgress,
			})
			if err != nil {
				log.Printf("  warning: failed to revoke egress on SG %s: %v", sgID, err)
			}
		}
	}

	// Second pass: delete the security groups.
	for _, sg := range out.SecurityGroups {
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
		}
	}

	return nil
}
