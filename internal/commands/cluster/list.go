package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/openshift-online/rosa-hyperfleet-api/clientset/platform"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws"
	platformclient "github.com/openshift-online/rosa-regional-platform-cli/internal/platform"
	"github.com/spf13/cobra"
)

type listOptions struct {
	limit  int
	offset int
	status string
	output string
}

type clusterSpec struct {
	Placement string `json:"placement"`
	Version   string `json:"version"`
	CloudURL  string `json:"oidcIssuerURL"`
}

type condition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type clusterStatus struct {
	Conditions []condition `json:"conditions"`
}

type clusterItem struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	CreatedAt string        `json:"created_at"`
	Spec      clusterSpec   `json:"spec"`
	Status    clusterStatus `json:"status"`
}

func newListCommand() *cobra.Command {
	opts := &listOptions{
		limit:  50,
		offset: 0,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List clusters from the platform API",
		Long: `List clusters from the platform API.

This command queries the platform API to retrieve a list of clusters.

Example:
  rosactl cluster list
  rosactl cluster list --limit 10
  rosactl cluster list --status Ready`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd.Context(), opts)
		},
	}

	cmd.Flags().IntVar(&opts.limit, "limit", opts.limit, "Maximum number of clusters to return (1-100)")
	cmd.Flags().IntVar(&opts.offset, "offset", opts.offset, "Number of clusters to skip")
	cmd.Flags().StringVar(&opts.status, "status", opts.status, "Filter by status (Pending, Progressing, Ready, Failed)")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "table", "Output format: table or json")

	return cmd
}

func runList(ctx context.Context, opts *listOptions) error {
	cfg, err := aws.NewConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	// Create the clientset
	cs, err := platformclient.NewClientset(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	// List clusters via SDK
	clusterList, err := cs.HyperfleetV1alpha1().Clusters().List(ctx, platform.ListOptions{
		Limit:  int64(opts.limit),
		Offset: int64(opts.offset),
	})
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	if opts.output == "json" {
		// Convert to JSON for output
		resultBytes, err := json.MarshalIndent(clusterList, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal response: %w", err)
		}
		fmt.Println(string(resultBytes))
		return nil
	}

	// Convert typed list to clusterItem for table display
	items := make([]clusterItem, 0, len(clusterList.Items))
	for _, c := range clusterList.Items {
		item := clusterItem{
			ID:   string(c.UID),
			Name: c.Name,
			Spec: clusterSpec{
				Version: c.Spec.HostedCluster.Release.Image,
			},
		}

		// Extract conditions from status
		if c.Status.Conditions != nil {
			for _, cond := range c.Status.Conditions {
				item.Status.Conditions = append(item.Status.Conditions, condition{
					Type:    cond.Type,
					Status:  string(cond.Status),
					Message: cond.Message,
				})
			}
		}

		// Filter by status if specified
		if opts.status != "" {
			if getConditionStatus(item.Status.Conditions, opts.status) == "-" {
				continue
			}
		}

		items = append(items, item)
	}

	return displayTable(items)
}

func displayTable(clusters []clusterItem) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	// Print header
	if _, err := fmt.Fprintln(w, "ID\tNAME\tVERSION\tAVAILABLE\tREADY\tMESSAGE"); err != nil {
		return err
	}

	// Print each cluster
	for _, cluster := range clusters {
		// Extract status and message from conditions
		available := getConditionStatus(cluster.Status.Conditions, "Available")
		ready := getConditionStatus(cluster.Status.Conditions, "Ready")
		message := getConditionMessage(cluster.Status.Conditions, "Ready")

		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			cluster.ID,
			cluster.Name,
			cluster.Spec.Version,
			available,
			ready,
			message,
		); err != nil {
			return err
		}
	}

	return w.Flush()
}

func getConditionStatus(conditions []condition, condType string) string {
	for _, cond := range conditions {
		if cond.Type == condType {
			return cond.Status
		}
	}
	return "-"
}

func getConditionMessage(conditions []condition, condType string) string {
	// First try the specified condition type
	for _, cond := range conditions {
		if cond.Type == condType && cond.Message != "" {
			return cond.Message
		}
	}
	// Fall back to Adapter1Successful which typically has the main status message
	for _, cond := range conditions {
		if cond.Type == "Adapter1Successful" && cond.Message != "" {
			return cond.Message
		}
	}
	// Finally return any condition with a message
	for _, cond := range conditions {
		if cond.Message != "" {
			return cond.Message
		}
	}
	return ""
}
