package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	configDir  = ".rosactl"
	configFile = "config.json"
)

// Config represents the CLI configuration
type Config struct {
	PlatformAPIURL string `json:"platform_api_url,omitempty"`
}

// getConfigPath returns the path to the config file
func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	configDirPath := filepath.Join(home, configDir)
	return filepath.Join(configDirPath, configFile), nil
}

// GetConfigPath returns the path to the config file (public version for display)
func GetConfigPath() (string, error) {
	return getConfigPath()
}

// ensureConfigDir ensures the config directory exists
func ensureConfigDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDirPath := filepath.Join(home, configDir)
	if err := os.MkdirAll(configDirPath, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return nil
}

// Load reads the configuration from disk
func Load() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	// If config file doesn't exist, return empty config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &Config{}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// Save writes the configuration to disk
func Save(cfg *Config) error {
	if err := ensureConfigDir(); err != nil {
		return err
	}

	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// SetPlatformAPIURL sets the platform API URL in the config
func SetPlatformAPIURL(url string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}

	cfg.PlatformAPIURL = url
	return Save(cfg)
}

// GetPlatformAPIURL gets the platform API URL from the config
func GetPlatformAPIURL() (string, error) {
	cfg, err := Load()
	if err != nil {
		return "", err
	}

	if cfg.PlatformAPIURL == "" {
		return "", fmt.Errorf("platform API URL not configured. Run 'rosactl login --url <URL>' first")
	}

	return cfg.PlatformAPIURL, nil
}

// MustGetPlatformAPIURL gets the platform API URL or panics if not configured
// Use this when the platform API URL is required for the command to work
func MustGetPlatformAPIURL() string {
	url, err := GetPlatformAPIURL()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	return url
}
