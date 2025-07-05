package cloud

import (
	"testing"
	"time"
)

func TestCloudInstanceConversion(t *testing.T) {
	// Test converting cloud instances to session instances
	cloudInstances := []CloudInstance{
		{
			ID:        "test-123",
			Status:    "Running",
			Tier:      "small",
			CreatedAt: time.Now(),
			AttachURL: "ws://example.com/attach",
		},
	}
	
	sessionInstances := ConvertToSessionInstances(cloudInstances)
	
	if len(sessionInstances) != 1 {
		t.Fatalf("Expected 1 instance, got %d", len(sessionInstances))
	}
	
	inst := sessionInstances[0]
	if !inst.IsCloud {
		t.Error("Expected IsCloud to be true")
	}
	if inst.CloudInstanceID != "test-123" {
		t.Errorf("Expected CloudInstanceID to be test-123, got %s", inst.CloudInstanceID)
	}
	if inst.CloudTier != "small" {
		t.Errorf("Expected CloudTier to be small, got %s", inst.CloudTier)
	}
	if inst.AttachURL != "ws://example.com/attach" {
		t.Errorf("Expected AttachURL to be ws://example.com/attach, got %s", inst.AttachURL)
	}
}