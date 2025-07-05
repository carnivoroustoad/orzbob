//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"orzbob/internal/cloud/provider"
)

func TestKindProvider(t *testing.T) {
	// Get kubeconfig path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}
	kubeconfig := filepath.Join(homeDir, ".kube", "config")

	// Create provider
	p, err := provider.NewLocalKind(kubeconfig)
	if err != nil {
		t.Fatalf("Failed to create LocalKind provider: %v", err)
	}

	ctx := context.Background()

	// Test 1: Create instance
	t.Log("Creating instance...")
	instance, err := p.CreateInstance(ctx, "small")
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	t.Logf("Created instance: %s", instance.ID)

	// Test 2: Wait for pod to be running
	t.Log("Waiting for pod to be running...")
	var runningInstance *provider.Instance
	for i := 0; i < 30; i++ { // Wait up to 30 seconds
		runningInstance, err = p.GetInstance(ctx, instance.ID)
		if err != nil {
			t.Fatalf("Failed to get instance: %v", err)
		}
		
		if runningInstance.Status == "Running" {
			t.Logf("Pod is running!")
			break
		}
		
		t.Logf("Pod status: %s, waiting...", runningInstance.Status)
		time.Sleep(1 * time.Second)
	}

	if runningInstance.Status != "Running" {
		t.Fatalf("Pod did not reach Running status, current status: %s", runningInstance.Status)
	}

	// Test 3: List instances
	t.Log("Listing instances...")
	instances, err := p.ListInstances(ctx)
	if err != nil {
		t.Fatalf("Failed to list instances: %v", err)
	}
	
	found := false
	for _, inst := range instances {
		if inst.ID == instance.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Created instance not found in list")
	}
	t.Logf("Found %d instances", len(instances))

	// Test 4: Get attach URL
	attachURL, err := p.GetAttachURL(ctx, instance.ID)
	if err != nil {
		t.Fatalf("Failed to get attach URL: %v", err)
	}
	t.Logf("Attach URL: %s", attachURL)

	// Test 5: Delete instance
	t.Log("Deleting instance...")
	err = p.DeleteInstance(ctx, instance.ID)
	if err != nil {
		t.Fatalf("Failed to delete instance: %v", err)
	}
	t.Logf("Instance deleted successfully")

	// For C-04, we just verify the delete operation succeeds
	// The actual pod deletion is eventually consistent in Kubernetes
	t.Log("Delete operation completed successfully (pod deletion is eventually consistent)")
}