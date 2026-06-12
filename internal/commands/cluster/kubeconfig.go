package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/config"
	"github.com/spf13/cobra"
)

func newKubeconfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubeconfig <cluster-id|cluster-name>",
		Short: "Generate a kubeconfig for a cluster using AWS IAM authentication",
		Long: `Generate a kubeconfig that uses rosactl as an exec credential plugin
for AWS IAM authentication. Pipe the output to a file and use with kubectl:

  rosactl cluster kubeconfig my-cluster > ~/.kube/my-cluster
  kubectl --kubeconfig=~/.kube/my-cluster get nodes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKubeconfig(cmd.Context(), args[0])
		},
	}

	return cmd
}

func runKubeconfig(ctx context.Context, nameOrID string) error {
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

	cluster, err := fetchClusterByName(ctx, baseURL, nameOrID, creds, region)
	if err != nil {
		return err
	}

	apiEndpoint, err := fetchAPIURL(ctx, baseURL, cluster.ID, creds, region)
	if err != nil {
		return err
	}
	if apiEndpoint == "" {
		return fmt.Errorf("cluster %q API endpoint not available yet", nameOrID)
	}

	rosactlPath, _ := os.Executable()
	if rosactlPath == "" {
		rosactlPath = "rosactl"
	} else {
		rosactlPath, _ = filepath.Abs(rosactlPath)
	}

	fmt.Printf(`apiVersion: v1
kind: Config
clusters:
  - cluster:
      server: %s
    name: %s
users:
  - name: %s-iam
    user:
      exec:
        apiVersion: client.authentication.k8s.io/v1
        interactiveMode: Never
        command: %s
        args:
          - cluster
          - get-token
          - --cluster-id
          - %s
          - --region
          - %s
contexts:
  - context:
      cluster: %s
      user: %s-iam
    name: %s
current-context: %s
`, apiEndpoint, cluster.Name,
		cluster.Name,
		rosactlPath,
		cluster.ID,
		region,
		cluster.Name, cluster.Name,
		cluster.Name,
		cluster.Name)

	return nil
}
