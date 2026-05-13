package clusteriam

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/crypto"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/services/clusteriam"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/services/clusteroidc"
	"github.com/spf13/cobra"
)

type createOptions struct {
	clusterName   string
	oidcIssuerURL string
	region        string
	noWait        bool
}

func newCreateCommand() *cobra.Command {
	opts := &createOptions{}

	cmd := &cobra.Command{
		Use:   "create CLUSTER_NAME",
		Short: "Create cluster IAM roles",
		Long: `Create IAM roles for a hosted cluster.

This command creates a CloudFormation stack with the following resources:
  - 7 control plane IAM roles (ingress, cloud-controller-manager, ebs-csi, etc.)
  - Worker node IAM role and instance profile

OIDC federation:
  If --oidc-issuer-url is provided, the OIDC provider is also created in a
  separate stack (rosa-{cluster-name}-oidc) and IAM trust policies are
  configured immediately.

  If omitted, roles are created with a placeholder trust policy. Run
  'rosactl cluster-oidc create' after obtaining the issuer URL from cluster creation
  to activate federation.

Examples:
  # Roles only (activate federation later with 'rosactl cluster-oidc create'):
  rosactl cluster-iam create my-cluster --region us-east-1

  # Roles + OIDC provider in one step (when issuer URL is known upfront):
  rosactl cluster-iam create my-cluster \
    --oidc-issuer-url https://d1234.cloudfront.net/my-cluster \
    --region us-east-1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.clusterName = args[0]
			return runCreate(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.oidcIssuerURL, "oidc-issuer-url", "", "OIDC issuer URL (optional — also creates OIDC provider if provided)")
	cmd.Flags().StringVar(&opts.region, "region", "", "AWS region (required)")
	cmd.Flags().BoolVar(&opts.noWait, "no-wait", false, "Return immediately without waiting for stack creation to complete")

	_ = cmd.MarkFlagRequired("region")

	return cmd
}

func runCreate(ctx context.Context, opts *createOptions) error {
	// Validate cluster name
	if err := validateClusterName(opts.clusterName); err != nil {
		return err
	}

	// Derive OIDC issuer domain if URL was provided
	var oidcIssuerDomain string
	if opts.oidcIssuerURL != "" {
		if !strings.HasPrefix(opts.oidcIssuerURL, "https://") {
			return fmt.Errorf("OIDC issuer URL must start with https://")
		}
		var err error
		oidcIssuerDomain, err = crypto.GetOIDCIssuerDomain(opts.oidcIssuerURL)
		if err != nil {
			return fmt.Errorf("failed to parse OIDC issuer URL: %w", err)
		}
	}

	fmt.Println("Creating cluster IAM roles...")
	fmt.Printf("   Cluster: %s\n", opts.clusterName)
	fmt.Printf("   Region: %s\n", opts.region)
	if oidcIssuerDomain != "" {
		fmt.Printf("   OIDC Issuer: %s\n", opts.oidcIssuerURL)
	}
	fmt.Println()

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(opts.region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create IAM stack
	iamReq := &clusteriam.CreateIAMRequest{
		ClusterName:      opts.clusterName,
		OIDCIssuerDomain: oidcIssuerDomain,
		NoWait:           opts.noWait,
		AWSConfig:        cfg,
	}

	fmt.Printf("Creating or updating CloudFormation stack: rosa-%s-iam\n", opts.clusterName)
	if !opts.noWait {
		fmt.Println("   This may take several minutes...")
	}
	fmt.Println()

	resp, err := clusteriam.CreateIAM(ctx, iamReq)
	if err != nil {
		return err
	}

	if opts.noWait {
		fmt.Println("Stack creation submitted!")
		fmt.Printf("   Stack ID: %s\n", resp.StackID)
	} else {
		fmt.Println("Cluster IAM roles created successfully!")
		fmt.Printf("   Stack ID: %s\n", resp.StackID)
		fmt.Println()

		if len(resp.Outputs) > 0 {
			fmt.Println("Created Resources:")
			for key, value := range resp.Outputs {
				fmt.Printf("  %s: %s\n", key, value)
			}
		}
	}

	// If OIDC issuer URL was provided, also create the OIDC provider stack
	if opts.oidcIssuerURL != "" {
		fmt.Println()
		fmt.Println("OIDC issuer URL provided — also creating OIDC provider...")
		fmt.Println()

		oidcReq := &clusteroidc.CreateOIDCRequest{
			ClusterName:   opts.clusterName,
			OIDCIssuerURL: opts.oidcIssuerURL,
			NoWait:        opts.noWait,
			AWSConfig:     cfg,
		}

		fmt.Printf("Creating CloudFormation stack: rosa-%s-oidc\n", opts.clusterName)
		if !opts.noWait {
			fmt.Println("   This may take a few minutes...")
		}
		fmt.Println()

		oidcResp, err := clusteroidc.CreateOIDC(ctx, oidcReq)
		if err != nil {
			return fmt.Errorf("IAM roles created but OIDC provider failed: %w", err)
		}

		if opts.noWait {
			fmt.Println("OIDC provider stack creation submitted!")
			fmt.Printf("   Stack ID: %s\n", oidcResp.StackID)
		} else {
			fmt.Println("Cluster OIDC provider created successfully!")
			fmt.Printf("   Stack ID: %s\n", oidcResp.StackID)
			fmt.Println()

			if len(oidcResp.Outputs) > 0 {
				fmt.Println("OIDC Resources:")
				for key, value := range oidcResp.Outputs {
					fmt.Printf("  %s: %s\n", key, value)
				}
			}
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
