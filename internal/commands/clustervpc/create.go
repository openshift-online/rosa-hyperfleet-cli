package clustervpc

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/services/clustervpc"
	"github.com/spf13/cobra"
)

type createOptions struct {
	clusterName        string
	region             string
	vpcCidr            string
	publicSubnetCidrs  string
	privateSubnetCidrs string
	availabilityZones  string
	singleNatGateway   bool
	noWait             bool
}

func newCreateCommand() *cobra.Command {
	opts := &createOptions{}

	cmd := &cobra.Command{
		Use:   "create CLUSTER_NAME",
		Short: "Create cluster VPC resources",
		Long: `Create VPC networking resources for a hosted cluster.

This command creates a CloudFormation stack containing VPC, subnets, NAT gateways,
routing, security groups, and Route53 private hosted zone.

Example:
  rosactl cluster-vpc create my-cluster --region us-east-1

  # With custom CIDR ranges
  rosactl cluster-vpc create my-cluster \
    --region us-east-1 \
    --vpc-cidr 10.1.0.0/16 \
    --public-subnet-cidrs 10.1.101.0/24,10.1.102.0/24,10.1.103.0/24 \
    --private-subnet-cidrs 10.1.0.0/19,10.1.32.0/19,10.1.64.0/19

  # With specific availability zones and per-AZ NAT gateways
  rosactl cluster-vpc create my-cluster \
    --region us-east-1 \
    --availability-zones us-east-1a,us-east-1b,us-east-1c \
    --single-nat-gateway=false`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.clusterName = args[0]
			return runCreate(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.region, "region", "", "AWS region (required)")
	cmd.Flags().StringVar(&opts.vpcCidr, "vpc-cidr", "10.0.0.0/16", "CIDR block for the VPC")
	cmd.Flags().StringVar(&opts.publicSubnetCidrs, "public-subnet-cidrs", "10.0.101.0/24,10.0.102.0/24,10.0.103.0/24", "Comma-separated public subnet CIDRs")
	cmd.Flags().StringVar(&opts.privateSubnetCidrs, "private-subnet-cidrs", "10.0.0.0/19,10.0.32.0/19,10.0.64.0/19", "Comma-separated private subnet CIDRs")
	cmd.Flags().StringVar(&opts.availabilityZones, "availability-zones", "", "Comma-separated availability zones, 1-3 (optional, auto-detected if empty)")
	cmd.Flags().BoolVar(&opts.singleNatGateway, "single-nat-gateway", true, "Use single NAT gateway (true=cost savings, false=HA per-AZ)")
	cmd.Flags().BoolVar(&opts.noWait, "no-wait", false, "Return immediately without waiting for stack creation to complete")

	_ = cmd.MarkFlagRequired("region")

	return cmd
}

func runCreate(ctx context.Context, opts *createOptions) error {
	fmt.Printf("🌐 Creating cluster VPC resources for: %s\n", opts.clusterName)
	fmt.Printf("   Region: %s\n", opts.region)
	fmt.Printf("   VPC CIDR: %s\n", opts.vpcCidr)
	fmt.Printf("   Single NAT Gateway: %t\n", opts.singleNatGateway)
	fmt.Println()

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(opts.region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Parse CIDR lists
	publicSubnets := strings.Split(opts.publicSubnetCidrs, ",")
	privateSubnets := strings.Split(opts.privateSubnetCidrs, ",")

	// Parse availability zones if provided
	var azs []string
	if opts.availabilityZones != "" {
		azs = strings.Split(opts.availabilityZones, ",")
	}

	// Create service request
	req := &clustervpc.CreateVPCRequest{
		ClusterName:        opts.clusterName,
		VpcCidr:            opts.vpcCidr,
		PublicSubnetCidrs:  publicSubnets,
		PrivateSubnetCidrs: privateSubnets,
		AvailabilityZones:  azs,
		SingleNatGateway:   opts.singleNatGateway,
		NoWait:             opts.noWait,
		AWSConfig:          cfg,
	}

	fmt.Println("📄 Loading CloudFormation template...")
	fmt.Printf("☁️  Creating CloudFormation stack: rosa-%s-vpc\n", opts.clusterName)
	if !opts.noWait {
		fmt.Println("   This may take several minutes...")
	}
	fmt.Println()

	// Call service layer
	resp, err := clustervpc.CreateVPC(ctx, req)
	if err != nil {
		return err
	}

	if opts.noWait {
		fmt.Println("✅ Stack creation submitted!")
		fmt.Printf("   Stack ID: %s\n", resp.StackID)
		fmt.Println()
		fmt.Printf("💡 Stack is being created asynchronously. Check status with:\n")
		fmt.Printf("   rosactl cluster-vpc describe %s --region %s\n", opts.clusterName, opts.region)
	} else {
		fmt.Println("✅ Cluster VPC resources created successfully!")
		fmt.Printf("   Stack ID: %s\n", resp.StackID)
		fmt.Println()

		if len(resp.Outputs) > 0 {
			fmt.Println("Outputs:")
			for key, value := range resp.Outputs {
				fmt.Printf("  %s: %s\n", key, value)
			}
		}
	}

	return nil
}
