package crypto

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// GetOIDCThumbprint fetches the TLS certificate from the OIDC issuer URL
// and calculates the SHA-1 fingerprint of the root certificate.
// This is required by AWS IAM when creating an OIDC provider.
//
// This function mimics Terraform's data.tls_certificate behavior:
// https://registry.terraform.io/providers/hashicorp/tls/latest/docs/data-sources/certificate
//
// HTTPS_PROXY / https_proxy is honoured automatically via http.ProxyFromEnvironment.
func GetOIDCThumbprint(ctx context.Context, issuerURL string) (string, error) {
	parsedURL, err := url.Parse(issuerURL)
	if err != nil {
		return "", fmt.Errorf("invalid OIDC issuer URL: %w", err)
	}
	if parsedURL.Scheme != "https" {
		return "", fmt.Errorf("OIDC issuer URL must use HTTPS, got: %s", parsedURL.Scheme)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyFromEnvironment
	client := &http.Client{Transport: transport}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuerURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to connect to OIDC issuer: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 {
		return "", fmt.Errorf("no TLS certificates found in connection to OIDC issuer")
	}

	// Get the root certificate (last in the chain).
	// AWS IAM requires the thumbprint of the root CA certificate.
	rootCert := resp.TLS.PeerCertificates[len(resp.TLS.PeerCertificates)-1]

	// SHA-1 fingerprint of the DER-encoded certificate — AWS IAM requirement.
	hash := sha1.Sum(rootCert.Raw)
	return hex.EncodeToString(hash[:]), nil
}

// GetOIDCIssuerDomain extracts the domain from an OIDC issuer URL
// by removing the https:// prefix. This is needed for the OIDCIssuerDomain
// CloudFormation parameter.
func GetOIDCIssuerDomain(issuerURL string) (string, error) {
	if !strings.HasPrefix(issuerURL, "https://") {
		return "", fmt.Errorf("OIDC issuer URL must start with https://, got: %s", issuerURL)
	}
	return strings.TrimPrefix(issuerURL, "https://"), nil
}
