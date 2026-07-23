package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"text/tabwriter"

	"github.com/openshift-online/rosa-regional-platform-cli/internal/client"
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

type listResponse struct {
	Items  []clusterItem `json:"items"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
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
	c, err := client.New(ctx)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v0/clusters?limit=%d&offset=%d", opts.limit, opts.offset)
	if opts.status != "" {
		path = fmt.Sprintf("%s&status=%s", path, url.QueryEscape(opts.status))
	}

	body, statusCode, err := c.Get(ctx, path)
	if err != nil {
		return err
	}
	if statusCode != 200 {
		return fmt.Errorf("API request failed with status %d: %s", statusCode, string(body))
	}

	if opts.output == "json" {
		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			fmt.Println(string(body))
			return nil
		}
		prettyJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(prettyJSON))
		return nil
	}

	var result listResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	return displayTable(result.Items)
}

func displayTable(clusters []clusterItem) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	if _, err := fmt.Fprintln(w, "ID\tNAME\tVERSION\tAVAILABLE\tREADY\tMESSAGE"); err != nil {
		return err
	}

	for _, cluster := range clusters {
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
	for _, cond := range conditions {
		if cond.Type == condType && cond.Message != "" {
			return cond.Message
		}
	}
	for _, cond := range conditions {
		if cond.Type == "Adapter1Successful" && cond.Message != "" {
			return cond.Message
		}
	}
	for _, cond := range conditions {
		if cond.Message != "" {
			return cond.Message
		}
	}
	return ""
}
