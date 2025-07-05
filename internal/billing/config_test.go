package billing

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Save current env
	oldAPIKey := os.Getenv("POLAR_API_KEY")
	oldWebhookSecret := os.Getenv("POLAR_WEBHOOK_SECRET")
	oldProjectID := os.Getenv("POLAR_PROJECT_ID")
	defer func() {
		os.Setenv("POLAR_API_KEY", oldAPIKey)
		os.Setenv("POLAR_WEBHOOK_SECRET", oldWebhookSecret)
		os.Setenv("POLAR_PROJECT_ID", oldProjectID)
	}()

	tests := []struct {
		name      string
		apiKey    string
		webhook   string
		projectID string
		wantErr   bool
	}{
		{
			name:      "Valid config",
			apiKey:    "polar_sk_test",
			webhook:   "whsec_test",
			projectID: "proj_test",
			wantErr:   false,
		},
		{
			name:      "Missing API key",
			apiKey:    "",
			webhook:   "whsec_test",
			projectID: "proj_test",
			wantErr:   true,
		},
		{
			name:      "Missing webhook secret",
			apiKey:    "polar_sk_test",
			webhook:   "",
			projectID: "proj_test",
			wantErr:   true,
		},
		{
			name:      "Missing project ID",
			apiKey:    "polar_sk_test",
			webhook:   "whsec_test",
			projectID: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("POLAR_API_KEY", tt.apiKey)
			os.Setenv("POLAR_WEBHOOK_SECRET", tt.webhook)
			os.Setenv("POLAR_PROJECT_ID", tt.projectID)

			config, err := LoadConfig()
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if config.PolarAPIKey != tt.apiKey {
					t.Errorf("API key mismatch: got %s, want %s", config.PolarAPIKey, tt.apiKey)
				}
				if config.PolarWebhookSecret != tt.webhook {
					t.Errorf("Webhook secret mismatch: got %s, want %s", config.PolarWebhookSecret, tt.webhook)
				}
				if config.PolarProjectID != tt.projectID {
					t.Errorf("Project ID mismatch: got %s, want %s", config.PolarProjectID, tt.projectID)
				}
			}
		})
	}
}

func TestLoadConfigOptional(t *testing.T) {
	// Clear env
	os.Unsetenv("POLAR_API_KEY")
	os.Unsetenv("POLAR_WEBHOOK_SECRET")
	os.Unsetenv("POLAR_PROJECT_ID")

	config := LoadConfigOptional()
	if config == nil {
		t.Fatal("LoadConfigOptional should not return nil")
	}

	if config.IsConfigured() {
		t.Error("Config should not be configured with empty env vars")
	}

	// Set env vars
	os.Setenv("POLAR_API_KEY", "test")
	os.Setenv("POLAR_WEBHOOK_SECRET", "test")
	os.Setenv("POLAR_PROJECT_ID", "test")

	config = LoadConfigOptional()
	if !config.IsConfigured() {
		t.Error("Config should be configured with all env vars set")
	}
}
