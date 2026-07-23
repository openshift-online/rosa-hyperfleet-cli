package cluster

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/config"
	"github.com/spf13/cobra"
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

	// Resolve name → ID if needed (fetchClusterByName matches on both name and ID)
	cluster, err := fetchClusterByName(ctx, baseURL, nameOrID, creds, region)
	if err != nil {
		return err
	}

	if !opts.yes {
		fmt.Fprintf(os.Stderr, "Are you sure you want to delete cluster %q (ID: %s)? [y/N] ", cluster.Name, cluster.ID)
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

	endpoint := fmt.Sprintf("%s/api/v0/clusters/%s", baseURL, url.PathEscape(cluster.ID))
	body, statusCode, err := signedDelete(ctx, endpoint, creds, region)
	if err != nil {
		return fmt.Errorf("delete request failed: %w", err)
	}

	switch statusCode {
	case http.StatusAccepted:
		fmt.Fprintf(os.Stderr, "Cluster %q (ID: %s) deletion initiated.\n", cluster.Name, cluster.ID)
	case http.StatusNotFound:
		return fmt.Errorf("cluster %q not found (may have already been deleted)", nameOrID)
	default:
		return fmt.Errorf("API request failed with status %d: %s", statusCode, string(body))
	}

	if !opts.wait {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Waiting for cluster %q to be deleted...\n", cluster.Name)
	const (
		pollInterval = 15 * time.Second
		timeout      = 10 * time.Minute
	)
	deadline := time.Now().Add(timeout)
	checkEndpoint := fmt.Sprintf("%s/api/v0/clusters/%s", baseURL, url.PathEscape(cluster.ID))
	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)

		_, err := signedGet(ctx, checkEndpoint, creds, region)
		if err != nil {
			// signedGet returns an error for non-200; a 404/410 means deletion is complete
			if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "410") {
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
