package cluster

import (
	"testing"
)

func TestComputeIAMRoleARNs(t *testing.T) {
	arns := computeIAMRoleARNs("my-cluster", "123456789012")

	expected := map[string]string{
		"IngressRoleArn":                "arn:aws:iam::123456789012:role/my-cluster-ingress",
		"CloudControllerManagerRoleArn": "arn:aws:iam::123456789012:role/my-cluster-cloud-controller-manager",
		"EBSCSIRoleArn":                 "arn:aws:iam::123456789012:role/my-cluster-ebs-csi",
		"ImageRegistryRoleArn":          "arn:aws:iam::123456789012:role/my-cluster-image-registry",
		"NetworkConfigRoleArn":          "arn:aws:iam::123456789012:role/my-cluster-network-config",
		"ControlPlaneOperatorRoleArn":   "arn:aws:iam::123456789012:role/my-cluster-control-plane-operator",
		"NodePoolManagementRoleArn":     "arn:aws:iam::123456789012:role/my-cluster-node-pool-management",
		"WorkerRoleArn":                 "arn:aws:iam::123456789012:role/my-cluster-ROSA-Worker-Role",
		"WorkerInstanceProfileName":     "my-cluster-ROSA-Worker-Role",
	}

	if len(arns) != len(expected) {
		t.Fatalf("got %d outputs, want %d", len(arns), len(expected))
	}

	for key, want := range expected {
		got, ok := arns[key]
		if !ok {
			t.Errorf("missing key %q", key)
			continue
		}
		if got != want {
			t.Errorf("arns[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestComputeIAMRoleARNs_KeysMatchCamelCase(t *testing.T) {
	arns := computeIAMRoleARNs("test", "000000000000")

	for key := range arns {
		camel := toCamelCase(key)
		if camel == "" {
			t.Errorf("toCamelCase(%q) returned empty", key)
		}
	}
}

func TestToCamelCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"VpcId", "vpcId"},
		{"SubnetIds", "subnetIds"},
		{"OIDCProviderArn", "oidcProviderArn"},
		{"OIDCProviderURL", "oidcProviderURL"},
		{"WorkerRoleArn", "workerRoleArn"},
		{"WorkerInstanceProfileName", "workerInstanceProfileName"},
		{"ControlPlaneRoleArn", "controlPlaneRoleArn"},
		{"", ""},
		{"A", "a"},
		{"ABC", "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toCamelCase(tt.input)
			if result != tt.expected {
				t.Errorf("toCamelCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
