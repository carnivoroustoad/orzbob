package main

import (
	"fmt"
	"os"
	"path/filepath"

	"orzbob/internal/cloud/config"
)

func main() {
	// Get config file path
	configPath := ".orz/cloud.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("❌ Config file not found: %s\n", configPath)
		fmt.Println("\nUsage: go run hack/validate-cloud-config.go [path/to/cloud.yaml]")
		os.Exit(1)
	}

	fmt.Printf("🔍 Validating %s...\n\n", configPath)

	// Load config
	workDir := filepath.Dir(filepath.Dir(configPath))
	cfg, err := config.LoadCloudConfig(workDir)
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		fmt.Printf("❌ Validation failed: %v\n", err)
		os.Exit(1)
	}

	// Print summary
	fmt.Println("✅ Configuration is valid!")
	fmt.Println("\n📋 Summary:")
	fmt.Printf("  Version: %s\n", cfg.Version)
	
	if cfg.Setup.Init != "" {
		fmt.Printf("  ✓ Init script: %d lines\n", countLines(cfg.Setup.Init))
	}
	if cfg.Setup.OnAttach != "" {
		fmt.Printf("  ✓ OnAttach script: %d lines\n", countLines(cfg.Setup.OnAttach))
	}
	
	if len(cfg.Services) > 0 {
		fmt.Printf("  ✓ Services: %d configured\n", len(cfg.Services))
		for name, svc := range cfg.Services {
			fmt.Printf("    - %s (%s)\n", name, svc.Image)
		}
	}
	
	if len(cfg.Env) > 0 {
		fmt.Printf("  ✓ Environment variables: %d defined\n", len(cfg.Env))
	}
	
	if cfg.Resources.CPU != "" || cfg.Resources.Memory != "" {
		fmt.Printf("  ✓ Resources: CPU=%s, Memory=%s\n", 
			cfg.Resources.CPU, cfg.Resources.Memory)
	}

	fmt.Println("\n🎉 Your cloud.yaml is ready to use!")
	fmt.Println("   Run: orz cloud new \"Your task here\"")
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	count := 1
	for _, c := range s {
		if c == '\n' {
			count++
		}
	}
	return count
}