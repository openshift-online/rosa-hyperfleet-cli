package cluster

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/crypto"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/services/clusteriam"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/services/clustervpc"
	"github.com/spf13/cobra"
)

type deployOptions struct {
	clusterName        string
	region             string
	oidcIssuerURL      string
	vpcCidr            string
	publicSubnetCidrs  string
	privateSubnetCidrs string
	availabilityZones  string
	singleNatGateway   bool
}

func newDeployCommand() *cobra.Command {
	opts := &deployOptions{}

	cmd := &cobra.Command{
		Use:   "deploy CLUSTER_NAME",
		Short: "Deploy cluster IAM and VPC resources in parallel",
		Long: `Deploy both IAM and VPC resources for a ROSA hosted cluster simultaneously.

This command runs the cluster-iam and cluster-vpc create operations in parallel,
reducing total deployment time compared to running them sequentially.

Example:
  rosactl cluster deploy my-cluster \
    --oidc-issuer-url https://d1234.cloudfront.net/my-cluster \
    --region us-east-1 \
    --availability-zones us-east-1a,us-east-1b,us-east-1c`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.clusterName = args[0]
			return runDeploy(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.oidcIssuerURL, "oidc-issuer-url", "", "OIDC issuer URL from Management Cluster (required)")
	cmd.Flags().StringVar(&opts.region, "region", "", "AWS region (required)")
	cmd.Flags().StringVar(&opts.availabilityZones, "availability-zones", "", "Comma-separated availability zones, 3 required (required)")
	cmd.Flags().StringVar(&opts.vpcCidr, "vpc-cidr", "10.0.0.0/16", "CIDR block for the VPC")
	cmd.Flags().StringVar(&opts.publicSubnetCidrs, "public-subnet-cidrs", "10.0.101.0/24,10.0.102.0/24,10.0.103.0/24", "Comma-separated public subnet CIDRs")
	cmd.Flags().StringVar(&opts.privateSubnetCidrs, "private-subnet-cidrs", "10.0.0.0/19,10.0.32.0/19,10.0.64.0/19", "Comma-separated private subnet CIDRs")
	cmd.Flags().BoolVar(&opts.singleNatGateway, "single-nat-gateway", true, "Use single NAT gateway (true=cost savings, false=HA per-AZ)")

	cmd.MarkFlagRequired("oidc-issuer-url")
	cmd.MarkFlagRequired("region")
	cmd.MarkFlagRequired("availability-zones")

	return cmd
}

type deployResult struct {
	name string
	err  error
}

func runDeploy(ctx context.Context, opts *deployOptions) error {
	if !strings.HasPrefix(opts.oidcIssuerURL, "https://") {
		return fmt.Errorf("OIDC issuer URL must start with https://")
	}

	azs := strings.Split(opts.availabilityZones, ",")
	if len(azs) < 3 {
		return fmt.Errorf("at least 3 availability zones are required, got %d", len(azs))
	}

	publicSubnets := strings.Split(opts.publicSubnetCidrs, ",")
	privateSubnets := strings.Split(opts.privateSubnetCidrs, ",")

	fmt.Printf("Deploying cluster resources for: %s\n", opts.clusterName)
	fmt.Printf("  Region:             %s\n", opts.region)
	fmt.Printf("  OIDC Issuer:        %s\n", opts.oidcIssuerURL)
	fmt.Printf("  VPC CIDR:           %s\n", opts.vpcCidr)
	fmt.Printf("  Availability Zones: %s\n", opts.availabilityZones)
	fmt.Println()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(opts.region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	results := make(chan deployResult, 2)
	var wg sync.WaitGroup

	// IAM deployment goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()

		fmt.Println("[IAM] Fetching TLS thumbprint from OIDC issuer...")
		thumbprint, err := crypto.GetOIDCThumbprint(ctx, opts.oidcIssuerURL)
		if err != nil {
			results <- deployResult{"IAM", fmt.Errorf("failed to fetch TLS thumbprint: %w", err)}
			return
		}

		iamReq := &clusteriam.CreateIAMRequest{
			ClusterName:    opts.clusterName,
			OIDCIssuerURL:  opts.oidcIssuerURL,
			OIDCThumbprint: thumbprint,
			AWSConfig:      cfg,
		}

		fmt.Printf("[IAM] Creating CloudFormation stack: rosa-%s-iam\n", opts.clusterName)
		resp, err := clusteriam.CreateIAM(ctx, iamReq)
		if err != nil {
			results <- deployResult{"IAM", err}
			return
		}

		fmt.Printf("[IAM] Done. Stack ID: %s\n", resp.StackID)
		results <- deployResult{"IAM", nil}
	}()

	// VPC deployment goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()

		vpcReq := &clustervpc.CreateVPCRequest{
			ClusterName:        opts.clusterName,
			VpcCidr:            opts.vpcCidr,
			PublicSubnetCidrs:  publicSubnets,
			PrivateSubnetCidrs: privateSubnets,
			AvailabilityZones:  azs,
			SingleNatGateway:   opts.singleNatGateway,
			AWSConfig:          cfg,
		}

		fmt.Printf("[VPC] Creating CloudFormation stack: rosa-%s-vpc\n", opts.clusterName)
		resp, err := clustervpc.CreateVPC(ctx, vpcReq)
		if err != nil {
			results <- deployResult{"VPC", err}
			return
		}

		fmt.Printf("[VPC] Done. Stack ID: %s\n", resp.StackID)
		results <- deployResult{"VPC", nil}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var errs []string
	for result := range results {
		if result.err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", result.name, result.err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("deployment failed:\n  %s", strings.Join(errs, "\n  "))
	}

	fmt.Println()
	fmt.Println("Cluster deployment complete. IAM and VPC resources are ready.")
	return nil
}
