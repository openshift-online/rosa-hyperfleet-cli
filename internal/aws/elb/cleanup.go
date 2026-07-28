package elb

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv1 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
)

// CleanVPCLoadBalancers deletes all classic ELBs and ALBs/NLBs in vpcID before
// security group cleanup. The Kubernetes CCM creates classic ELBs for LoadBalancer
// services; they hold SG dependencies (k8s-elb-* naming) that prevent SG deletion
// with DependencyViolation. ELBs must be removed before ec2.CleanVPCForDeletion.
func CleanVPCLoadBalancers(ctx context.Context, cfg aws.Config, vpcID string) error {
	if err := deleteClassicELBs(ctx, elbv1.NewFromConfig(cfg), vpcID); err != nil {
		return fmt.Errorf("classic ELBs: %w", err)
	}
	if err := deleteALBsNLBs(ctx, elbv2.NewFromConfig(cfg), vpcID); err != nil {
		return fmt.Errorf("ALBs/NLBs: %w", err)
	}
	return nil
}

func deleteClassicELBs(ctx context.Context, client *elbv1.Client, vpcID string) error {
	var names []string

	paginator := elbv1.NewDescribeLoadBalancersPaginator(client, &elbv1.DescribeLoadBalancersInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("describing classic ELBs: %w", err)
		}
		for _, lb := range page.LoadBalancerDescriptions {
			if aws.ToString(lb.VPCId) == vpcID {
				names = append(names, aws.ToString(lb.LoadBalancerName))
			}
		}
	}

	for _, name := range names {
		log.Printf("  deleting classic ELB %s", name)
		if _, err := client.DeleteLoadBalancer(ctx, &elbv1.DeleteLoadBalancerInput{
			LoadBalancerName: aws.String(name),
		}); err != nil {
			return fmt.Errorf("delete classic ELB %s: %w", name, err)
		}
	}

	if len(names) > 0 {
		log.Printf("  deleted %d classic ELB(s)", len(names))
	}
	return nil
}

func deleteALBsNLBs(ctx context.Context, client *elbv2.Client, vpcID string) error {
	var arns []string

	paginator := elbv2.NewDescribeLoadBalancersPaginator(client, &elbv2.DescribeLoadBalancersInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("describing ALBs/NLBs: %w", err)
		}
		for _, lb := range page.LoadBalancers {
			if aws.ToString(lb.VpcId) == vpcID {
				arns = append(arns, aws.ToString(lb.LoadBalancerArn))
			}
		}
	}

	waiter := elbv2.NewLoadBalancersDeletedWaiter(client)
	for _, arn := range arns {
		log.Printf("  deleting ALB/NLB %s", arn)
		if _, err := client.DeleteLoadBalancer(ctx, &elbv2.DeleteLoadBalancerInput{
			LoadBalancerArn: aws.String(arn),
		}); err != nil {
			return fmt.Errorf("delete ALB/NLB %s: %w", arn, err)
		}
		if err := waiter.Wait(ctx, &elbv2.DescribeLoadBalancersInput{
			LoadBalancerArns: []string{arn},
		}, 5*time.Minute); err != nil {
			return fmt.Errorf("waiting for ALB/NLB %s deletion: %w", arn, err)
		}
	}

	if len(arns) > 0 {
		log.Printf("  deleted %d ALB/NLB(s)", len(arns))
	}
	return nil
}
