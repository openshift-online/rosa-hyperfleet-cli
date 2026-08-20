package cluster

import (
	"context"
	"fmt"

	v1alpha1 "github.com/openshift-online/rosa-hyperfleet-api/api/v1alpha1/public"
	hfclient "github.com/openshift-online/rosa-hyperfleet-api/clientset"
	"github.com/openshift-online/rosa-hyperfleet-api/clientset/platform"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

// getClusterByNameOrID fetches a cluster by ID or name.
// It first tries to get by ID (fast path), then falls back to searching by name.
func getClusterByNameOrID(ctx context.Context, cs *hfclient.Clientset, nameOrID string) (*v1alpha1.Cluster, error) {
	// Try to get by ID first (fast path)
	cluster, err := cs.HyperfleetV1alpha1().Clusters().Get(ctx, nameOrID, platform.GetOptions{})
	if err == nil {
		return cluster, nil
	}

	// If not found, search by name
	if k8serrors.IsNotFound(err) {
		const pageSize = 100
		for offset := 0; ; offset += pageSize {
			clusterList, err := cs.HyperfleetV1alpha1().Clusters().List(ctx, platform.ListOptions{
				Limit:  pageSize,
				Offset: int64(offset),
			})
			if err != nil {
				return nil, fmt.Errorf("failed to list clusters: %w", err)
			}

			// Search for matching name or ID
			for _, c := range clusterList.Items {
				if c.Name == nameOrID || string(c.UID) == nameOrID {
					return &c, nil
				}
			}

			// If we got fewer items than requested, we've reached the end
			if len(clusterList.Items) < pageSize {
				break
			}
		}

		return nil, fmt.Errorf("cluster %q not found", nameOrID)
	}

	// Other error
	return nil, fmt.Errorf("failed to fetch cluster: %w", err)
}
