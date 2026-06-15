package clusteroidc

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/services/clusteroidc"
	"github.com/spf13/cobra"
)

type deleteOptions struct {
	clusterName string
	region      string
	noWait      bool
}

func newDeleteCommand() *cobra.Command {
	opts := &deleteOptions{}

	cmd := &cobra.Command{
		Use:   "delete CLUSTER_NAME",
		Short: "Delete cluster OIDC provider",
		Long: `Delete the IAM OIDC provider for a hosted cluster.

This command deletes the CloudFormation stack containing the OIDC provider
(rosa-{cluster-name}-oidc). The IAM roles stack is left unchanged.

Example:
  rosactl cluster-oidc delete my-cluster --region us-east-1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.clusterName = args[0]
			return runDelete(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.region, "region", "", "AWS region (required)")
	cmd.Flags().BoolVar(&opts.noWait, "no-wait", true, "Return immediately without waiting for stack deletion to complete")
	_ = cmd.MarkFlagRequired("region")

	return cmd
}

func runDelete(ctx context.Context, opts *deleteOptions) error {
	fmt.Printf("Deleting cluster OIDC provider for: %s\n", opts.clusterName)
	fmt.Printf("   Region: %s\n", opts.region)
	fmt.Println()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(opts.region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	req := &clusteroidc.DeleteOIDCRequest{
		ClusterName: opts.clusterName,
		NoWait:      opts.noWait,
		AWSConfig:   cfg,
	}

	fmt.Printf("Deleting CloudFormation stack: rosa-%s-oidc\n", opts.clusterName)
	if !opts.noWait {
		fmt.Println("   This may take a few minutes...")
	}
	fmt.Println()

	if err := clusteroidc.DeleteOIDC(ctx, req); err != nil {
		return err
	}

	if opts.noWait {
		fmt.Println("Stack deletion submitted!")
	} else {
		fmt.Println("Cluster OIDC provider deleted successfully!")
	}
	return nil
}
