package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/config"
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
	CloudURL  string `json:"cloudUrl"`
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
	// Get the platform API URL from config
	baseURL, err := config.GetPlatformAPIURL()
	if err != nil {
		return err
	}

	// Build the API endpoint URL with properly encoded query parameters
	params := url.Values{
		"limit":  {strconv.Itoa(opts.limit)},
		"offset": {strconv.Itoa(opts.offset)},
	}
	if opts.status != "" {
		params.Set("status", opts.status)
	}
	endpoint := fmt.Sprintf("%s/api/v0/clusters?%s", baseURL, params.Encode())

	// Load AWS config for SigV4 signing
	cfg, err := aws.NewConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Sign the request with AWS SigV4
	signer := v4.NewSigner()
	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve AWS credentials: %w", err)
	}

	// Determine the region from AWS config
	region := cfg.Region
	if region == "" {
		region = "us-east-1" // Default region
	}

	// SHA256 hash of empty body for GET requests
	emptyBodyHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	err = signer.SignHTTP(ctx, creds, req, emptyBodyHash, "execute-api", region, time.Now())
	if err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	// Execute the request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error responses
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// If JSON output requested, print raw response
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

	// Parse the JSON response for table display
	var result listResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Display as table
	return displayTable(result.Items)
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
