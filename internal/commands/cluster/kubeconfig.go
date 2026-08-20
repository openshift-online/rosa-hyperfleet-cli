package cluster

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws"
	platformclient "github.com/openshift-online/rosa-regional-platform-cli/internal/platform"
	"github.com/spf13/cobra"
)

//go:embed kubeconfig.tmpl
var kubeconfigTemplate string

var kubeconfigTmpl = template.Must(template.New("kubeconfig").Parse(kubeconfigTemplate))

type kubeconfigData struct {
	Server      string
	ClusterName string
	RosactlPath string
	ClusterID   string
	Region      string
}

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
	cfg, err := aws.NewConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	// Create the clientset
	cs, err := platformclient.NewClientset(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	// Fetch cluster via SDK (try by ID first, then search by name)
	cluster, err := getClusterByNameOrID(ctx, cs, nameOrID)
	if err != nil {
		return err
	}

	// Extract API endpoint from cluster status
	if cluster.Status.ControlPlaneEndpoint.Host == "" {
		return fmt.Errorf("cluster %q API endpoint not available yet", nameOrID)
	}

	apiEndpoint := fmt.Sprintf("https://%s:%d",
		cluster.Status.ControlPlaneEndpoint.Host,
		cluster.Status.ControlPlaneEndpoint.Port)

	rosactlPath, _ := os.Executable()
	if rosactlPath == "" {
		rosactlPath = "rosactl"
	} else {
		rosactlPath, _ = filepath.Abs(rosactlPath)
	}

	var buf bytes.Buffer
	if err := kubeconfigTmpl.Execute(&buf, kubeconfigData{
		Server:      apiEndpoint,
		ClusterName: cluster.Name,
		RosactlPath: rosactlPath,
		ClusterID:   string(cluster.UID),
		Region:      region,
	}); err != nil {
		return fmt.Errorf("failed to render kubeconfig: %w", err)
	}

	_, err = os.Stdout.Write(buf.Bytes())
	return err
}
