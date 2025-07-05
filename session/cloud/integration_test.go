package cloud

import (
	"context"
	"testing"
	"time"

	"orzbob/session"
)

// TestCloudManagerIntegration tests the CloudManager integration
func TestCloudManagerIntegration(t *testing.T) {
	// Skip if no API URL is set
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	manager := NewManager()

	// Test authentication check
	if manager.IsAuthenticated() {
		t.Log("Manager is authenticated")
	} else {
		t.Skip("Manager is not authenticated, skipping integration tests")
	}

	ctx := context.Background()

	// Test listing instances
	t.Run("ListInstances", func(t *testing.T) {
		instances, err := manager.ListInstances(ctx)
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}

		t.Logf("Found %d cloud instances", len(instances))
		for _, inst := range instances {
			t.Logf("  - %s: %s (%s)", inst.ID, inst.Status, inst.Tier)
		}
	})

	// Test converting to session instances
	t.Run("ConvertToSessionInstances", func(t *testing.T) {
		cloudInstances := []CloudInstance{
			{
				ID:        "test-cloud-123",
				Status:    "Running",
				Tier:      "medium",
				CreatedAt: time.Now(),
				AttachURL: "wss://api.orzbob.com/v1/instances/test-cloud-123/attach",
			},
		}

		sessionInstances := ConvertToSessionInstances(cloudInstances)

		if len(sessionInstances) != 1 {
			t.Fatalf("Expected 1 session instance, got %d", len(sessionInstances))
		}

		inst := sessionInstances[0]
		if !inst.IsCloud {
			t.Error("Expected IsCloud to be true")
		}
		if inst.CloudInstanceID != "test-cloud-123" {
			t.Errorf("Expected CloudInstanceID to be test-cloud-123, got %s", inst.CloudInstanceID)
		}
		if inst.CloudTier != "medium" {
			t.Errorf("Expected CloudTier to be medium, got %s", inst.CloudTier)
		}
		if inst.Title != "cloud-test-clou" {
			t.Errorf("Expected Title to be cloud-test-clou, got %s", inst.Title)
		}
	})
}

// TestCloudInstanceUI tests the UI rendering of cloud instances
func TestCloudInstanceUI(t *testing.T) {
	// Create a cloud instance
	cloudInst := &session.Instance{
		Title:           "My Cloud Instance",
		Path:            "/cloud/test-123",
		Branch:          "",
		Status:          session.Ready,
		Program:         "claude",
		IsCloud:         true,
		CloudInstanceID: "test-123",
		CloudTier:       "large",
		CloudStatus:     "Running",
		CreatedAt:       time.Now(),
	}

	// Test that cloud fields are preserved in serialization
	data := cloudInst.ToInstanceData()
	if !data.IsCloud {
		t.Error("Expected IsCloud to be preserved in InstanceData")
	}
	if data.CloudTier != "large" {
		t.Errorf("Expected CloudTier to be large, got %s", data.CloudTier)
	}
}

// TestWebSocketTmuxIntegration tests WebSocket tmux integration
func TestWebSocketTmuxIntegration(t *testing.T) {
	// This is a placeholder for WebSocket tmux testing
	// In a real test, we would:
	// 1. Create a mock WebSocket server
	// 2. Test WSAttach method
	// 3. Test reconnection logic
	// 4. Test terminal I/O over WebSocket

	t.Skip("WebSocket tmux integration test not implemented")
}
