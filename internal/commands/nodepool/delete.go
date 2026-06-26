package nodepool

import (
	"context"
	"fmt"
	"os"

	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/config"
	"github.com/spf13/cobra"
)

func newDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete NODEPOOL_ID",
		Short: "Delete a node pool",
		Long: `Delete a node pool from a ROSA hosted cluster.

Examples:
  rosactl nodepool delete <nodepool-id> --region us-east-1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd.Context(), args[0])
		},
	}

	return cmd
}

func runDelete(ctx context.Context, nodepoolID string) error {
	baseURL, err := config.GetPlatformAPIURL()
	if err != nil {
		return err
	}

	cfg, err := aws.NewConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve AWS credentials: %w", err)
	}

	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	endpoint := fmt.Sprintf("%s/api/v0/nodepools/%s", baseURL, nodepoolID)
	body, statusCode, err := signedDelete(ctx, endpoint, creds, region)
	if err != nil {
		return err
	}

	if statusCode != 202 {
		return fmt.Errorf("API request failed with status %d: %s", statusCode, string(body))
	}

	fmt.Fprintf(os.Stderr, "✓ NodePool %s deletion initiated\n", nodepoolID)
	return nil
}
