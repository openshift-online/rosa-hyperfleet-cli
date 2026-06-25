package cluster

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws/cloudformation"
)

// GenerateClusterConfigRequest contains parameters for generating cluster configuration
type GenerateClusterConfigRequest struct {
	ClusterName        string
	Region             string
	TargetProjectID    string
	Version            string
	ComputeReplicas    int
	ComputeMachineType string
	PlacementCluster   string
	Provider           string
	MultiAZ            bool
	LabelEnvironment   string
	LabelTeam          string
	AWSConfig          aws.Config
}

// GenerateClusterConfigResponse contains the generated cluster configuration
type GenerateClusterConfigResponse struct {
	ClusterConfig map[string]interface{}
}

// SubmitClusterRequest contains parameters for submitting cluster to platform API
type SubmitClusterRequest struct {
	Payload           map[string]interface{}
	PlatformAPIURL    string
	PlacementOverride string // Optional - overrides placement in payload if set
	AWSConfig         aws.Config
}

// SubmitClusterResponse contains the API response
type SubmitClusterResponse struct {
	Response map[string]interface{}
}

// GenerateClusterConfig generates a cluster configuration by querying CloudFormation stacks.
// If the IAM stack does not exist yet, role ARNs are computed from the cluster name and
// AWS account ID. This enables a provisioning flow where the IAM stack is created after
// the cluster (with the OIDC issuer URL known), avoiding an IAM trust policy UPDATE and
// the ~10-15 min eventual consistency delay it causes.
func GenerateClusterConfig(ctx context.Context, req *GenerateClusterConfigRequest) (*GenerateClusterConfigResponse, error) {
	cfnClient := cloudformation.NewClient(req.AWSConfig)

	// Get IAM outputs: prefer real stack, fall back to computed ARNs
	iamOutputs, err := getIAMOutputs(ctx, cfnClient, req.AWSConfig, req.ClusterName)
	if err != nil {
		return nil, err
	}

	// Get VPC stack information
	vpcStackName := fmt.Sprintf("rosa-%s-vpc", req.ClusterName)
	vpcStack, err := cfnClient.DescribeStack(ctx, vpcStackName)
	if err != nil {
		return nil, fmt.Errorf("failed to describe VPC stack: %w", err)
	}

	// Build the spec object with base fields
	spec := map[string]interface{}{
		"provider":             req.Provider,
		"region":               req.Region,
		"version":              req.Version,
		"multi_az":             req.MultiAZ,
		"compute_machine_type": req.ComputeMachineType,
		"compute_replicas":     req.ComputeReplicas,
		"placement":            req.PlacementCluster,
	}

	// Merge IAM outputs directly into spec with camelCase keys
	for key, value := range iamOutputs {
		camelKey := toCamelCase(key)
		spec[camelKey] = value
	}

	// Merge VPC outputs directly into spec with camelCase keys
	for key, value := range vpcStack.Outputs {
		camelKey := toCamelCase(key)

		// PrivateSubnetIds comes as comma-separated string from VPC stack outputs;
		// the CRD expects []string.
		if key == "PrivateSubnetIds" {
			parts := strings.Split(value, ",")
			trimmed := make([]string, 0, len(parts))
			for _, s := range parts {
				if t := strings.TrimSpace(s); t != "" {
					trimmed = append(trimmed, t)
				}
			}
			spec[camelKey] = trimmed
			continue
		}

		spec[camelKey] = value
	}

	// Build labels
	labels := map[string]interface{}{
		"environment": req.LabelEnvironment,
		"team":        req.LabelTeam,
		"region":      req.Region,
	}

	// Build the cluster object
	clusterObj := map[string]interface{}{
		"kind":              "Cluster",
		"name":              req.ClusterName,
		"target_project_id": req.TargetProjectID,
		"labels":            labels,
		"spec":              spec,
	}

	return &GenerateClusterConfigResponse{
		ClusterConfig: clusterObj,
	}, nil
}

// getIAMOutputs returns IAM role ARNs either from the existing CloudFormation stack
// or by computing them from the cluster name and AWS account ID.
func getIAMOutputs(ctx context.Context, cfnClient *cloudformation.Client, cfg aws.Config, clusterName string) (map[string]string, error) {
	iamStackName := fmt.Sprintf("rosa-%s-iam", clusterName)
	iamStack, err := cfnClient.DescribeStack(ctx, iamStackName)
	if err != nil {
		var notFound *cloudformation.StackNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("failed to describe IAM stack: %w", err)
		}

		accountID, stsErr := getAWSAccountID(ctx, cfg)
		if stsErr != nil {
			return nil, fmt.Errorf("IAM stack not found and failed to get AWS account ID: %w", stsErr)
		}

		fmt.Printf("IAM stack %s not found — computing role ARNs from account %s\n", iamStackName, accountID)
		return computeIAMRoleARNs(clusterName, accountID), nil
	}

	return iamStack.Outputs, nil
}

