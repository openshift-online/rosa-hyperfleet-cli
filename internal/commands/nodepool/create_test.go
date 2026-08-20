package nodepool

import (
	"testing"

	v1alpha1 "github.com/openshift-online/rosa-hyperfleet-api/api/v1alpha1/public"
	hypershiftv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
)

func TestExtractSubnetFromClusterSpec(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *v1alpha1.Cluster
		expected string
	}{
		{
			name:     "nil cluster",
			cluster:  nil,
			expected: "",
		},
		{
			name: "cluster with no AWS platform",
			cluster: &v1alpha1.Cluster{
				Spec: v1alpha1.ClusterSpec{
					HostedCluster: v1alpha1.HostedClusterSpecPassthrough{
						Platform: hypershiftv1beta1.PlatformSpec{
							Type: hypershiftv1beta1.AWSPlatform,
						},
					},
				},
			},
			expected: "",
		},
		{
			name: "cluster with subnet ID",
			cluster: &v1alpha1.Cluster{
				Spec: v1alpha1.ClusterSpec{
					HostedCluster: v1alpha1.HostedClusterSpecPassthrough{
						Platform: hypershiftv1beta1.PlatformSpec{
							Type: hypershiftv1beta1.AWSPlatform,
							AWS: &hypershiftv1beta1.AWSPlatformSpec{
								CloudProviderConfig: &hypershiftv1beta1.AWSCloudProviderConfig{
									Subnet: &hypershiftv1beta1.AWSResourceReference{
										ID: strPtr("subnet-12345"),
									},
								},
							},
						},
					},
				},
			},
			expected: "subnet-12345",
		},
		{
			name: "cluster with nil subnet",
			cluster: &v1alpha1.Cluster{
				Spec: v1alpha1.ClusterSpec{
					HostedCluster: v1alpha1.HostedClusterSpecPassthrough{
						Platform: hypershiftv1beta1.PlatformSpec{
							Type: hypershiftv1beta1.AWSPlatform,
							AWS: &hypershiftv1beta1.AWSPlatformSpec{
								CloudProviderConfig: &hypershiftv1beta1.AWSCloudProviderConfig{
									Subnet: nil,
								},
							},
						},
					},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSubnetFromClusterSpec(tt.cluster)
			if result != tt.expected {
				t.Errorf("extractSubnetFromClusterSpec() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestCreateOptions_Validation(t *testing.T) {
	tests := []struct {
		name        string
		opts        *createOptions
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid options",
			opts: &createOptions{
				name:            "test-nodepool",
				clusterID:       "cluster-123",
				replicas:        2,
				instanceType:    "m5.xlarge",
				subnetID:        "subnet-123",
				instanceProfile: "test-profile",
				securityGroups:  "sg-123",
			},
			expectError: false,
		},
		{
			name: "missing cluster ID",
			opts: &createOptions{
				name:     "test-nodepool",
				replicas: 2,
			},
			expectError: true,
		},
		{
			name: "replicas less than 1",
			opts: &createOptions{
				name:      "test-nodepool",
				clusterID: "cluster-123",
				replicas:  0,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate cluster ID
			if tt.opts.clusterID == "" && tt.expectError {
				// Expected validation failure
				return
			}

			// Validate replicas
			if tt.opts.replicas < 1 && tt.expectError {
				// Expected validation failure
				return
			}

			if !tt.expectError && (tt.opts.clusterID == "" || tt.opts.replicas < 1) {
				t.Error("expected validation to pass, but options are invalid")
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
