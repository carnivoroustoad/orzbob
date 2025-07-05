package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// CloudConfig represents the .orz/cloud.yaml configuration
type CloudConfig struct {
	// Version of the config schema
	Version string `yaml:"version"`

	// Setup contains initialization scripts
	Setup SetupConfig `yaml:"setup"`

	// Services defines sidecar containers
	Services map[string]ServiceConfig `yaml:"services"`

	// Environment variables to set
	Env map[string]string `yaml:"env"`

	// Resources for the main container
	Resources ResourceConfig `yaml:"resources"`
}

// SetupConfig contains initialization and attachment scripts
type SetupConfig struct {
	// Init script runs once when the instance is created
	Init string `yaml:"init"`

	// OnAttach script runs each time someone attaches to the instance
	OnAttach string `yaml:"onAttach"`
}

// ServiceConfig defines a sidecar service
type ServiceConfig struct {
	// Image is the Docker image to use
	Image string `yaml:"image"`

	// Environment variables for the service
	Env map[string]string `yaml:"env"`

	// Ports to expose
	Ports []int `yaml:"ports"`

	// Health check configuration
	Health HealthConfig `yaml:"health"`
}

// HealthConfig defines health check settings
type HealthConfig struct {
	// Command to run for health check
	Command []string `yaml:"command"`

	// Interval between health checks
	Interval string `yaml:"interval"`

	// Timeout for health check
	Timeout string `yaml:"timeout"`

	// Number of retries before considering unhealthy
	Retries int `yaml:"retries"`
}

// ResourceConfig defines resource requirements
type ResourceConfig struct {
	// CPU in cores (e.g., "2" or "500m")
	CPU string `yaml:"cpu"`

	// Memory in bytes (e.g., "4Gi" or "512Mi")
	Memory string `yaml:"memory"`

	// GPU count (for GPU instances)
	GPU int `yaml:"gpu"`
}

// LoadCloudConfig loads the cloud configuration from .orz/cloud.yaml
func LoadCloudConfig(workDir string) (*CloudConfig, error) {
	configPath := filepath.Join(workDir, ".orz", "cloud.yaml")

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Return empty config if file doesn't exist
		return &CloudConfig{
			Version:  "1.0",
			Setup:    SetupConfig{},
			Services: make(map[string]ServiceConfig),
			Env:      make(map[string]string),
		}, nil
	}

	// Read the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cloud config: %w", err)
	}

	// Parse YAML
	var config CloudConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse cloud config: %w", err)
	}

	// Set defaults
	if config.Version == "" {
		config.Version = "1.0"
	}
	if config.Services == nil {
		config.Services = make(map[string]ServiceConfig)
	}
	if config.Env == nil {
		config.Env = make(map[string]string)
	}

	return &config, nil
}

// Validate checks if the configuration is valid
func (c *CloudConfig) Validate() error {
	// Check version
	if c.Version != "1.0" {
		return fmt.Errorf("unsupported config version: %s", c.Version)
	}

	// Validate services
	for name, service := range c.Services {
		if service.Image == "" {
			return fmt.Errorf("service %s: image is required", name)
		}

		// Validate health check if present
		if len(service.Health.Command) > 0 {
			if service.Health.Interval == "" {
				service.Health.Interval = "30s"
			}
			if service.Health.Timeout == "" {
				service.Health.Timeout = "5s"
			}
			if service.Health.Retries == 0 {
				service.Health.Retries = 3
			}
		}
	}

	return nil
}
