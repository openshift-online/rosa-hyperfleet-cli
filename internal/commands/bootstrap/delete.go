package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws/cloudformation"
	"github.com/spf13/cobra"
)

type deleteOptions struct {
	region    string
	stackName string
	noWait    bool
}

func newDeleteCommand() *cobra.Command {
	opts := &deleteOptions{}

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete the Lambda function bootstrap stack",
		Long: `Delete the Lambda function infrastructure via CloudFormation.

This command deletes the Lambda function and its execution role.

Example:
  rosactl bootstrap delete --region us-east-1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.region, "region", "", "AWS region (required)")
	cmd.Flags().StringVar(&opts.stackName, "stack-name", defaultStackName, "Name of the CloudFormation stack")
	cmd.Flags().BoolVar(&opts.noWait, "no-wait", true, "Return immediately without waiting for stack deletion to complete")

	_ = cmd.MarkFlagRequired("region")

	return cmd
}

func runDelete(ctx context.Context, opts *deleteOptions) error {
	fmt.Printf("🗑️  Deleting Lambda bootstrap stack: %s\n", opts.stackName)
	fmt.Printf("   Region: %s\n", opts.region)
	fmt.Println()

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(opts.region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create CloudFormation client
	cfnClient := cloudformation.NewClient(cfg)

	fmt.Println("📋 Deleting CloudFormation stack...")

	// Delete stack
	err = cfnClient.DeleteStack(ctx, opts.stackName, 10*time.Minute, opts.noWait)
	if err != nil {
		return fmt.Errorf("failed to delete stack: %w", err)
	}

	if opts.noWait {
		fmt.Println("✅ Stack deletion submitted!")
	} else {
		fmt.Println("✅ Stack deleted successfully!")
	}

	return nil
}
