package session

import (
	"testing"
	"time"
)

func TestCloudInstanceSerialization(t *testing.T) {
	// Create a cloud instance
	inst := &Instance{
		Title:           "test-cloud",
		Path:            "/cloud/123",
		Branch:          "",
		Status:          Ready,
		Program:         "claude",
		IsCloud:         true,
		CloudInstanceID: "cloud-123",
		AttachURL:       "ws://example.com/attach",
		CloudTier:       "medium",
		CloudStatus:     "Running",
		CreatedAt:       time.Now(),
	}

	// Convert to data
	data := inst.ToInstanceData()

	// Verify cloud fields are preserved
	if !data.IsCloud {
		t.Error("Expected IsCloud to be preserved")
	}
	if data.CloudInstanceID != "cloud-123" {
		t.Errorf("Expected CloudInstanceID to be cloud-123, got %s", data.CloudInstanceID)
	}
	if data.CloudTier != "medium" {
		t.Errorf("Expected CloudTier to be medium, got %s", data.CloudTier)
	}

	// Convert back from data
	restored, err := FromInstanceData(data)
	if err != nil {
		t.Fatalf("Failed to restore from data: %v", err)
	}

	// Verify restored instance has cloud fields
	if !restored.IsCloud {
		t.Error("Expected restored IsCloud to be true")
	}
	if restored.CloudInstanceID != "cloud-123" {
		t.Errorf("Expected restored CloudInstanceID to be cloud-123, got %s", restored.CloudInstanceID)
	}
	if restored.AttachURL != "ws://example.com/attach" {
		t.Errorf("Expected restored AttachURL to be ws://example.com/attach, got %s", restored.AttachURL)
	}
}
