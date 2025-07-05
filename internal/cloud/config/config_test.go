package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCloudConfig(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create .orz directory
	orzDir := filepath.Join(tmpDir, ".orz")
	if err := os.MkdirAll(orzDir, 0755); err != nil {
		t.Fatalf("Failed to create .orz directory: %v", err)
	}

	// Test case 1: No config file
	config, err := LoadCloudConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load empty config: %v", err)
	}
	if config.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", config.Version)
	}

	// Test case 2: Valid config file
	configContent := `version: "1.0"
setup:
  init: |
    echo "Initializing environment..."
    touch /tmp/marker_init_done
  onAttach: |
    echo "Welcome! Environment ready."
    
services:
  postgres:
    image: postgres:15
    env:
      POSTGRES_PASSWORD: secret
      POSTGRES_DB: myapp
    ports: [5432]
    health:
      command: ["pg_isready", "-U", "postgres"]
      interval: "10s"
      timeout: "5s"
      retries: 5
      
  redis:
    image: redis:7
    ports: [6379]
    health:
      command: ["redis-cli", "ping"]
      
env:
  APP_ENV: development
  DEBUG: "true"
  
resources:
  cpu: "4"
  memory: "8Gi"
`

	// Write config file
	configPath := filepath.Join(orzDir, "cloud.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load and validate
	config, err = LoadCloudConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify setup scripts
	if config.Setup.Init == "" {
		t.Error("Expected init script to be set")
	}
	if config.Setup.OnAttach == "" {
		t.Error("Expected onAttach script to be set")
	}

	// Verify services
	if len(config.Services) != 2 {
		t.Errorf("Expected 2 services, got %d", len(config.Services))
	}

	postgres, ok := config.Services["postgres"]
	if !ok {
		t.Error("Expected postgres service to exist")
	} else {
		if postgres.Image != "postgres:15" {
			t.Errorf("Expected postgres:15 image, got %s", postgres.Image)
		}
		if postgres.Env["POSTGRES_PASSWORD"] != "secret" {
			t.Error("Expected POSTGRES_PASSWORD to be set")
		}
		if len(postgres.Ports) != 1 || postgres.Ports[0] != 5432 {
			t.Error("Expected postgres port 5432")
		}
	}

	// Verify environment variables
	if config.Env["APP_ENV"] != "development" {
		t.Errorf("Expected APP_ENV=development, got %s", config.Env["APP_ENV"])
	}

	// Verify resources
	if config.Resources.CPU != "4" {
		t.Errorf("Expected CPU=4, got %s", config.Resources.CPU)
	}
	if config.Resources.Memory != "8Gi" {
		t.Errorf("Expected Memory=8Gi, got %s", config.Resources.Memory)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  CloudConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: CloudConfig{
				Version: "1.0",
				Services: map[string]ServiceConfig{
					"postgres": {
						Image: "postgres:15",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid version",
			config: CloudConfig{
				Version: "2.0",
			},
			wantErr: true,
		},
		{
			name: "missing service image",
			config: CloudConfig{
				Version: "1.0",
				Services: map[string]ServiceConfig{
					"postgres": {
						Image: "",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
