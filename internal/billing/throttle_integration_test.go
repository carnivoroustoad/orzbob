package billing

import (
	"testing"
	"time"
)

func TestThrottleControlPlaneIntegration(t *testing.T) {
	// Create mock client and manager
	mockClient := &MockPolarClient{}
	
	quotaEngine, _ := NewQuotaEngine(mockClient, NewMemoryQuotaPersistence())
	throttleService := NewThrottleService(quotaEngine)
	
	manager := &Manager{
		client:          mockClient,
		quotaEngine:     quotaEngine,
		throttleService: throttleService,
	}
	
	// Create control plane integration
	integration := NewControlPlaneIntegration(manager)
	
	// Test instance lifecycle
	t.Run("instance lifecycle", func(t *testing.T) {
		instanceID := "test-instance"
		orgID := "test-org"
		
		// Create instance
		integration.OnInstanceCreate(instanceID, orgID)
		
		// Check status (should not be paused)
		paused, info := integration.GetInstanceThrottleStatus(instanceID)
		if paused {
			t.Error("Expected instance to not be paused initially")
		}
		if info != nil {
			t.Error("Expected no pause info for running instance")
		}
		
		// Send heartbeat
		integration.OnHeartbeat(instanceID)
		
		// Delete instance
		integration.OnInstanceDelete(instanceID)
		
		// Verify instance is no longer tracked
		paused, _, _ = manager.GetThrottleService().GetInstanceStatus(instanceID)
		if paused {
			t.Error("Expected deleted instance to not be tracked")
		}
	})
	
	// Test pause info messages
	t.Run("pause info messages", func(t *testing.T) {
		tests := []struct {
			reason      ThrottleReason
			expectedMsg string
			canResume   bool
		}{
			{
				reason:      ThrottleReasonContinuousLimit,
				expectedMsg: "Instance paused: Exceeded 8-hour continuous run limit",
				canResume:   true,
			},
			{
				reason:      ThrottleReasonDailyLimit,
				expectedMsg: "Instance paused: Exceeded 24-hour daily usage cap",
				canResume:   false,
			},
			{
				reason:      ThrottleReasonIdle,
				expectedMsg: "Instance paused: Idle timeout (30 minutes)",
				canResume:   true,
			},
		}
		
		for _, tt := range tests {
			info := GetPauseInfo(tt.reason)
			if info.Message != tt.expectedMsg {
				t.Errorf("Expected message %q, got %q", tt.expectedMsg, info.Message)
			}
			if info.CanResume != tt.canResume {
				t.Errorf("Expected CanResume=%v for %s", tt.canResume, tt.reason)
			}
		}
	})
	
	// Test daily usage formatting
	t.Run("daily usage formatting", func(t *testing.T) {
		orgID := "test-org"
		
		// Set some daily usage
		manager.GetThrottleService().mu.Lock()
		today := time.Now().Format("2006-01-02")
		if manager.GetThrottleService().orgDailyUsage[orgID] == nil {
			manager.GetThrottleService().orgDailyUsage[orgID] = make(map[string]time.Duration)
		}
		manager.GetThrottleService().orgDailyUsage[orgID][today] = 3*time.Hour + 45*time.Minute
		manager.GetThrottleService().mu.Unlock()
		
		// Get formatted usage
		usage := integration.GetOrgDailyUsage(orgID)
		if usage != "3h 45m" {
			t.Errorf("Expected usage '3h 45m', got %q", usage)
		}
	})
}

func TestGetPauseInfo(t *testing.T) {
	tests := []struct {
		reason        ThrottleReason
		expectedState InstanceState
		canResume     bool
		resumeAfter   string
	}{
		{
			reason:        ThrottleReasonContinuousLimit,
			expectedState: InstanceStatePaused,
			canResume:     true,
			resumeAfter:   "after a break",
		},
		{
			reason:        ThrottleReasonDailyLimit,
			expectedState: InstanceStatePaused,
			canResume:     false,
			resumeAfter:   "tomorrow",
		},
		{
			reason:        ThrottleReasonIdle,
			expectedState: InstanceStatePaused,
			canResume:     true,
			resumeAfter:   "anytime",
		},
	}
	
	for _, tt := range tests {
		t.Run(string(tt.reason), func(t *testing.T) {
			info := GetPauseInfo(tt.reason)
			
			if info.State != tt.expectedState {
				t.Errorf("Expected state %s, got %s", tt.expectedState, info.State)
			}
			
			if info.Reason != tt.reason {
				t.Errorf("Expected reason %s, got %s", tt.reason, info.Reason)
			}
			
			if info.CanResume != tt.canResume {
				t.Errorf("Expected CanResume %v, got %v", tt.canResume, info.CanResume)
			}
			
			if info.ResumeAfter != tt.resumeAfter {
				t.Errorf("Expected ResumeAfter %q, got %q", tt.resumeAfter, info.ResumeAfter)
			}
		})
	}
}