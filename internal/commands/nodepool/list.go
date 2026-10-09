package nodepool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	v1alpha1 "github.com/openshift-online/rosa-hyperfleet-api/api/v1alpha1/public"
	hyperfleet "github.com/openshift-online/rosa-hyperfleet-api/clientset"
	"github.com/openshift-online/rosa-hyperfleet-api/clientset/platform"
	hfrest "github.com/openshift-online/rosa-hyperfleet-api/clientset/rest"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/config"
	"github.com/spf13/cobra"
)

type listOptions struct {
	clusterName string
	limit       int
	offset      int
	output      string
}

func newListCommand() *cobra.Command {
	opts := &listOptions{
		limit:  50,
		offset: 0,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List node pools for a cluster",
		Long: `List node pools for a ROSA hosted cluster.

Examples:
  rosactl nodepool list --cluster-name <name>
  rosactl nodepool list --cluster-name <name> --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.clusterName == "" {
				return fmt.Errorf("--cluster-name is required")
			}
			if opts.limit < 1 || opts.limit > 100 {
				return fmt.Errorf("--limit must be between 1 and 100")
			}
			if opts.offset < 0 {
				return fmt.Errorf("--offset must be non-negative")
			}
			return runList(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.clusterName, "cluster-name", "", "Account-scoped Cluster name (required)")
	cmd.Flags().IntVar(&opts.limit, "limit", opts.limit, "Maximum number of nodepools to return (1-100)")
	cmd.Flags().IntVar(&opts.offset, "offset", opts.offset, "Number of nodepools to skip")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "table", "Output format: table or json")

	return cmd
}

func runList(ctx context.Context, opts *listOptions) error {
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

	// The NodePool API is account-scoped. Filter the account's pages by the
	// validated <cluster-name>.<child-name> convention, then apply pagination
	// to that cluster's results.
	const pageSize = int64(100)
	nodepools := cs.HyperfleetV1alpha1().NodePools()
	var clusterNodePools []v1alpha1.NodePool
	for offset := int64(0); ; offset += pageSize {
		page, err := nodepools.List(ctx, platform.NodePoolListOptions{Limit: pageSize, Offset: offset})
		if err != nil {
			return fmt.Errorf("failed to list nodepools: %w", err)
		}
		clusterNodePools = append(clusterNodePools, filterNodePoolsByClusterName(page.Items, opts.clusterName)...)
		if len(page.Items) < int(pageSize) {
			break
		}
	}
	clusterNodePools = paginateNodePools(clusterNodePools, opts.offset, opts.limit)

	if opts.output == "json" {
		prettyJSON, err := json.MarshalIndent(clusterNodePools, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(prettyJSON))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if _, err := fmt.Fprintln(w, "ID\tNAME\tREPLICAS\tINSTANCE_TYPE\tPHASE"); err != nil {
		return err
	}

	for _, np := range clusterNodePools {
		replicas := "-"
		instanceType := "-"
		phase := string(np.Status.Phase)

		if np.Spec.NodePool.Replicas != nil {
			replicas = fmt.Sprintf("%d", *np.Spec.NodePool.Replicas)
		}
		if np.Spec.NodePool.Platform.AWS != nil && np.Spec.NodePool.Platform.AWS.InstanceType != "" {
			instanceType = np.Spec.NodePool.Platform.AWS.InstanceType
		}
		if phase == "" {
			phase = "-"
		}

		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			string(np.UID), np.Name, replicas, instanceType, phase); err != nil {
			return err
		}
	}

	return w.Flush()
}

func filterNodePoolsByClusterName(nodepools []v1alpha1.NodePool, clusterName string) []v1alpha1.NodePool {
	filtered := make([]v1alpha1.NodePool, 0, len(nodepools))
	for _, nodepool := range nodepools {
		if strings.HasPrefix(nodepool.Name, clusterName+".") {
			filtered = append(filtered, nodepool)
		}
	}
	return filtered
}

func paginateNodePools(nodepools []v1alpha1.NodePool, offset, limit int) []v1alpha1.NodePool {
	start := min(offset, len(nodepools))
	end := min(start+limit, len(nodepools))
	return nodepools[start:end]
}
