package config_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"orzbob/internal/cloud/config"
)

func TestInitScriptExecution(t *testing.T) {
	// Create temp workspace
	tmpDir := t.TempDir()

	// Create .orz directory
	orzDir := filepath.Join(tmpDir, ".orz")
	if err := os.MkdirAll(orzDir, 0755); err != nil {
		t.Fatalf("Failed to create .orz directory: %v", err)
	}

	// Create cloud.yaml
	cloudConfig := `version: "1.0"
setup:
  init: |
    echo "Running init script..."
    touch /tmp/test_marker_init_done_$$
    echo "Init completed" > init_result.txt
env:
  TEST_VAR: "Hello from test"
`
	configPath := filepath.Join(orzDir, "cloud.yaml")
	if err := os.WriteFile(configPath, []byte(cloudConfig), 0644); err != nil {
		t.Fatalf("Failed to write cloud.yaml: %v", err)
	}

	// Load config
	cfg, err := config.LoadCloudConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	// Verify init script loaded
	if cfg.Setup.Init == "" {
		t.Fatal("Init script not loaded")
	}

	// Execute init script
	scriptPath := filepath.Join(tmpDir, "init.sh")
	if err := os.WriteFile(scriptPath, []byte(cfg.Setup.Init), 0755); err != nil {
		t.Fatalf("Failed to write init script: %v", err)
	}

	cmd := exec.Command("/bin/bash", scriptPath)
	cmd.Dir = tmpDir
	cmd.Env = os.Environ()

	// Add config env vars
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Init script failed: %v\nOutput: %s", err, output)
	}

	t.Logf("Init script output: %s", output)

	// Verify result file created
	resultPath := filepath.Join(tmpDir, "init_result.txt")
	if content, err := os.ReadFile(resultPath); err != nil {
		t.Errorf("Failed to read result file: %v", err)
	} else {
		t.Logf("Result file content: %s", content)
	}
}
