package nodepool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	v1alpha1 "github.com/openshift-online/rosa-hyperfleet-api/api/v1alpha1/public"
	"github.com/openshift-online/rosa-hyperfleet-api/clientset/platform"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws/cloudformation"
	platformclient "github.com/openshift-online/rosa-regional-platform-cli/internal/platform"
	"github.com/spf13/cobra"
)

type createOptions struct {
	name            string
	clusterID       string
	replicas        int
	instanceType    string
	subnetID        string
	instanceProfile string
	securityGroups  string
	output          string
}

func newCreateCommand() *cobra.Command {
	opts := &createOptions{
		replicas:     2,
		instanceType: "m6a.xlarge",
	}

	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "Create a node pool for a cluster",
		Long: `Create a node pool for a ROSA hosted cluster.

If --subnet-id, --instance-profile, and --security-groups are omitted,
they are auto-discovered from the cluster's spec.

Examples:
  # Create with defaults (auto-discover infra from cluster)
  rosactl nodepool create my-nodepool --cluster-id <id> --region us-east-1

  # Create with explicit settings
  rosactl nodepool create my-nodepool --cluster-id <id> --replicas 3 --instance-type m5.2xlarge`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.name = args[0]
			if opts.clusterID == "" {
				return fmt.Errorf("--cluster-id is required")
			}
			if opts.replicas < 1 {
				return fmt.Errorf("--replicas must be at least 1")
			}
			return runCreate(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.clusterID, "cluster-id", "", "Cluster ID (required)")
	cmd.Flags().IntVar(&opts.replicas, "replicas", opts.replicas, "Number of worker replicas")
	cmd.Flags().StringVar(&opts.instanceType, "instance-type", opts.instanceType, "EC2 instance type")
	cmd.Flags().StringVar(&opts.subnetID, "subnet-id", "", "Subnet ID (auto-discovered from cluster if omitted)")
	cmd.Flags().StringVar(&opts.instanceProfile, "instance-profile", "", "IAM instance profile (auto-discovered from cluster if omitted)")
	cmd.Flags().StringVar(&opts.securityGroups, "security-groups", "", "Comma-separated security group IDs (auto-discovered from cluster if omitted)")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "", "Output format (json)")

	return cmd
}

func runCreate(ctx context.Context, opts *createOptions) error {
	cfg, err := aws.NewConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	if cfg.Region == "" {
		return aws.ErrRegionRequired
	}

	// Create the clientset
	cs, err := platformclient.NewClientset(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	// Auto-discover infra from cluster spec and CloudFormation stacks.
	if opts.subnetID == "" || opts.instanceProfile == "" || opts.securityGroups == "" {
		cluster, err := cs.HyperfleetV1alpha1().Clusters().Get(ctx, opts.clusterID, platform.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to fetch cluster spec for auto-discovery: %w", err)
		}

		if opts.subnetID == "" {
			opts.subnetID = extractSubnetFromClusterSpec(cluster)
		}
		if opts.instanceProfile == "" || opts.securityGroups == "" {
			cfnClient := cloudformation.NewClient(cfg)
			clusterName := cluster.Name
			if opts.instanceProfile == "" {
				iamStackName := fmt.Sprintf("rosa-%s-iam", clusterName)
				iamStack, err := cfnClient.DescribeStack(ctx, iamStackName)
				if err != nil {
					return fmt.Errorf("failed to describe stack %s: %w", iamStackName, err)
				}
				opts.instanceProfile = iamStack.Outputs["WorkerInstanceProfileName"]
			}
			if opts.securityGroups == "" {
				vpcStackName := fmt.Sprintf("rosa-%s-vpc", clusterName)
				vpcStack, err := cfnClient.DescribeStack(ctx, vpcStackName)
				if err != nil {
					return fmt.Errorf("failed to describe stack %s: %w", vpcStackName, err)
				}
				opts.securityGroups = vpcStack.Outputs["WorkerSecurityGroupId"]
			}
		}
	}

	if opts.subnetID == "" {
		return fmt.Errorf("--subnet-id is required (could not auto-discover from cluster)")
	}
	if opts.instanceProfile == "" {
		return fmt.Errorf("--instance-profile is required (could not auto-discover from cluster)")
	}
	if opts.securityGroups == "" {
		return fmt.Errorf("--security-groups is required (could not auto-discover from cluster)")
	}

	sgParts := strings.Split(opts.securityGroups, ",")
	sgRefs := make([]map[string]interface{}, 0, len(sgParts))
	for _, sg := range sgParts {
		if id := strings.TrimSpace(sg); id != "" {
			sgRefs = append(sgRefs, map[string]interface{}{"id": id})
		}
	}

	// Build the payload with Kubernetes-style structure
	payload := map[string]interface{}{
		"apiVersion": "hyperfleet.io/v1alpha1",
		"kind":       "NodePool",
		"metadata": map[string]interface{}{
			"name":      opts.name,
			"namespace": opts.clusterID, // NodePools are namespaced by cluster ID
		},
		"spec": map[string]interface{}{
			"nodePool": map[string]interface{}{
				"replicas": opts.replicas,
				"platform": map[string]interface{}{
					"type": "AWS",
					"aws": map[string]interface{}{
						"instanceType":    opts.instanceType,
						"instanceProfile": opts.instanceProfile,
						"subnet":          map[string]interface{}{"id": opts.subnetID},
						"securityGroups":  sgRefs,
					},
				},
			},
		},
	}

	// Convert map to typed NodePool via JSON marshal/unmarshal
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	var nodepool v1alpha1.NodePool
	if err := json.Unmarshal(payloadBytes, &nodepool); err != nil {
		return fmt.Errorf("failed to unmarshal into NodePool type: %w", err)
	}

	// Create the nodepool via SDK
	// Note: NodePools are namespaced by cluster ID
	created, err := cs.HyperfleetV1alpha1().NodePools(opts.clusterID).Create(ctx, &nodepool, platform.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create nodepool via SDK: %w", err)
	}

	// Convert response to map for display compatibility
	responseBytes, err := json.Marshal(created)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(responseBytes, &result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if opts.output == "json" {
		prettyJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(prettyJSON))
		return nil
	}

	fmt.Fprintf(os.Stderr, "\n✓ NodePool created successfully\n")
	fmt.Fprintf(os.Stderr, "\nNodePool Details:\n")
	fmt.Fprintf(os.Stderr, "  Name:          %s\n", created.Name)
	fmt.Fprintf(os.Stderr, "  ID:            %s\n", string(created.UID))
	fmt.Fprintf(os.Stderr, "  Cluster:       %s\n", opts.clusterID)
	fmt.Fprintf(os.Stderr, "  Replicas:      %d\n", opts.replicas)
	fmt.Fprintf(os.Stderr, "  Instance Type: %s\n", opts.instanceType)

	return nil
}

func extractSubnetFromClusterSpec(cluster *v1alpha1.Cluster) string {
	if cluster == nil {
		return ""
	}

	awsPlat := cluster.Spec.HostedCluster.Platform.AWS
	if awsPlat == nil {
		return ""
	}

	cpc := awsPlat.CloudProviderConfig
	if cpc == nil || cpc.Subnet == nil || cpc.Subnet.ID == nil {
		return ""
	}

	return *cpc.Subnet.ID
}
