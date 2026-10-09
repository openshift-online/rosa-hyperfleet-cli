package nodepool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	v1alpha1 "github.com/openshift-online/rosa-hyperfleet-api/api/v1alpha1/public"
	hyperfleet "github.com/openshift-online/rosa-hyperfleet-api/clientset"
	"github.com/openshift-online/rosa-hyperfleet-api/clientset/platform"
	hfrest "github.com/openshift-online/rosa-hyperfleet-api/clientset/rest"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws/cloudformation"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/config"
	"github.com/spf13/cobra"
)

type createOptions struct {
	name            string
	clusterName     string
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
  rosactl nodepool create my-cluster.my-nodepool --cluster-name my-cluster --region us-east-1

  # Create with explicit settings
  rosactl nodepool create my-cluster.my-nodepool --cluster-name my-cluster --replicas 3 --instance-type m5.2xlarge`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.name = args[0]
			if opts.clusterName == "" {
				return fmt.Errorf("--cluster-name is required")
			}
			if !strings.HasPrefix(opts.name, opts.clusterName+".") {
				return fmt.Errorf("nodepool %q does not belong to cluster %q", opts.name, opts.clusterName)
			}
			if opts.replicas < 1 {
				return fmt.Errorf("--replicas must be at least 1")
			}
			return runCreate(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.clusterName, "cluster-name", "", "Account-scoped Cluster name (required)")
	cmd.Flags().IntVar(&opts.replicas, "replicas", opts.replicas, "Number of worker replicas")
	cmd.Flags().StringVar(&opts.instanceType, "instance-type", opts.instanceType, "EC2 instance type")
	cmd.Flags().StringVar(&opts.subnetID, "subnet-id", "", "Subnet ID (auto-discovered from cluster if omitted)")
	cmd.Flags().StringVar(&opts.instanceProfile, "instance-profile", "", "IAM instance profile (auto-discovered from cluster if omitted)")
	cmd.Flags().StringVar(&opts.securityGroups, "security-groups", "", "Comma-separated security group IDs (auto-discovered from cluster if omitted)")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "", "Output format (json)")

	return cmd
}

func runCreate(ctx context.Context, opts *createOptions) error {
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
		return aws.ErrRegionRequired
	}

	// Auto-discover infra from cluster spec and CloudFormation stacks.
	if opts.subnetID == "" || opts.instanceProfile == "" || opts.securityGroups == "" {
		cluster, err := fetchClusterSpec(ctx, baseURL, opts.clusterName, creds, region)
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

	payload := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": opts.name,
			// The platform API assigns the authenticated account namespace on write.
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

	// Convert map to v1alpha1.NodePool
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	var nodepool v1alpha1.NodePool
	if err := json.Unmarshal(payloadBytes, &nodepool); err != nil {
		return fmt.Errorf("failed to unmarshal nodepool: %w", err)
	}

	// Load AWS config for clientset
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	accountID, err := config.GetAccountID()
	if err != nil {
		return fmt.Errorf("failed to get account ID: %w", err)
	}

	// Create clientset
	cs, err := hyperfleet.NewForConfig(&hfrest.Config{
		Host:      baseURL,
		AccountID: accountID,
		AWSConfig: awsCfg,
	})
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	// An empty client namespace keeps the account ID from client configuration on
	// the request; the API sets the canonical account namespace on write.
	createdNodepool, err := cs.HyperfleetV1alpha1().NodePools().Create(ctx, &nodepool, platform.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create nodepool: %w", err)
	}

	if opts.output == "json" {
		prettyJSON, err := json.MarshalIndent(createdNodepool, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(prettyJSON))
		return nil
	}

	fmt.Fprintf(os.Stderr, "\n✓ NodePool created successfully\n")
	fmt.Fprintf(os.Stderr, "\nNodePool Details:\n")
	fmt.Fprintf(os.Stderr, "  Name:          %s\n", opts.name)
	fmt.Fprintf(os.Stderr, "  ID:            %s\n", string(createdNodepool.UID))
	fmt.Fprintf(os.Stderr, "  Cluster:       %s\n", opts.clusterName)
	fmt.Fprintf(os.Stderr, "  Replicas:      %d\n", opts.replicas)
	fmt.Fprintf(os.Stderr, "  Instance Type: %s\n", opts.instanceType)

	return nil
}

func extractSubnetFromClusterSpec(cluster *v1alpha1.Cluster) string {
	if cluster.Spec.HostedCluster.Platform.AWS != nil &&
		cluster.Spec.HostedCluster.Platform.AWS.CloudProviderConfig != nil &&
		cluster.Spec.HostedCluster.Platform.AWS.CloudProviderConfig.Subnet != nil &&
		cluster.Spec.HostedCluster.Platform.AWS.CloudProviderConfig.Subnet.ID != nil {
		return *cluster.Spec.HostedCluster.Platform.AWS.CloudProviderConfig.Subnet.ID
	}
	return ""
}

func fetchClusterSpec(ctx context.Context, baseURL, clusterName string, creds awssdk.Credentials, region string) (*v1alpha1.Cluster, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	accountID, err := config.GetAccountID()
	if err != nil {
		return nil, fmt.Errorf("failed to get account ID: %w", err)
	}

	cs, err := hyperfleet.NewForConfig(&hfrest.Config{
		Host:      baseURL,
		AccountID: accountID,
		AWSConfig: awsCfg,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	cluster, err := cs.HyperfleetV1alpha1().Clusters().Get(ctx, clusterName, platform.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster %s: %w", clusterName, err)
	}

	return cluster, nil
}
