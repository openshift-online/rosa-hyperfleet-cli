package cluster

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

func signedGet(ctx context.Context, url string, creds awssdk.Credentials, region string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	payloadHash := sha256.Sum256([]byte(""))
	payloadHashStr := hex.EncodeToString(payloadHash[:])

	signer := v4.NewSigner()
	if err := signer.SignHTTP(ctx, creds, req, payloadHashStr, "execute-api", region, time.Now()); err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func signedDelete(ctx context.Context, url string, creds awssdk.Credentials, region string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	payloadHash := sha256.Sum256([]byte(""))
	payloadHashStr := hex.EncodeToString(payloadHash[:])

	signer := v4.NewSigner()
	if err := signer.SignHTTP(ctx, creds, req, payloadHashStr, "execute-api", region, time.Now()); err != nil {
		return nil, 0, fmt.Errorf("failed to sign request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	return body, resp.StatusCode, nil
}

func fetchAPIURL(ctx context.Context, baseURL, clusterID string, creds awssdk.Credentials, region string) (string, error) {
	endpoint := fmt.Sprintf("%s/api/v0/clusters/%s/statuses", baseURL, clusterID)
	body, err := signedGet(ctx, endpoint, creds, region)
	if err != nil {
		return "", fmt.Errorf("failed to fetch cluster statuses: %w", err)
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

func fetchClusterByName(ctx context.Context, baseURL, name string, creds awssdk.Credentials, region string) (*clusterItem, error) {
	const pageSize = 100
	for offset := 0; ; offset += pageSize {
		endpoint := fmt.Sprintf("%s/api/v0/clusters?limit=%d&offset=%d", baseURL, pageSize, offset)
		body, err := signedGet(ctx, endpoint, creds, region)
		if err != nil {
			return nil, fmt.Errorf("failed to list clusters: %w", err)
		}

		var resp listResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("failed to parse cluster list: %w", err)
		}

		for _, c := range resp.Items {
			if c.Name == name || c.ID == name {
				return &c, nil
			}
		}

		if len(resp.Items) < pageSize {
			break
		}
	}
	return nil, fmt.Errorf("cluster %q not found", name)
}
