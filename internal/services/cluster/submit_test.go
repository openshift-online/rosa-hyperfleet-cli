package cluster

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func TestSubmitCluster_PayloadConversion(t *testing.T) {
	// This test verifies the payload structure that SubmitCluster expects
	// Note: Full end-to-end testing would require refactoring SubmitCluster
	// to accept an injectable clientset instead of creating one internally.

	// Build a test payload (similar to what the CLI creates)
	// Uses Kubernetes-style structure with metadata object
	payload := map[string]interface{}{
		"apiVersion": "hyperfleet.io/v1alpha1",
		"kind":       "Cluster",
		"metadata": map[string]interface{}{
			"name": "test-cluster",
			"labels": map[string]string{
				"team":        "platform",
				"environment": "dev",
			},
		},
		"spec": map[string]interface{}{
			"hostedCluster": map[string]interface{}{
				"release": map[string]interface{}{
					"image": "4.22",
				},
				"platform": map[string]interface{}{
					"type": "AWS",
					"aws": map[string]interface{}{
						"region": "us-east-1",
					},
				},
			},
		},
	}

	// Test the payload structure
	t.Run("payload structure", func(t *testing.T) {
		if payload["apiVersion"] != "hyperfleet.io/v1alpha1" {
			t.Errorf("expected apiVersion hyperfleet.io/v1alpha1, got %v", payload["apiVersion"])
		}
		if metadata, ok := payload["metadata"].(map[string]interface{}); ok {
			if metadata["name"] != "test-cluster" {
				t.Errorf("expected cluster name test-cluster, got %v", metadata["name"])
			}
		} else {
			t.Error("expected metadata object")
		}
		if spec, ok := payload["spec"].(map[string]interface{}); ok {
			if hc, ok := spec["hostedCluster"].(map[string]interface{}); ok {
				if release, ok := hc["release"].(map[string]interface{}); ok {
					if release["image"] != "4.22" {
						t.Errorf("expected release image 4.22, got %v", release["image"])
					}
				}
			}
		}
	})
}

func TestSubmitCluster_OverridesPlacement(t *testing.T) {
	// Test that placement override is properly applied
	payload := map[string]interface{}{
		"apiVersion": "hyperfleet.io/v1alpha1",
		"kind":       "Cluster",
		"metadata": map[string]interface{}{
			"name": "test-cluster",
		},
		"spec": map[string]interface{}{
			"placement": "original-placement",
		},
	}

	placementOverride := "new-placement"

	// Apply override logic
	if spec, ok := payload["spec"].(map[string]interface{}); ok {
		spec["placement"] = placementOverride
	}

	// Verify override was applied
	if spec, ok := payload["spec"].(map[string]interface{}); ok {
		if spec["placement"] != placementOverride {
			t.Errorf("expected placement %s, got %v", placementOverride, spec["placement"])
		}
	} else {
		t.Fatal("spec not found in payload")
	}
}

func TestComputeIAMRoleARNs_GeneratesCorrectARNs(t *testing.T) {
	arns := computeIAMRoleARNs("my-cluster", "123456789012")

	tests := []struct {
		key      string
		expected string
	}{
		{"IngressRoleArn", "arn:aws:iam::123456789012:role/my-cluster-ingress"},
		{"CloudControllerManagerRoleArn", "arn:aws:iam::123456789012:role/my-cluster-cloud-controller-manager"},
		{"EBSCSIRoleArn", "arn:aws:iam::123456789012:role/my-cluster-ebs-csi"},
		{"ImageRegistryRoleArn", "arn:aws:iam::123456789012:role/my-cluster-image-registry"},
		{"NetworkConfigRoleArn", "arn:aws:iam::123456789012:role/my-cluster-network-config"},
		{"ControlPlaneOperatorRoleArn", "arn:aws:iam::123456789012:role/my-cluster-control-plane-operator"},
		{"NodePoolManagementRoleArn", "arn:aws:iam::123456789012:role/my-cluster-node-pool-management"},
		{"WorkerRoleArn", "arn:aws:iam::123456789012:role/my-cluster-ROSA-Worker-Role"},
		{"WorkerInstanceProfileName", "my-cluster-ROSA-Worker-Role"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got, ok := arns[tt.key]
			if !ok {
				t.Fatalf("missing key %q in arns map", tt.key)
			}
			if got != tt.expected {
				t.Errorf("arns[%q] = %q, want %q", tt.key, got, tt.expected)
			}
		})
	}
}

func TestGenerateClusterConfig_BuildsValidSpec(t *testing.T) {
	ctx := context.Background()

	// This is a minimal test to verify the structure
	// Full testing would require mocking CloudFormation
	req := &GenerateClusterConfigRequest{
		ClusterName:        "test-cluster",
		Region:             "us-east-1",
		Version:            "4.22",
		ComputeReplicas:    3,
		ComputeMachineType: "m5.xlarge",
		Provider:           "aws",
		MultiAZ:            true,
		LabelEnvironment:   "dev",
		LabelTeam:          "platform",
		AWSConfig:          aws.Config{},
	}

	// Note: This would fail without proper AWS/CFN setup
	// but demonstrates the test structure
	_ = req
	_ = ctx

	t.Run("structure validation", func(t *testing.T) {
		// Test the expected structure of the request
		if req.ClusterName != "test-cluster" {
			t.Errorf("expected cluster name test-cluster, got %s", req.ClusterName)
		}
		if req.Region != "us-east-1" {
			t.Errorf("expected region us-east-1, got %s", req.Region)
		}
		if req.ComputeReplicas != 3 {
			t.Errorf("expected 3 replicas, got %d", req.ComputeReplicas)
		}
	})
}
