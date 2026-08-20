package nodepool

import (
	"context"
	"fmt"
	"os"

	"github.com/openshift-online/rosa-hyperfleet-api/clientset/platform"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws"
	platformclient "github.com/openshift-online/rosa-regional-platform-cli/internal/platform"
	"github.com/spf13/cobra"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

type nodepoolDeleteOptions struct {
	clusterID string
}

func newDeleteCommand() *cobra.Command {
	opts := &nodepoolDeleteOptions{}

	cmd := &cobra.Command{
		Use:   "delete NODEPOOL_ID",
		Short: "Delete a node pool",
		Long: `Delete a node pool from a ROSA hosted cluster.

Examples:
  rosactl nodepool delete <nodepool-id> --cluster-id <cluster-id> --region us-east-1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.clusterID == "" {
				return fmt.Errorf("--cluster-id is required")
			}
			return runDelete(cmd.Context(), args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.clusterID, "cluster-id", "", "Cluster ID (required)")

	return cmd
}

func runDelete(ctx context.Context, nodepoolID string, opts *nodepoolDeleteOptions) error {
	cfg, err := aws.NewConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	if cfg.Region == "" {
		return aws.ErrRegionRequired
	}

	// Create the clientset
	cs, err := platformclient.NewClientset(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	// Delete the nodepool via SDK
	// NodePools are namespaced by cluster ID
	err = cs.HyperfleetV1alpha1().NodePools(opts.clusterID).Delete(ctx, nodepoolID, platform.DeleteOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return fmt.Errorf("nodepool %q not found in cluster %q", nodepoolID, opts.clusterID)
		}
		return fmt.Errorf("failed to delete nodepool: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ NodePool %s deletion initiated\n", nodepoolID)
	return nil
}
