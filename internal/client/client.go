// Package client provides a SigV4-signed HTTP client for the Platform API.
// All commands that talk to the Platform API should use this package rather
// than inlining signing logic directly.
package client

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/config"
)

const (
	defaultRegion  = "us-east-1"
	defaultTimeout = 30 * time.Second
)

// Client is a SigV4-signed HTTP client for the Platform API. Construct one
// with New and reuse it across calls — it holds a single *http.Client and
// resolved AWS credentials.
type Client struct {
	baseURL    string
	creds      awssdk.Credentials
	region     string
	httpClient *http.Client
}

// New resolves the Platform API URL and AWS credentials once and returns a
// ready-to-use Client. All commands should call New at the start of their
// RunE handler and pass the Client down to any helpers that need it.
func New(ctx context.Context) (*Client, error) {
	baseURL, err := config.GetPlatformAPIURL()
	if err != nil {
		return nil, err
	}

	cfg, err := aws.NewConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve AWS credentials: %w", err)
	}

	region := cfg.Region
	if region == "" {
		region = defaultRegion
	}

	return &Client{
		baseURL: baseURL,
		creds:   creds,
		region:  region,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}, nil
}

// BaseURL returns the Platform API base URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// Region returns the AWS region the client is configured for.
func (c *Client) Region() string {
	return c.region
}

// Get performs a SigV4-signed GET request and returns the response body and
// status code. The caller is responsible for checking the status code.
func (c *Client) Get(ctx context.Context, path string) ([]byte, int, error) {
	return c.do(ctx, http.MethodGet, path, nil)
}

// Post performs a SigV4-signed POST request with a JSON body and returns the
// response body and status code.
func (c *Client) Post(ctx context.Context, path string, body []byte) ([]byte, int, error) {
	return c.do(ctx, http.MethodPost, path, body)
}

// Delete performs a SigV4-signed DELETE request and returns the response body
// and status code.
func (c *Client) Delete(ctx context.Context, path string) ([]byte, int, error) {
	return c.do(ctx, http.MethodDelete, path, nil)
}

// do is the single implementation of a signed HTTP request. All exported
// methods delegate here.
func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, int, error) {
	url := c.baseURL + path

	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	hash := sha256.Sum256(body)
	payloadHash := hex.EncodeToString(hash[:])

	signer := v4.NewSigner()
	if err := signer.SignHTTP(ctx, c.creds, req, payloadHash, "execute-api", c.region, time.Now()); err != nil {
		return nil, 0, fmt.Errorf("failed to sign request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}
