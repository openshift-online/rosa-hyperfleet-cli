package login

import (
	"fmt"
	"net/url"

	"github.com/openshift-online/rosa-regional-platform-cli/internal/config"
	"github.com/spf13/cobra"
)

type loginOptions struct {
	url string
}

// NewLoginCommand creates the login command
func NewLoginCommand() *cobra.Command {
	opts := &loginOptions{}

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to the platform API",
		Long: `Configure the CLI to connect to a platform API.

This command stores the platform API base URL for future API calls.

Example:
  rosactl login --url https://api.platform.example.com
  rosactl login --url https://api.platform.example.com/prod`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(opts)
		},
	}

	cmd.Flags().StringVar(&opts.url, "url", "", "Platform API base URL (required)")
	_ = cmd.MarkFlagRequired("url")

	return cmd
}

func runLogin(opts *loginOptions) error {
	// Validate URL
	parsedURL, err := url.Parse(opts.url)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Enforce https scheme only — plain http transmits SigV4 credentials unencrypted
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("URL scheme must be https")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("URL must include a host")
	}

	// Reject URLs with query or fragment components
	if parsedURL.RawQuery != "" {
		return fmt.Errorf("URL must not include query parameters")
	}

	if parsedURL.Fragment != "" {
		return fmt.Errorf("URL must not include a fragment")
	}

	// Build normalized baseURL from components (including path if present)
	baseURL := parsedURL.Scheme + "://" + parsedURL.Host + parsedURL.Path

	// Save the URL to config
	if err := config.SetPlatformAPIURL(baseURL); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Printf("✓ Logged in successfully\n")
	fmt.Printf("  Platform API URL: %s\n", baseURL)

	// Show where the config is stored
	home, _ := config.GetConfigPath()
	if home != "" {
		fmt.Printf("  Config saved to: %s\n", home)
	}

	return nil
}
