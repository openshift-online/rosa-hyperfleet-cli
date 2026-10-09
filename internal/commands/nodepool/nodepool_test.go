package nodepool

import (
	"io"
	"testing"

	v1alpha1 "github.com/openshift-online/rosa-hyperfleet-api/api/v1alpha1/public"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNodePoolCreate_ClusterNameRequired(t *testing.T) {
	cmd := newCreateCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"my-np"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() expected error for missing --cluster-name, got nil")
	}
	want := "--cluster-name is required"
	if err.Error() != want {
		t.Errorf("Execute() error = %q, want %q", err.Error(), want)
	}
}

func TestNodePoolCreate_NameMustStartWithClusterName(t *testing.T) {
	tests := []struct {
		name    string
		cluster string
		want    string
	}{
		{
			name:    "other-cluster.my-np",
			cluster: "my-cluster",
			want:    `nodepool "other-cluster.my-np" does not belong to cluster "my-cluster"`,
		},
		{
			name:    "my-clustered.my-np",
			cluster: "my-cluster",
			want:    `nodepool "my-clustered.my-np" does not belong to cluster "my-cluster"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newCreateCommand()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{tt.name, "--cluster-name", tt.cluster})

			err := cmd.Execute()
			if err == nil {
				t.Fatal("Execute() expected error for node pool name that does not belong to cluster, got nil")
			}
			if err.Error() != tt.want {
				t.Errorf("Execute() error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestNodePoolCreate_ReplicasMustBePositive(t *testing.T) {
	cmd := newCreateCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"my-cluster.my-np", "--cluster-name", "my-cluster", "--replicas", "0"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() expected error for --replicas 0, got nil")
	}
	want := "--replicas must be at least 1"
	if err.Error() != want {
		t.Errorf("Execute() error = %q, want %q", err.Error(), want)
	}
}

func TestNodePoolCreate_RequiresName(t *testing.T) {
	cmd := newCreateCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--cluster-name", "my-cluster"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() expected error for missing node pool name, got nil")
	}
}

func TestNodePoolList_ClusterNameRequired(t *testing.T) {
	cmd := newListCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() expected error for missing --cluster-name, got nil")
	}
	want := "--cluster-name is required"
	if err.Error() != want {
		t.Errorf("Execute() error = %q, want %q", err.Error(), want)
	}
}

func TestNodePoolList_LimitBounds(t *testing.T) {
	tests := []struct {
		name  string
		limit string
		want  string
	}{
		{"zero", "0", "--limit must be between 1 and 100"},
		{"over max", "101", "--limit must be between 1 and 100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newListCommand()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{"--cluster-name", "my-cluster", "--limit", tt.limit})

			err := cmd.Execute()
			if err == nil {
				t.Fatal("Execute() expected error, got nil")
			}
			if err.Error() != tt.want {
				t.Errorf("Execute() error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestNodePoolDelete_RequiresNodePoolName(t *testing.T) {
	cmd := newDeleteCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() expected error for missing nodepool name, got nil")
	}
}

func TestNodePoolDelete_ClusterNameRequired(t *testing.T) {
	cmd := newDeleteCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"my-cluster.my-np"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() expected error for missing --cluster-name, got nil")
	}
	if want := "--cluster-name is required"; err.Error() != want {
		t.Errorf("Execute() error = %q, want %q", err.Error(), want)
	}
}

func TestNodePoolListFiltersByClusterNameBeforeApplyingPagination(t *testing.T) {
	nodepools := []v1alpha1.NodePool{
		{ObjectMeta: metav1.ObjectMeta{Name: "alpha.one"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "beta.one"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "alpha.two"}},
	}

	filtered := filterNodePoolsByClusterName(nodepools, "alpha")
	page := paginateNodePools(filtered, 1, 1)
	if len(page) != 1 || page[0].Name != "alpha.two" {
		t.Fatalf("page = %#v, want [alpha.two]", page)
	}
}
