package platform

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	hfclient "github.com/openshift-online/rosa-hyperfleet-api/clientset"
	hfrest "github.com/openshift-online/rosa-hyperfleet-api/clientset/rest"
	pkgconfig "github.com/openshift-online/rosa-regional-platform-cli/internal/config"
)

// NewClientset creates a Hyperfleet platform API clientset from AWS config.
// It resolves the platform API URL from config and derives the AWS account ID
// from STS GetCallerIdentity.
func NewClientset(ctx context.Context, awsCfg aws.Config) (*hfclient.Clientset, error) {
	baseURL, err := pkgconfig.GetPlatformAPIURL()
	if err != nil {
		return nil, fmt.Errorf("failed to get platform API URL: %w", err)
	}

	accountID, err := getAWSAccountID(ctx, awsCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get AWS account ID: %w", err)
	}

	cfg := &hfrest.Config{
		Host:      baseURL,
		AccountID: accountID,
		AWSConfig: awsCfg,
	}

	cs, err := hfclient.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return cs, nil
}

func getAWSAccountID(ctx context.Context, cfg aws.Config) (string, error) {
	stsClient := sts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}
	return aws.ToString(identity.Account), nil
}
