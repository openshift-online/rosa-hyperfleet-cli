package platform

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

func TestNewClientset_RequiresPlatformURL(t *testing.T) {
	t.Setenv("ROSA_PLATFORM_API_URL", "")

	ctx := context.Background()
	cfg := aws.Config{
		Region: "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider(
			"test-access-key",
			"test-secret-key",
			"",
		),
	}

	_, err := NewClientset(ctx, cfg)
	if err == nil {
		t.Fatal("NewClientset() expected error when platform URL not set, got nil")
	}
	// The error could be about missing platform URL or about AWS credentials
	// depending on whether the env var check happens first
	expectedErrors := []string{
		"failed to get platform API URL: ROSA_PLATFORM_API_URL environment variable is not set",
		"failed to get AWS account ID",
	}

	matched := false
	for _, expected := range expectedErrors {
		if stringContains(err.Error(), expected) {
			matched = true
			break
		}
	}

	if !matched {
		t.Errorf("unexpected error message: %v", err)
	}
}

func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || indexOfString(s, substr) >= 0)
}

func indexOfString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func TestNewClientset_RequiresValidURL(t *testing.T) {
	t.Setenv("ROSA_PLATFORM_API_URL", "https://example.execute-api.us-east-1.amazonaws.com")

	ctx := context.Background()
	cfg := aws.Config{
		Region: "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider(
			"test-access-key",
			"test-secret-key",
			"",
		),
	}

	// This will fail at STS GetCallerIdentity since we're using fake credentials
	_, err := NewClientset(ctx, cfg)
	if err == nil {
		t.Fatal("NewClientset() expected error with fake credentials, got nil")
	}
	// Should fail at getting AWS account ID
	if !stringContains(err.Error(), "failed to get AWS account ID") {
		t.Errorf("expected error to contain 'failed to get AWS account ID', got: %v", err)
	}
}
