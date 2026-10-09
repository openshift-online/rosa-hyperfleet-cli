package cluster

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	v1alpha1 "github.com/openshift-online/rosa-hyperfleet-api/api/v1alpha1/public"
	hyperfleet "github.com/openshift-online/rosa-hyperfleet-api/clientset"
	"github.com/openshift-online/rosa-hyperfleet-api/clientset/platform"
	hfrest "github.com/openshift-online/rosa-hyperfleet-api/clientset/rest"
	pkgconfig "github.com/openshift-online/rosa-regional-platform-cli/internal/config"
)

func fetchAPIURL(ctx context.Context, baseURL, clusterName string, creds awssdk.Credentials, region string) (string, error) {
	// Load AWS config for clientset
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w", err)
	}

	accountID, err := pkgconfig.GetAccountID()
	if err != nil {
		return "", fmt.Errorf("failed to get account ID: %w", err)
	}

	// Create clientset
	cs, err := hyperfleet.NewForConfig(&hfrest.Config{
		Host:      baseURL,
		AccountID: accountID,
		AWSConfig: awsCfg,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create clientset: %w", err)
	}

	// Get cluster to access status
	cluster, err := cs.HyperfleetV1alpha1().Clusters().Get(ctx, clusterName, platform.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to fetch cluster %q: %w", clusterName, err)
	}

	if cluster.Status.ControlPlaneEndpoint.Host != "" {
		return fmt.Sprintf("https://%s:%d", cluster.Status.ControlPlaneEndpoint.Host, cluster.Status.ControlPlaneEndpoint.Port), nil
	}

	return "", nil
}

func fetchClusterByName(ctx context.Context, baseURL, clusterName string, creds awssdk.Credentials, region string) (*v1alpha1.Cluster, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	accountID, err := pkgconfig.GetAccountID()
	if err != nil {
		return nil, fmt.Errorf("failed to get account ID: %w", err)
	}

	cs, err := hyperfleet.NewForConfig(&hfrest.Config{
		Host:      baseURL,
		AccountID: accountID,
		AWSConfig: awsCfg,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	cluster, err := cs.HyperfleetV1alpha1().Clusters().Get(ctx, clusterName, platform.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster %q: %w", clusterName, err)
	}
	return cluster, nil
}
