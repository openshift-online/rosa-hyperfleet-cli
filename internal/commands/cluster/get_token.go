package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	iamaws "github.com/openshift-online/rosa-regional-platform-cli/internal/aws"
	"github.com/spf13/cobra"
)

const (
	// audiencePrefix scopes a token to a single cluster. The hosted KAS only
	// accepts tokens whose aud is "rosa:cluster:<cluster-id>".
	audiencePrefix = "rosa:cluster:"
	// tokenDuration must stay within the KAS claim validation rule (exp - iat <= 900).
	tokenDuration = 900 * time.Second
	// tokenRefreshSkew makes kubectl request a new token before the KAS rejects it.
	tokenRefreshSkew = time.Minute
)

// ClusterAudience returns the token audience for a cluster.
func ClusterAudience(clusterID string) string {
	return audiencePrefix + clusterID
}

func newGetTokenCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-token --cluster-id <cluster-id>",
		Short: "Generate an IAM authentication token for a cluster",
		Long: `Request a short-lived JWT from AWS STS (sts:GetWebIdentityToken) for
authenticating to a hosted cluster. This command is used as a kubectl exec
credential plugin.

Requires IAM outbound identity federation to be enabled in the AWS account
(aws iam enable-outbound-web-identity-federation) and the sts:GetWebIdentityToken
permission.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterID, _ := cmd.Flags().GetString("cluster-id")
			if clusterID == "" {
				return fmt.Errorf("--cluster-id is required")
			}
			return runGetToken(cmd.Context(), clusterID)
		},
	}

	cmd.Flags().String("cluster-id", "", "Cluster ID to generate a token for")
	_ = cmd.MarkFlagRequired("cluster-id")

	return cmd
}

func runGetToken(ctx context.Context, clusterID string) error {
	cfg, err := iamaws.NewConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}
	// GetWebIdentityToken is not served by the STS global endpoint.
	if cfg.Region == "" {
		return fmt.Errorf("an AWS region is required: GetWebIdentityToken is only available on regional STS endpoints")
	}

	out, err := sts.NewFromConfig(cfg).GetWebIdentityToken(ctx, &sts.GetWebIdentityTokenInput{
		Audience:         []string{ClusterAudience(clusterID)},
		SigningAlgorithm: aws.String("ES384"),
		DurationSeconds:  aws.Int32(int32(tokenDuration.Seconds())),
	})
	if err != nil {
		return fmt.Errorf("failed to get web identity token (is outbound identity federation enabled and sts:GetWebIdentityToken allowed?): %w", err)
	}

	expiration := time.Now().Add(tokenDuration)
	if out.Expiration != nil {
		expiration = *out.Expiration
	}

	credential, _ := json.Marshal(execCredential(aws.ToString(out.WebIdentityToken), expiration.Add(-tokenRefreshSkew)))
	if _, err := fmt.Fprint(os.Stdout, string(credential)); err != nil {
		return fmt.Errorf("writing token to stdout: %w", err)
	}
	return nil
}

func execCredential(token string, expiration time.Time) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "client.authentication.k8s.io/v1",
		"kind":       "ExecCredential",
		"status": map[string]interface{}{
			"expirationTimestamp": expiration.UTC().Format(time.RFC3339),
			"token":               token,
		},
	}
}
