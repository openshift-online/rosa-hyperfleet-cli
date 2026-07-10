package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/client"
	clusterservice "github.com/openshift-online/rosa-regional-platform-cli/internal/services/cluster"
	"github.com/spf13/cobra"
)

type createOptions struct {
	clusterName        string
	region             string
	targetProjectID    string
	version            string
	computeReplicas    int
	computeMachineType string
	placementCluster   string
	provider           string
	multiAZ            bool
	labelEnvironment   string
	labelTeam          string
	dryRun             bool
	outputFile         string
	payloadFile        string
	output             string
}

func newCreateCommand() *cobra.Command {
	opts := &createOptions{
		region:             "us-east-1",
		targetProjectID:    "",
		version:            "4.22",
		computeReplicas:    3,
		computeMachineType: "m5.xlarge",
		placementCluster:   "",
		provider:           "aws",
		multiAZ:            true,
		labelEnvironment:   "dev",
		labelTeam:          "platform",
	}

	cmd := &cobra.Command{
		Use:   "create CLUSTER_NAME",
		Short: "Create a cluster configuration and submit to platform API",
		Long: `Create a cluster configuration by gathering IAM and VPC information
from CloudFormation stacks, then submit to the platform API.

Three modes:
1. Default: Generate config from CloudFormation stacks AND submit to platform API
2. --dry-run: Only generate cluster configuration without submitting
3. --payload: Only submit an existing cluster configuration file

Examples:
  # Generate config and create cluster (default)
  rosactl cluster create my-cluster --region us-east-1

  # Generate config only (dry-run mode)
  rosactl cluster create my-cluster --region us-east-1 --dry-run
  rosactl cluster create my-cluster --region us-east-1 --dry-run --output-file my-cluster.json

  # Submit existing payload
  rosactl cluster create my-cluster --region us-east-1 --payload my-cluster.json
  rosactl cluster create my-cluster --region us-east-1 --payload my-cluster.json --placement mgmt-cluster-01`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.clusterName = args[0]

			// Validate mode: cannot use both --dry-run and --payload
			if opts.dryRun && opts.payloadFile != "" {
				return fmt.Errorf("cannot use both --dry-run and --payload flags")
			}

			// Set default output file only in dry-run mode
			if opts.dryRun && opts.payloadFile == "" && opts.outputFile == "" {
				opts.outputFile = fmt.Sprintf("%s-cluster.json", opts.clusterName)
			}

			return runCreate(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.region, "region", opts.region, "AWS region")
	cmd.Flags().StringVar(&opts.targetProjectID, "target-project-id", opts.targetProjectID, "Target project ID (dry-run mode only)")
	cmd.Flags().StringVar(&opts.version, "version", opts.version, "OpenShift version (dry-run mode only)")
	cmd.Flags().IntVar(&opts.computeReplicas, "compute-replicas", opts.computeReplicas, "Number of compute replicas (dry-run mode only)")
	cmd.Flags().StringVar(&opts.computeMachineType, "compute-machine-type", opts.computeMachineType, "Compute machine type (dry-run mode only)")
	cmd.Flags().StringVar(&opts.placementCluster, "placement", opts.placementCluster, "Management cluster name (overrides payload value)")
	cmd.Flags().StringVar(&opts.provider, "provider", opts.provider, "Cloud provider (dry-run mode only)")
	cmd.Flags().BoolVar(&opts.multiAZ, "multi-az", opts.multiAZ, "Enable multi-AZ deployment (dry-run mode only)")
	cmd.Flags().StringVar(&opts.labelEnvironment, "label-environment", opts.labelEnvironment, "Environment label (dry-run mode only)")
	cmd.Flags().StringVar(&opts.labelTeam, "label-team", opts.labelTeam, "Team label (dry-run mode only)")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Generate cluster configuration without submitting to API")
	cmd.Flags().StringVar(&opts.outputFile, "output-file", "", "Output file for cluster configuration (default in dry-run: <cluster-name>-cluster.json)")
	cmd.Flags().StringVar(&opts.payloadFile, "payload", "", "JSON payload file to POST to platform API")
	cmd.Flags().StringVar(&opts.output, "output", "", "Output format (json)")

	return cmd
}

func runCreate(ctx context.Context, opts *createOptions) error {
	if opts.payloadFile != "" {
		// Payload mode: POST existing file to platform API
		return runCreateWithPayload(ctx, opts)
	}

	if opts.dryRun {
		// Dry-run mode: Generate configuration only
		return runCreateDryRun(ctx, opts)
	}

	// Default mode: Generate configuration AND submit to platform API
	return runCreateAndSubmit(ctx, opts)
}

func printClusterSummary(response map[string]interface{}) {
	// Extract key fields from the response
	name := getStringField(response, "name")
	id := getStringField(response, "id")
	version := getStringField(response, "version")

	// Get cloudUrl from spec
	cloudURL := ""
	if spec, ok := response["spec"].(map[string]interface{}); ok {
		cloudURL = getStringField(spec, "cloudUrl")
	}

	// Print summary
	fmt.Println("\n✓ Cluster created successfully")
	fmt.Printf("\nCluster Details:\n")
	if name != "" {
		fmt.Printf("  Name:      %s\n", name)
	}
	if id != "" {
		fmt.Printf("  ID:        %s\n", id)
	}
	if version != "" {
		fmt.Printf("  Version:   %s\n", version)
	}
	if cloudURL != "" {
		fmt.Printf("  Cloud URL: %s\n", cloudURL)
	}
}

func getStringField(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal
		}
	}
	return ""
}

