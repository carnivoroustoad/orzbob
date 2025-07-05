package billing

import (
	"fmt"
	"os"
)

// Config holds billing configuration
type Config struct {
	PolarAPIKey        string
	PolarWebhookSecret string
	PolarProjectID     string
	PolarOrgID         string
}

// LoadConfig loads billing configuration from environment variables
func LoadConfig() (*Config, error) {
	config := &Config{
		PolarAPIKey:        os.Getenv("POLAR_ACCESS_TOKEN"), // Also check old env var name
		PolarWebhookSecret: os.Getenv("POLAR_WEBHOOK_SECRET"),
		PolarProjectID:     os.Getenv("POLAR_PROJECT_ID"),
		PolarOrgID:         os.Getenv("POLAR_ORGANIZATION_ID"),
	}

	// Support both POLAR_ACCESS_TOKEN and POLAR_API_KEY
	if config.PolarAPIKey == "" {
		config.PolarAPIKey = os.Getenv("POLAR_API_KEY")
	}

	if config.PolarAPIKey == "" {
		return nil, fmt.Errorf("POLAR_ACCESS_TOKEN or POLAR_API_KEY environment variable is required")
	}
	if config.PolarWebhookSecret == "" {
		return nil, fmt.Errorf("POLAR_WEBHOOK_SECRET environment variable is required")
	}
	if config.PolarProjectID == "" {
		return nil, fmt.Errorf("POLAR_PROJECT_ID environment variable is required")
	}

	return config, nil
}

// LoadConfigOptional loads billing configuration but doesn't fail if missing
func LoadConfigOptional() *Config {
	config := &Config{
		PolarAPIKey:        os.Getenv("POLAR_ACCESS_TOKEN"),
		PolarWebhookSecret: os.Getenv("POLAR_WEBHOOK_SECRET"),
		PolarProjectID:     os.Getenv("POLAR_PROJECT_ID"),
		PolarOrgID:         os.Getenv("POLAR_ORGANIZATION_ID"),
	}

	// Support both POLAR_ACCESS_TOKEN and POLAR_API_KEY
	if config.PolarAPIKey == "" {
		config.PolarAPIKey = os.Getenv("POLAR_API_KEY")
	}

	return config
}

// IsConfigured returns true if billing is properly configured
func (c *Config) IsConfigured() bool {
	return c.PolarAPIKey != "" && c.PolarWebhookSecret != "" && (c.PolarProjectID != "" || c.PolarOrgID != "")
}
