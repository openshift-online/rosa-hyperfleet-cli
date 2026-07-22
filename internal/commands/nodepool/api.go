package nodepool

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
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

func signedRequest(ctx context.Context, method, url string, body []byte, creds awssdk.Credentials, region string) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	payloadHash := sha256.Sum256(body)
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

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

func signedGet(ctx context.Context, url string, creds awssdk.Credentials, region string) ([]byte, int, error) {
	return signedRequest(ctx, http.MethodGet, url, nil, creds, region)
}

func signedPost(ctx context.Context, url string, body []byte, creds awssdk.Credentials, region string) ([]byte, int, error) {
	return signedRequest(ctx, http.MethodPost, url, body, creds, region)
}

func signedDelete(ctx context.Context, url string, creds awssdk.Credentials, region string) ([]byte, int, error) {
	return signedRequest(ctx, http.MethodDelete, url, nil, creds, region)
}
