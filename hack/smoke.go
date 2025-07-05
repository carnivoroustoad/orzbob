//go:build tools
// +build tools

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"orzbob/internal/cloud/provider"
)

func main() {
	// Get kubeconfig path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get home directory: %v", err)
	}
	kubeconfig := filepath.Join(homeDir, ".kube", "config")

	// Create provider
	p, err := provider.NewLocalKind(kubeconfig)
	if err != nil {
		log.Fatalf("Failed to create LocalKind provider: %v", err)
	}

	ctx := context.Background()

	// Create instance
	fmt.Println("Creating instance...")
	instance, err := p.CreateInstance(ctx, "small")
	if err != nil {
		log.Fatalf("Failed to create instance: %v", err)
	}
	fmt.Printf("Created instance: %s\n", instance.ID)

	// Wait for pod to be running
	fmt.Println("Waiting for pod to be running...")
	var runningInstance *provider.Instance
	for i := 0; i < 60; i++ {
		inst, err := p.GetInstance(ctx, instance.ID)
		if err != nil {
			log.Fatalf("Failed to get instance: %v", err)
		}
		
		if inst.Status == "Running" {
			fmt.Println("Pod is running!")
			runningInstance = inst
			break
		}
		
		fmt.Printf("Pod status: %s, waiting...\n", inst.Status)
		
		// If pending for too long, check pod events
		if i > 5 && inst.Status == "Pending" {
			cmd := exec.Command("kubectl", "describe", "pod", inst.PodName, "-n", inst.Namespace)
			output, _ := cmd.CombinedOutput()
			if i == 6 { // Only print once
				fmt.Printf("Pod describe output:\n%s\n", output)
			}
		}
		
		time.Sleep(2 * time.Second)
	}

	if runningInstance == nil || runningInstance.Status != "Running" {
		log.Fatalf("Pod did not reach Running status")
	}

	// Give the pod time to clone the repository
	fmt.Println("Waiting for repository clone to complete...")
	time.Sleep(10 * time.Second)

	// Check if repository files exist in the pod
	fmt.Println("Checking for repository files in pod...")
	cmd := exec.Command("kubectl", "exec", "-n", runningInstance.Namespace, runningInstance.PodName, "--", "ls", "-la", "/workspace")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Failed to list files: %v\nOutput: %s", err, output)
		// Try without /workspace
		cmd = exec.Command("kubectl", "exec", "-n", runningInstance.Namespace, runningInstance.PodName, "--", "pwd")
		pwdOut, _ := cmd.CombinedOutput()
		log.Printf("Current directory: %s", pwdOut)
		
		cmd = exec.Command("kubectl", "exec", "-n", runningInstance.Namespace, runningInstance.PodName, "--", "ls", "-la")
		output, err = cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("Failed to list files in current dir: %v", err)
		}
	}
	
	fmt.Printf("Files in pod:\n%s\n", output)

	// Check for specific repo files
	expectedFiles := []string{"README.md", "go.mod", "main.go", ".git"}
	missingFiles := []string{}
	
	for _, file := range expectedFiles {
		if !strings.Contains(string(output), file) {
			missingFiles = append(missingFiles, file)
		}
	}

	// Also check git status
	fmt.Println("\nChecking git status...")
	cmd = exec.Command("kubectl", "exec", "-n", runningInstance.Namespace, runningInstance.PodName, "--", "git", "status")
	gitOutput, gitErr := cmd.CombinedOutput()
	if gitErr == nil {
		fmt.Printf("Git status:\n%s\n", gitOutput)
	} else {
		fmt.Printf("Git status failed: %v\n", gitErr)
	}

	// Clean up
	fmt.Println("\nDeleting instance...")
	if err := p.DeleteInstance(ctx, instance.ID); err != nil {
		log.Printf("Warning: Failed to delete instance: %v", err)
	}

	// Report results
	if len(missingFiles) > 0 {
		log.Fatalf("❌ Repository clone failed! Missing files: %v", missingFiles)
	} else {
		fmt.Println("✅ Repository clone successful! All expected files found in pod.")
	}
}