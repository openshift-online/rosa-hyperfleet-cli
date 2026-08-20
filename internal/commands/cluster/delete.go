package cluster

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	v1alpha1 "github.com/openshift-online/rosa-hyperfleet-api/api/v1alpha1/public"
	"github.com/openshift-online/rosa-hyperfleet-api/clientset/platform"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws"
	platformclient "github.com/openshift-online/rosa-regional-platform-cli/internal/platform"
	"github.com/spf13/cobra"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

type deleteOptions struct {
	yes  bool
	wait bool
}

func newDeleteCommand() *cobra.Command {
	opts := &deleteOptions{}

	cmd := &cobra.Command{
		Use:   "delete <cluster-id|cluster-name>",
		Short: "Delete a hosted cluster",
		Long: `Delete a ROSA hosted cluster via the platform API.

The cluster is identified by name or ID. A confirmation prompt is shown
unless --yes is passed. Use --wait to poll until the cluster is fully
removed.

Examples:
  rosactl cluster delete my-cluster --region us-east-1
  rosactl cluster delete my-cluster --region us-east-1 --yes
  rosactl cluster delete aafa2c73-f265-4a63-a6d9-673a7b999ee9 --region us-east-1 --wait`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeleteCluster(cmd.Context(), args[0], opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&opts.wait, "wait", false, "Wait for the cluster to be fully deleted")

	return cmd
}

func runDeleteCluster(ctx context.Context, nameOrID string, opts *deleteOptions) error {
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

	// Fetch cluster to resolve name → ID and get display name
	cluster, err := getClusterByNameOrID(ctx, cs, nameOrID)
	if err != nil {
		return err
	}

	clusterName := cluster.Name
	clusterID := string(cluster.UID)

	if !opts.yes {
		fmt.Fprintf(os.Stderr, "Are you sure you want to delete cluster %q (ID: %s)? [y/N] ", clusterName, clusterID)
		reader := bufio.NewReader(os.Stdin)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			fmt.Fprintln(os.Stderr, "Deletion cancelled.")
			return nil
		}
	}

	// Delete the cluster via SDK
	err = cs.HyperfleetV1alpha1().Clusters().Delete(ctx, clusterID, platform.DeleteOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return fmt.Errorf("cluster %q not found (may have already been deleted)", nameOrID)
		}
		return fmt.Errorf("failed to delete cluster: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Cluster %q (ID: %s) deletion initiated.\n", clusterName, clusterID)

	if !opts.wait {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Waiting for cluster %q to be deleted...\n", clusterName)
	const (
		pollInterval = 15 * time.Second
		timeout      = 10 * time.Minute
	)

	// Use the SDK's WaitUntil to poll for deletion
	err = cs.HyperfleetV1alpha1().Clusters().WaitUntil(ctx, clusterID, func(c *v1alpha1.Cluster) bool {
		// Cluster is deleted when it's not found (c == nil)
		return c == nil
	}, pollInterval, timeout)

	if err != nil {
		if err == context.DeadlineExceeded {
			return fmt.Errorf("timed out waiting for cluster %q to be deleted after %s", clusterName, timeout)
		}
		return fmt.Errorf("error waiting for cluster deletion: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Cluster %q deleted successfully.\n", clusterName)
	return nil
}
