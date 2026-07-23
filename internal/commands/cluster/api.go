package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/openshift-online/rosa-regional-platform-cli/internal/client"
)

func fetchAPIURL(ctx context.Context, c *client.Client, clusterID string) (string, error) {
	path := fmt.Sprintf("/api/v0/clusters/%s/statuses", url.PathEscape(clusterID))
	body, statusCode, err := c.Get(ctx, path)
	if err != nil {
		return "", fmt.Errorf("failed to fetch cluster statuses: %w", err)
	}
	if statusCode != 200 {
		return "", fmt.Errorf("failed to fetch cluster statuses: status %d: %s", statusCode, string(body))
	}

	var envelope struct {
		Status *struct {
			ControlPlaneEndpoint *struct {
				Host string `json:"host"`
				Port int32  `json:"port"`
			} `json:"controlPlaneEndpoint"`
		} `json:"status"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", fmt.Errorf("failed to parse cluster statuses: %w", err)
	}

	if envelope.Status != nil && envelope.Status.ControlPlaneEndpoint != nil {
		ep := envelope.Status.ControlPlaneEndpoint
		if ep.Host != "" {
			return fmt.Sprintf("https://%s:%d", ep.Host, ep.Port), nil
		}
	}
	return "", nil
}

func fetchClusterByName(ctx context.Context, c *client.Client, name string) (*clusterItem, error) {
	const pageSize = 100
	for offset := 0; ; offset += pageSize {
		path := fmt.Sprintf("/api/v0/clusters?limit=%d&offset=%d", pageSize, offset)
		body, statusCode, err := c.Get(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("failed to list clusters: %w", err)
		}
		if statusCode != 200 {
			return nil, fmt.Errorf("failed to list clusters: status %d: %s", statusCode, string(body))
		}

		var resp listResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("failed to parse cluster list: %w", err)
		}

		for _, item := range resp.Items {
			if item.Name == name || item.ID == name {
				return &item, nil
			}
		}

		if len(resp.Items) < pageSize {
			break
		}
	}
	return nil, fmt.Errorf("cluster %q not found", name)
}
