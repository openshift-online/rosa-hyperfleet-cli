package cluster

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

// ResolveIAMLoginIssuer returns the AWS account's IAM outbound identity
// federation issuer. The cluster trusts tokens from this issuer for API server
// login, and the cluster creator becomes cluster-admin.
func ResolveIAMLoginIssuer(ctx context.Context, cfg aws.Config) (string, error) {
	info, err := iam.NewFromConfig(cfg).GetOutboundWebIdentityFederationInfo(ctx, &iam.GetOutboundWebIdentityFederationInfoInput{})
	if err != nil {
		return "", fmt.Errorf("looking up the AWS account's outbound identity federation issuer (needs iam:GetOutboundWebIdentityFederationInfo): %w", err)
	}
	if !info.JwtVendingEnabled || aws.ToString(info.IssuerIdentifier) == "" {
		return "", fmt.Errorf("AWS IAM login needs outbound identity federation enabled in this AWS account: run 'aws iam enable-outbound-web-identity-federation'")
	}
	return aws.ToString(info.IssuerIdentifier), nil
}
