package clustervpc

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/services/clustervpc"
	"github.com/spf13/cobra"
)

// CreateOptions holds options for the cluster-vpc create command.
// It is exported so that composite commands (e.g. cluster deploy) can reuse
// the same flag definitions without duplicating them.
type CreateOptions struct {
	ClusterName        string
	Region             string
	VpcCidr            string
	PublicSubnetCidrs  string
	PrivateSubnetCidrs string
	AvailabilityZones  string
	SingleNatGateway   bool
}

// AddFlags registers the VPC-specific flags on cmd bound to opts.
// The shared --region flag is intentionally excluded; callers add it once.
func AddFlags(cmd *cobra.Command, opts *CreateOptions) {
	cmd.Flags().StringVar(&opts.VpcCidr, "vpc-cidr", "10.0.0.0/16", "CIDR block for the VPC")
	cmd.Flags().StringVar(&opts.PublicSubnetCidrs, "public-subnet-cidrs", "10.0.101.0/24,10.0.102.0/24,10.0.103.0/24", "Comma-separated public subnet CIDRs")
	cmd.Flags().StringVar(&opts.PrivateSubnetCidrs, "private-subnet-cidrs", "10.0.0.0/19,10.0.32.0/19,10.0.64.0/19", "Comma-separated private subnet CIDRs")
	cmd.Flags().StringVar(&opts.AvailabilityZones, "availability-zones", "", "Comma-separated availability zones (optional, auto-detected if empty)")
	cmd.Flags().BoolVar(&opts.SingleNatGateway, "single-nat-gateway", true, "Use single NAT gateway (true=cost savings, false=HA per-AZ)")
}

func newCreateCommand() *cobra.Command {
	opts := &CreateOptions{}

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
			opts.ClusterName = args[0]
			return RunCreate(cmd.Context(), opts)
		},
	}

	AddFlags(cmd, opts)
	cmd.Flags().StringVar(&opts.Region, "region", "", "AWS region (required)")

	cmd.MarkFlagRequired("region")

	return cmd
}

// RunCreate executes the VPC creation workflow. It is exported so that
// composite commands can invoke it directly.
func RunCreate(ctx context.Context, opts *CreateOptions) error {
	fmt.Printf("🌐 Creating cluster VPC resources for: %s\n", opts.ClusterName)
	fmt.Printf("   Region: %s\n", opts.Region)
	fmt.Printf("   VPC CIDR: %s\n", opts.VpcCidr)
	fmt.Printf("   Single NAT Gateway: %t\n", opts.SingleNatGateway)
	fmt.Println()

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(opts.Region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Parse CIDR lists
	publicSubnets := strings.Split(opts.PublicSubnetCidrs, ",")
	privateSubnets := strings.Split(opts.PrivateSubnetCidrs, ",")

	// Parse availability zones if provided
	var azs []string
	if opts.AvailabilityZones != "" {
		azs = strings.Split(opts.AvailabilityZones, ",")
	}

	// Create service request
	req := &clustervpc.CreateVPCRequest{
		ClusterName:        opts.ClusterName,
		VpcCidr:            opts.VpcCidr,
		PublicSubnetCidrs:  publicSubnets,
		PrivateSubnetCidrs: privateSubnets,
		AvailabilityZones:  azs,
		SingleNatGateway:   opts.SingleNatGateway,
		AWSConfig:          cfg,
	}

	fmt.Println("📄 Loading CloudFormation template...")
	fmt.Printf("☁️  Creating CloudFormation stack: rosa-%s-vpc\n", opts.ClusterName)
	fmt.Println("   This may take several minutes...")
	fmt.Println()

	// Call service layer
	resp, err := clustervpc.CreateVPC(ctx, req)
	if err != nil {
		return err
	}

	fmt.Println("✅ Cluster VPC resources created successfully!")
	fmt.Printf("   Stack ID: %s\n", resp.StackID)
	fmt.Println()

	if len(resp.Outputs) > 0 {
		fmt.Println("Outputs:")
		for key, value := range resp.Outputs {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}

	return nil
}
