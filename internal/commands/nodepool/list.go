package nodepool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"text/tabwriter"

	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/config"
	"github.com/spf13/cobra"
)

type listOptions struct {
	clusterID string
	limit     int
	offset    int
	output    string
}

type nodepoolItem struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	ClusterID string                 `json:"cluster_id"`
	Spec      map[string]interface{} `json:"spec"`
	Status    *nodepoolStatus        `json:"status"`
}

type nodepoolStatus struct {
	Phase string `json:"phase"`
}

type listResponse struct {
	Items  []nodepoolItem `json:"items"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
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
  rosactl nodepool list --cluster-id <id>
  rosactl nodepool list --cluster-id <id> --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.clusterID == "" {
				return fmt.Errorf("--cluster-id is required")
			}
			return runList(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.clusterID, "cluster-id", "", "Cluster ID (required)")
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

	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve AWS credentials: %w", err)
	}

	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	endpoint := fmt.Sprintf("%s/api/v0/nodepools?limit=%d&offset=%d&clusterId=%s",
		baseURL, opts.limit, opts.offset, url.QueryEscape(opts.clusterID))

	body, statusCode, err := signedGet(ctx, endpoint, creds, region)
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

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if _, err := fmt.Fprintln(w, "ID\tNAME\tREPLICAS\tINSTANCE_TYPE\tPHASE"); err != nil {
		return err
	}

	for _, np := range result.Items {
		replicas := "-"
		instanceType := "-"
		phase := "-"

		if spec := np.Spec; spec != nil {
			if r, ok := spec["replicas"].(float64); ok {
				replicas = fmt.Sprintf("%.0f", r)
			}
			if p, ok := spec["platform"].(map[string]interface{}); ok {
				if a, ok := p["aws"].(map[string]interface{}); ok {
					if it, ok := a["instanceType"].(string); ok {
						instanceType = it
					}
				}
			}
		}
		if np.Status != nil && np.Status.Phase != "" {
			phase = np.Status.Phase
		}

		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			np.ID, np.Name, replicas, instanceType, phase); err != nil {
			return err
		}
	}

	return w.Flush()
}
