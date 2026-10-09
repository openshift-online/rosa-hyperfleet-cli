package cluster

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	hyperfleet "github.com/openshift-online/rosa-hyperfleet-api/clientset"
	"github.com/openshift-online/rosa-hyperfleet-api/clientset/platform"
	hfrest "github.com/openshift-online/rosa-hyperfleet-api/clientset/rest"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/config"
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
		Use:   "delete <cluster-name>",
		Short: "Delete a hosted cluster",
		Long: `Delete a ROSA hosted cluster via the platform API.

The cluster is identified by its account-scoped name. A confirmation prompt is shown
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

func runDeleteCluster(ctx context.Context, clusterName string, opts *deleteOptions) error {
	baseURL, err := config.GetPlatformAPIURL()
	if err != nil {
		return err
	}

	cfg, err := aws.NewConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	region := cfg.Region
	if region == "" {
		return aws.ErrRegionRequired
	}

	// Load AWS config for clientset
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	accountID, err := config.GetAccountID()
	if err != nil {
		return fmt.Errorf("failed to get account ID: %w", err)
	}

	// Create clientset
	cs, err := hyperfleet.NewForConfig(&hfrest.Config{
		Host:      baseURL,
		AccountID: accountID,
		AWSConfig: awsCfg,
	})
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	cluster, err := cs.HyperfleetV1alpha1().Clusters().Get(ctx, clusterName, platform.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return fmt.Errorf("cluster %q not found", clusterName)
		}
		return fmt.Errorf("failed to get cluster %q: %w", clusterName, err)
	}

	if !opts.yes {
		fmt.Fprintf(os.Stderr, "Are you sure you want to delete cluster %q (ID: %s)? [y/N] ", cluster.Name, string(cluster.UID))
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

	// Delete cluster via clientset
	if err := cs.HyperfleetV1alpha1().Clusters().Delete(ctx, cluster.Name, platform.DeleteOptions{}); err != nil {
		if k8serrors.IsNotFound(err) {
			return fmt.Errorf("cluster %q not found (may have already been deleted)", cluster.Name)
		}
		return fmt.Errorf("failed to delete cluster: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Cluster %q (ID: %s) deletion initiated.\n", cluster.Name, string(cluster.UID))

	if !opts.wait {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Waiting for cluster %q to be deleted...\n", cluster.Name)
	const (
		pollInterval = 15 * time.Second
		timeout      = 10 * time.Minute
	)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)

		_, err := cs.HyperfleetV1alpha1().Clusters().Get(ctx, cluster.Name, platform.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				fmt.Fprintf(os.Stderr, "Cluster %q deleted successfully.\n", cluster.Name)
				return nil
			}
			// Transient error — keep polling
			fmt.Fprintf(os.Stderr, "Polling cluster status (transient error): %v\n", err)
			continue
		}
		fmt.Fprintf(os.Stderr, "Cluster %q still deleting...\n", cluster.Name)
	}

	return fmt.Errorf("timed out waiting for cluster %q to be deleted after %s", cluster.Name, timeout)
}
