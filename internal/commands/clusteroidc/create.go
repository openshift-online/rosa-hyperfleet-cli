package clusteroidc

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/services/clusteroidc"
	"github.com/spf13/cobra"
)

type createOptions struct {
	clusterName    string
	oidcIssuerURL  string
	oidcThumbprint string
	region         string
	noWait         bool
}

func newCreateCommand() *cobra.Command {
	opts := &createOptions{}

	cmd := &cobra.Command{
		Use:   "create CLUSTER_NAME",
		Short: "Create cluster OIDC provider",
		Long: `Create an IAM OIDC provider for a hosted cluster.

This command:
1. Fetches the TLS thumbprint from the OIDC issuer URL (unless --oidc-thumbprint is provided)
2. Creates a CloudFormation stack with the IAM OIDC provider (rosa-{cluster-name}-oidc)
3. Updates the IAM roles stack (rosa-{cluster-name}-iam) trust policies with the issuer domain

The IAM roles stack must already exist (created via 'rosactl cluster-iam create').

Example:
  rosactl cluster-oidc create my-cluster \
    --oidc-issuer-url https://d1234.cloudfront.net/my-cluster \
    --region us-east-1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.clusterName = args[0]
			return runCreate(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.oidcIssuerURL, "oidc-issuer-url", "", "OIDC issuer URL from the cluster (required)")
	cmd.Flags().StringVar(&opts.oidcThumbprint, "oidc-thumbprint", "", "TLS thumbprint (optional, fetched automatically if omitted)")
	cmd.Flags().StringVar(&opts.region, "region", "", "AWS region (required)")
	cmd.Flags().BoolVar(&opts.noWait, "no-wait", false, "Return immediately without waiting for stack creation to complete")

	_ = cmd.MarkFlagRequired("oidc-issuer-url")
	_ = cmd.MarkFlagRequired("region")

	return cmd
}

func runCreate(ctx context.Context, opts *createOptions) error {
	if !strings.HasPrefix(opts.oidcIssuerURL, "https://") {
		return fmt.Errorf("OIDC issuer URL must start with https://")
	}

	fmt.Println("Creating cluster OIDC provider...")
	fmt.Printf("   Cluster: %s\n", opts.clusterName)
	fmt.Printf("   OIDC Issuer: %s\n", opts.oidcIssuerURL)
	fmt.Printf("   Region: %s\n", opts.region)
	fmt.Println()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(opts.region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	req := &clusteroidc.CreateOIDCRequest{
		ClusterName:    opts.clusterName,
		OIDCIssuerURL:  opts.oidcIssuerURL,
		OIDCThumbprint: opts.oidcThumbprint,
		NoWait:         opts.noWait,
		AWSConfig:      cfg,
	}

	fmt.Printf("Creating CloudFormation stack: rosa-%s-oidc\n", opts.clusterName)
	if !opts.noWait {
		fmt.Println("   This may take a few minutes...")
	}
	fmt.Println()

	resp, err := clusteroidc.CreateOIDC(ctx, req)
	if err != nil {
		return err
	}

	if opts.noWait {
		fmt.Println("Stack creation submitted!")
		fmt.Printf("   Stack ID: %s\n", resp.StackID)
	} else {
		fmt.Println("Cluster OIDC provider created successfully!")
		fmt.Printf("   Stack ID: %s\n", resp.StackID)
		fmt.Println()

		if len(resp.Outputs) > 0 {
			fmt.Println("Created Resources:")
			for key, value := range resp.Outputs {
				fmt.Printf("  %s: %s\n", key, value)
			}
		}
	}

	return nil
}
