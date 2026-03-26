package clusteriam

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/crypto"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/services/clusteriam"
	"github.com/spf13/cobra"
)

// CreateOptions holds options for the cluster-iam create command.
// It is exported so that composite commands (e.g. cluster deploy) can reuse
// the same flag definitions without duplicating them.
type CreateOptions struct {
	ClusterName   string
	OIDCIssuerURL string
	Region        string
}

// AddFlags registers the IAM-specific flags on cmd bound to opts.
// --region is intentionally excluded: it is a persistent root-level flag
// inherited by all commands.
func AddFlags(cmd *cobra.Command, opts *CreateOptions) {
	cmd.Flags().StringVar(&opts.OIDCIssuerURL, "oidc-issuer-url", "", "OIDC issuer URL from Management Cluster (required)")
}

func newCreateCommand() *cobra.Command {
	opts := &CreateOptions{}

	cmd := &cobra.Command{
		Use:   "create CLUSTER_NAME",
		Short: "Create cluster IAM resources",
		Long: `Create IAM OIDC provider and roles for a hosted cluster.

This command:
1. Fetches the TLS thumbprint from the OIDC issuer URL
2. Creates a CloudFormation stack with the following resources:
   - IAM OIDC Provider
   - 7 control plane IAM roles (ingress, cloud-controller-manager, ebs-csi, etc.)
   - Worker node IAM role and instance profile

Example:
  rosactl cluster-iam create my-cluster \
    --oidc-issuer-url https://d1234.cloudfront.net/my-cluster \
    --region us-east-1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ClusterName = args[0]
			opts.Region, _ = cmd.Flags().GetString("region")
			return RunCreate(cmd.Context(), opts)
		},
	}

	AddFlags(cmd, opts)

	cmd.MarkFlagRequired("oidc-issuer-url")

	return cmd
}

// RunCreate executes the IAM creation workflow. It is exported so that
// composite commands can invoke it directly.
func RunCreate(ctx context.Context, opts *CreateOptions) error {
	// Validate cluster name
	if err := validateClusterName(opts.ClusterName); err != nil {
		return err
	}

	if opts.Region == "" {
		return fmt.Errorf("--region is required")
	}

	// Validate OIDC issuer URL
	if !strings.HasPrefix(opts.OIDCIssuerURL, "https://") {
		return fmt.Errorf("OIDC issuer URL must start with https://")
	}

	fmt.Println("🔐 Creating cluster IAM resources...")
	fmt.Printf("   Cluster: %s\n", opts.ClusterName)
	fmt.Printf("   OIDC Issuer: %s\n", opts.OIDCIssuerURL)
	fmt.Printf("   Region: %s\n", opts.Region)
	fmt.Println()

	// Fetch TLS thumbprint
	fmt.Println("🔍 Fetching TLS thumbprint from OIDC issuer...")
	thumbprint, err := crypto.GetOIDCThumbprint(ctx, opts.OIDCIssuerURL)
	if err != nil {
		return fmt.Errorf("failed to fetch TLS thumbprint: %w", err)
	}
	fmt.Printf("   Thumbprint: %s\n", thumbprint)
	fmt.Println()

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(opts.Region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create service request
	req := &clusteriam.CreateIAMRequest{
		ClusterName:    opts.ClusterName,
		OIDCIssuerURL:  opts.OIDCIssuerURL,
		OIDCThumbprint: thumbprint,
		AWSConfig:      cfg,
	}

	fmt.Println("📄 Preparing IAM CloudFormation operation...")
	fmt.Printf("☁️  Creating or updating CloudFormation stack: rosa-%s-iam\n", opts.ClusterName)
	fmt.Println("   This may take several minutes...")
	fmt.Println()

	// Call service layer
	resp, err := clusteriam.CreateIAM(ctx, req)
	if err != nil {
		return err
	}

	fmt.Println("✅ Cluster IAM resources created successfully!")
	fmt.Printf("   Stack ID: %s\n", resp.StackID)
	fmt.Println()

	if len(resp.Outputs) > 0 {
		fmt.Println("Created Resources:")
		for key, value := range resp.Outputs {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}

	return nil
}

func validateClusterName(name string) error {
	if name == "" {
		return fmt.Errorf("cluster name cannot be empty")
	}

	// Cluster name must be lowercase alphanumeric with hyphens
	for i, c := range name {
		if c >= 'a' && c <= 'z' {
			continue
		}
		if c >= '0' && c <= '9' && i > 0 {
			continue
		}
		if c == '-' && i > 0 && i < len(name)-1 {
			continue
		}
		return fmt.Errorf("cluster name must be lowercase alphanumeric with hyphens, got: %s", name)
	}

	return nil
}
