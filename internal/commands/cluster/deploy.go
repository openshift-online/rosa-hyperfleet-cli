package cluster

import (
	"context"
	"fmt"
	"strings"
	"sync"

	cmdiam "github.com/openshift-online/rosa-regional-platform-cli/internal/commands/clusteriam"
	cmdvpc "github.com/openshift-online/rosa-regional-platform-cli/internal/commands/clustervpc"
	"github.com/spf13/cobra"
)

// deployOptions composes the individual command option types so that flag
// definitions are owned exactly once — in the clusteriam and clustervpc
// packages — and automatically inherited here.
type deployOptions struct {
	clusterName string
	region      string
	iam         cmdiam.CreateOptions
	vpc         cmdvpc.CreateOptions
}

func newDeployCommand() *cobra.Command {
	opts := &deployOptions{}

	cmd := &cobra.Command{
		Use:   "deploy CLUSTER_NAME",
		Short: "Deploy cluster IAM and VPC resources in parallel",
		Long: `Deploy both IAM and VPC resources for a ROSA hosted cluster simultaneously.

This command runs the cluster-iam and cluster-vpc create operations in parallel,
reducing total deployment time compared to running them sequentially.

Example:
  rosactl cluster deploy my-cluster \
    --oidc-issuer-url https://d1234.cloudfront.net/my-cluster \
    --region us-east-1 \
    --availability-zones us-east-1a,us-east-1b,us-east-1c`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.clusterName = args[0]
			opts.region, _ = cmd.Flags().GetString("region")
			return runDeploy(cmd.Context(), opts)
		},
	}

	// Delegate flag registration to each sub-command package so this list
	// never diverges from the individual create commands.
	// --region is inherited from the root persistent flag; no need to add it here.
	cmdiam.AddFlags(cmd, &opts.iam)
	cmdvpc.AddFlags(cmd, &opts.vpc)

	cmd.MarkFlagRequired("oidc-issuer-url")
	cmd.MarkFlagRequired("availability-zones")

	return cmd
}

type deployResult struct {
	name string
	err  error
}

func runDeploy(ctx context.Context, opts *deployOptions) error {
	if opts.region == "" {
		return fmt.Errorf("--region is required")
	}

	// Wire shared fields into the sub-options before dispatch.
	opts.iam.ClusterName = opts.clusterName
	opts.iam.Region = opts.region
	opts.vpc.ClusterName = opts.clusterName
	opts.vpc.Region = opts.region

	fmt.Printf("Deploying cluster resources for: %s\n", opts.clusterName)
	fmt.Printf("  Region:             %s\n", opts.region)
	fmt.Printf("  OIDC Issuer:        %s\n", opts.iam.OIDCIssuerURL)
	fmt.Printf("  VPC CIDR:           %s\n", opts.vpc.VpcCidr)
	fmt.Printf("  Availability Zones: %s\n", opts.vpc.AvailabilityZones)
	fmt.Println()
	fmt.Println("Running IAM and VPC deployments in parallel...")
	fmt.Println()

	// Use a cancellable context so that if one deployment fails the other is
	// aborted rather than running to completion unnecessarily.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan deployResult, 2)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := cmdiam.RunCreate(ctx, &opts.iam)
		if err != nil {
			cancel()
		}
		results <- deployResult{"IAM", err}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := cmdvpc.RunCreate(ctx, &opts.vpc)
		if err != nil {
			cancel()
		}
		results <- deployResult{"VPC", err}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var errs []string
	for result := range results {
		if result.err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", result.name, result.err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("deployment failed:\n  %s", strings.Join(errs, "\n  "))
	}

	fmt.Println()
	fmt.Println("Cluster deployment complete. IAM and VPC resources are ready.")
	return nil
}
