package nodepool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws/cloudformation"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/config"
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

	// Auto-discover infra from cluster spec and CloudFormation stacks.
	if opts.subnetID == "" || opts.instanceProfile == "" || opts.securityGroups == "" {
		cluster, err := fetchClusterSpec(ctx, baseURL, opts.clusterID, creds, region)
		if err != nil {
			return fmt.Errorf("failed to fetch cluster spec for auto-discovery: %w", err)
		}
		if opts.subnetID == "" {
			opts.subnetID = extractSubnetFromClusterSpec(cluster.Spec)
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
		"cluster_id": opts.clusterID,
		"name":       opts.name,
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

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v0/nodepools", baseURL)
	body, statusCode, err := signedPost(ctx, endpoint, payloadBytes, creds, region)
	if err != nil {
		return err
	}

	if statusCode != 201 {
		return fmt.Errorf("API request failed with status %d: %s", statusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if opts.output == "json" {
		prettyJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(prettyJSON))
		return nil
	}

	fmt.Fprintf(os.Stderr, "\n✓ NodePool created successfully\n")
	fmt.Fprintf(os.Stderr, "\nNodePool Details:\n")
	fmt.Fprintf(os.Stderr, "  Name:          %s\n", opts.name)
	if id, ok := result["id"].(string); ok {
		fmt.Fprintf(os.Stderr, "  ID:            %s\n", id)
	}
	fmt.Fprintf(os.Stderr, "  Cluster:       %s\n", opts.clusterID)
	fmt.Fprintf(os.Stderr, "  Replicas:      %d\n", opts.replicas)
	fmt.Fprintf(os.Stderr, "  Instance Type: %s\n", opts.instanceType)

	return nil
}

type clusterResponse struct {
	ID   string                 `json:"id"`
	Name string                 `json:"name"`
	Spec map[string]interface{} `json:"spec"`
}

func extractSubnetFromClusterSpec(spec map[string]interface{}) string {
	hc, _ := spec["hostedCluster"].(map[string]interface{})
	if hc == nil {
		return ""
	}
	platform, _ := hc["platform"].(map[string]interface{})
	if platform == nil {
		return ""
	}
	awsPlat, _ := platform["aws"].(map[string]interface{})
	if awsPlat == nil {
		return ""
	}
	cpc, _ := awsPlat["cloudProviderConfig"].(map[string]interface{})
	if cpc == nil {
		return ""
	}
	subnet, _ := cpc["subnet"].(map[string]interface{})
	if subnet == nil {
		return ""
	}
	id, _ := subnet["id"].(string)
	return id
}

func fetchClusterSpec(ctx context.Context, baseURL, clusterID string, creds awssdk.Credentials, region string) (*clusterResponse, error) {
	endpoint := fmt.Sprintf("%s/api/v0/clusters/%s", baseURL, url.PathEscape(clusterID))
	body, statusCode, err := signedGet(ctx, endpoint, creds, region)
	if err != nil {
		return nil, err
	}
	if statusCode != 200 {
		return nil, fmt.Errorf("failed to get cluster %s: status %d: %s", clusterID, statusCode, string(body))
	}

	var cluster clusterResponse
	if err := json.Unmarshal(body, &cluster); err != nil {
		return nil, fmt.Errorf("failed to parse cluster response: %w", err)
	}
	return &cluster, nil
}
