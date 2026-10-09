package nodepool

import (
	"context"
	"fmt"
	"os"
	"strings"

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
	clusterName string
}

func newDeleteCommand() *cobra.Command {
	opts := &deleteOptions{}

	cmd := &cobra.Command{
		Use:   "delete NODEPOOL_NAME",
		Short: "Delete a node pool",
		Long: `Delete a node pool from a ROSA hosted cluster.

Examples:
  rosactl nodepool delete my-cluster.my-nodepool --cluster-name my-cluster --region us-east-1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.clusterName == "" {
				return fmt.Errorf("--cluster-name is required")
			}
			return runDelete(cmd.Context(), args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.clusterName, "cluster-name", "", "Account-scoped Cluster name (required)")

	return cmd
}

func runDelete(ctx context.Context, nodepoolName string, opts *deleteOptions) error {
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

	nodepools := cs.HyperfleetV1alpha1().NodePools()
	nodepool, err := nodepools.Get(ctx, nodepoolName, platform.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return fmt.Errorf("nodepool %q not found", nodepoolName)
		}
		return fmt.Errorf("failed to get nodepool %q: %w", nodepoolName, err)
	}
	if !strings.HasPrefix(nodepool.Name, opts.clusterName+".") {
		return fmt.Errorf("nodepool %q does not belong to cluster %q", nodepoolName, opts.clusterName)
	}

	if err := nodepools.Delete(ctx, nodepool.Name, platform.DeleteOptions{}); err != nil {
		if k8serrors.IsNotFound(err) {
			return fmt.Errorf("nodepool %q not found", nodepoolName)
		}
		return fmt.Errorf("failed to delete nodepool %q: %w", nodepoolName, err)
	}

	fmt.Fprintf(os.Stderr, "✓ NodePool %s deletion initiated\n", nodepoolName)
	return nil
}