func runCreateDryRun(ctx context.Context, opts *createOptions) error {
	// Load AWS config
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(opts.region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Build service request
	req := &clusterservice.GenerateClusterConfigRequest{
		ClusterName:        opts.clusterName,
		Region:             opts.region,
		TargetProjectID:    opts.targetProjectID,
		Version:            opts.version,
		ComputeReplicas:    opts.computeReplicas,
		ComputeMachineType: opts.computeMachineType,
		PlacementCluster:   opts.placementCluster,
		Provider:           opts.provider,
		MultiAZ:            opts.multiAZ,
		LabelEnvironment:   opts.labelEnvironment,
		LabelTeam:          opts.labelTeam,
		AWSConfig:          cfg,
	}

	// Generate cluster configuration
	resp, err := clusterservice.GenerateClusterConfig(ctx, req)
	if err != nil {
		return err
	}

	// Convert to JSON
	jsonBytes, err := json.MarshalIndent(resp.ClusterConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cluster object: %w", err)
	}

	// Print to stdout
	fmt.Println(string(jsonBytes))

	// Save to file
	if err := os.WriteFile(opts.outputFile, jsonBytes, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	// Print confirmation message to stderr so it doesn't interfere with stdout JSON
	fmt.Fprintf(os.Stderr, "\n✓ Cluster configuration saved to: %s\n", opts.outputFile)

	return nil
}

func runCreateAndSubmit(ctx context.Context, opts *createOptions) error {
	// Load AWS config (needed for CloudFormation calls in GenerateClusterConfig)
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(opts.region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Build service request to generate cluster config
	genReq := &clusterservice.GenerateClusterConfigRequest{
		ClusterName:        opts.clusterName,
		Region:             opts.region,
		TargetProjectID:    opts.targetProjectID,
		Version:            opts.version,
		ComputeReplicas:    opts.computeReplicas,
		ComputeMachineType: opts.computeMachineType,
		PlacementCluster:   opts.placementCluster,
		Provider:           opts.provider,
		MultiAZ:            opts.multiAZ,
		LabelEnvironment:   opts.labelEnvironment,
		LabelTeam:          opts.labelTeam,
		AWSConfig:          cfg,
	}

	// Generate cluster configuration
	genResp, err := clusterservice.GenerateClusterConfig(ctx, genReq)
	if err != nil {
		return err
	}

	// Optionally save to file if output file was explicitly specified
	if opts.outputFile != "" {
		jsonBytes, err := json.MarshalIndent(genResp.ClusterConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal cluster object: %w", err)
		}

		if err := os.WriteFile(opts.outputFile, jsonBytes, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}

		fmt.Fprintf(os.Stderr, "✓ Cluster configuration saved to: %s\n", opts.outputFile)
	}

	c, err := client.New(ctx)
	if err != nil {
		return err
	}

	submitReq := &clusterservice.SubmitClusterRequest{
		Payload: genResp.ClusterConfig,
		Client:  c,
	}

	if opts.output != "json" {
		fmt.Fprintf(os.Stderr, "Submitting cluster to platform API...\n")
	}

	submitResp, err := clusterservice.SubmitCluster(ctx, submitReq)
	if err != nil {
		return err
	}

	if opts.output == "json" {
		jsonBytes, err := json.MarshalIndent(submitResp.Response, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal response: %w", err)
		}
		fmt.Println(string(jsonBytes))
	} else {
		printClusterSummary(submitResp.Response)
	}

	return nil
}

func runCreateWithPayload(ctx context.Context, opts *createOptions) error {
	payloadBytes, err := os.ReadFile(opts.payloadFile)
	if err != nil {
		return fmt.Errorf("failed to read payload file: %w", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return fmt.Errorf("invalid JSON in payload file: %w", err)
	}

	if currentName, ok := payload["name"].(string); ok && currentName != opts.clusterName {
		fmt.Fprintf(os.Stderr, "Overriding cluster name: %s → %s\n", currentName, opts.clusterName)
	}
	payload["name"] = opts.clusterName

	c, err := client.New(ctx)
	if err != nil {
		return err
	}

	var placementOverride string
	if opts.placementCluster != "" {
		if spec, ok := payload["spec"].(map[string]interface{}); ok {
			currentPlacement := spec["placement"]
			if currentPlacement != opts.placementCluster {
				placementOverride = opts.placementCluster
				fmt.Fprintf(os.Stderr, "Overriding placement: %v → %s\n", currentPlacement, opts.placementCluster)
			}
		}
	}

	req := &clusterservice.SubmitClusterRequest{
		Payload:           payload,
		Client:            c,
		PlacementOverride: placementOverride,
	}

	resp, err := clusterservice.SubmitCluster(ctx, req)
	if err != nil {
		return err
	}

	if opts.output == "json" {
		jsonBytes, err := json.MarshalIndent(resp.Response, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal response: %w", err)
		}
		fmt.Println(string(jsonBytes))
	} else {
		printClusterSummary(resp.Response)
	}

	return nil
}