func getAWSAccountID(ctx context.Context, cfg aws.Config) (string, error) {
	stsClient := sts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}
	return aws.ToString(identity.Account), nil
}

// computeIAMRoleARNs returns the same output map that the rosa-{cluster}-iam
// CloudFormation stack would produce. Role names match the template exactly.
func computeIAMRoleARNs(clusterName, accountID string) map[string]string {
	arn := func(roleName string) string {
		return fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, roleName)
	}

	return map[string]string{
		"IngressRoleArn":                arn(clusterName + "-ingress"),
		"CloudControllerManagerRoleArn": arn(clusterName + "-cloud-controller-manager"),
		"EBSCSIRoleArn":                 arn(clusterName + "-ebs-csi"),
		"ImageRegistryRoleArn":          arn(clusterName + "-image-registry"),
		"NetworkConfigRoleArn":          arn(clusterName + "-network-config"),
		"ControlPlaneOperatorRoleArn":   arn(clusterName + "-control-plane-operator"),
		"NodePoolManagementRoleArn":     arn(clusterName + "-node-pool-management"),
		"WorkerRoleArn":                 arn(clusterName + "-ROSA-Worker-Role"),
		"WorkerInstanceProfileName":     clusterName + "-ROSA-Worker-Role",
	}
}

// SubmitCluster submits a cluster configuration to the platform API
func SubmitCluster(ctx context.Context, req *SubmitClusterRequest) (*SubmitClusterResponse, error) {
	// Make a copy of the payload to avoid modifying the original
	payload := make(map[string]interface{})
	for k, v := range req.Payload {
		payload[k] = v
	}

	// Override placement if specified
	if req.PlacementOverride != "" {
		if spec, ok := payload["spec"].(map[string]interface{}); ok {
			spec["placement"] = req.PlacementOverride
		}
	}

	// Marshal payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Build the API endpoint URL
	endpoint := fmt.Sprintf("%s/api/v0/clusters", req.PlatformAPIURL)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Calculate SHA256 hash of the request body
	hash := sha256.Sum256(payloadBytes)
	payloadHash := hex.EncodeToString(hash[:])

	// Sign the request with AWS SigV4
	signer := v4.NewSigner()
	creds, err := req.AWSConfig.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve AWS credentials: %w", err)
	}

	// Determine the region from AWS config
	region := req.AWSConfig.Region
	if region == "" {
		region = "us-east-1" // Default region
	}

	err = signer.SignHTTP(ctx, creds, httpReq, payloadHash, "execute-api", region, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	// Execute the request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error responses
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the JSON response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &SubmitClusterResponse{
		Response: result,
	}, nil
}

// toCamelCase converts a PascalCase string to camelCase
// Examples: "VpcId" -> "vpcId", "OIDCProviderArn" -> "oidcProviderArn"
func toCamelCase(s string) string {
	if s == "" {
		return s
	}

	runes := []rune(s)

	// Find the end of the leading uppercase sequence
	// For "OIDCProviderArn", we want to lowercase "OIDC" but keep "P" uppercase
	uppercaseEnd := 0
	for i := 0; i < len(runes); i++ {
		if !unicode.IsUpper(runes[i]) {
			// Found a non-uppercase character
			break
		}
		uppercaseEnd = i
	}

	// If we have multiple uppercase letters followed by a lowercase letter,
	// we should keep the last uppercase as-is (it starts the next word)
	// e.g., "OIDCProvider" -> uppercase until 'P', keep 'P' uppercase
	if uppercaseEnd > 0 && uppercaseEnd+1 < len(runes) && unicode.IsLower(runes[uppercaseEnd+1]) {
		uppercaseEnd--
	}

	// Convert the leading uppercase sequence to lowercase
	var result strings.Builder
	for i := 0; i <= uppercaseEnd; i++ {
		result.WriteRune(unicode.ToLower(runes[i]))
	}

	// Append the rest unchanged
	for i := uppercaseEnd + 1; i < len(runes); i++ {
		result.WriteRune(runes[i])
	}

	return result.String()
}
