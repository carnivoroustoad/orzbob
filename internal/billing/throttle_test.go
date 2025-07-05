package billing

import (
	"context"
	"testing"
	"time"
)

func TestThrottleService_ContinuousLimit(t *testing.T) {
	quotaEngine, _ := NewQuotaEngine(&MockPolarClient{}, NewMemoryQuotaPersistence())
	throttleService := NewThrottleService(quotaEngine)

	// Set shorter limits for testing
	throttleService.SetLimits(100*time.Millisecond, 1*time.Hour, 1*time.Hour)

	// Track paused instances
	pausedInstances := make(map[string]ThrottleReason)
	throttleService.SetPauseCallback(func(instanceID string, reason ThrottleReason) error {
		pausedInstances[instanceID] = reason
		return nil
	})

	// Register instance
	instanceID := "test-instance-1"
	orgID := "test-org"
	throttleService.RegisterInstance(instanceID, orgID)

	// Manually set start time to exceed continuous limit
	throttleService.mu.Lock()
	if tracker, exists := throttleService.instances[instanceID]; exists {
		tracker.StartTime = time.Now().Add(-200 * time.Millisecond)
	}
	throttleService.mu.Unlock()

	// Check instances (should trigger pause)
	throttleService.checkAllInstances()

	// Verify instance was paused
	if reason, paused := pausedInstances[instanceID]; !paused {
		t.Error("Expected instance to be paused for continuous limit")
	} else if reason != ThrottleReasonContinuousLimit {
		t.Errorf("Expected pause reason to be continuous_limit, got %s", reason)
	}

	// Verify instance state
	paused, reason, runtime := throttleService.GetInstanceStatus(instanceID)
	if !paused {
		t.Error("Expected instance to be marked as paused")
	}
	if reason != ThrottleReasonContinuousLimit {
		t.Errorf("Expected reason to be continuous_limit, got %s", reason)
	}
	if runtime < 200*time.Millisecond {
		t.Errorf("Expected runtime to be at least 200ms, got %v", runtime)
	}
}

func TestThrottleService_IdleTimeout(t *testing.T) {
	quotaEngine, _ := NewQuotaEngine(&MockPolarClient{}, NewMemoryQuotaPersistence())
	throttleService := NewThrottleService(quotaEngine)

	// Set shorter limits for testing
	throttleService.SetLimits(1*time.Hour, 1*time.Hour, 100*time.Millisecond)

	// Track paused instances
	pausedInstances := make(map[string]ThrottleReason)
	throttleService.SetPauseCallback(func(instanceID string, reason ThrottleReason) error {
		pausedInstances[instanceID] = reason
		return nil
	})

	// Register instance
	instanceID := "test-instance-2"
	orgID := "test-org"
	throttleService.RegisterInstance(instanceID, orgID)

	// Manually set last active time to trigger idle timeout
	throttleService.mu.Lock()
	if tracker, exists := throttleService.instances[instanceID]; exists {
		tracker.LastActiveTime = time.Now().Add(-200 * time.Millisecond)
	}
	throttleService.mu.Unlock()

	// Check instances (should trigger pause)
	throttleService.checkAllInstances()

	// Verify instance was paused
	if reason, paused := pausedInstances[instanceID]; !paused {
		t.Error("Expected instance to be paused for idle timeout")
	} else if reason != ThrottleReasonIdle {
		t.Errorf("Expected pause reason to be idle, got %s", reason)
	}
}

func TestThrottleService_DailyLimit(t *testing.T) {
	quotaEngine, _ := NewQuotaEngine(&MockPolarClient{}, NewMemoryQuotaPersistence())
	throttleService := NewThrottleService(quotaEngine)

	// Set daily limit to 1 hour for testing
	throttleService.SetLimits(8*time.Hour, 1*time.Hour, 30*time.Minute)

	// Track paused instances
	pausedInstances := make(map[string]ThrottleReason)
	throttleService.SetPauseCallback(func(instanceID string, reason ThrottleReason) error {
		pausedInstances[instanceID] = reason
		return nil
	})

	orgID := "test-org"
	today := time.Now().Format("2006-01-02")

	// Simulate 45 minutes of usage
	throttleService.mu.Lock()
	if throttleService.orgDailyUsage[orgID] == nil {
		throttleService.orgDailyUsage[orgID] = make(map[string]time.Duration)
	}
	throttleService.orgDailyUsage[orgID][today] = 45 * time.Minute
	throttleService.mu.Unlock()

	// Register new instance that would exceed daily limit
	instanceID := "test-instance-3"
	throttleService.RegisterInstance(instanceID, orgID)

	// Set instance to have been running for 20 minutes
	throttleService.mu.Lock()
	if tracker, exists := throttleService.instances[instanceID]; exists {
		tracker.DayStartTime = time.Now().Add(-20 * time.Minute)
	}
	throttleService.mu.Unlock()

	// Check instances (should trigger pause due to daily limit)
	throttleService.checkAllInstances()

	// Verify instance was paused
	if reason, paused := pausedInstances[instanceID]; !paused {
		t.Error("Expected instance to be paused for daily limit")
	} else if reason != ThrottleReasonDailyLimit {
		t.Errorf("Expected pause reason to be daily_limit, got %s", reason)
	}

	// Verify org daily usage
	usage := throttleService.GetOrgDailyUsage(orgID)
	if usage < 45*time.Minute {
		t.Errorf("Expected daily usage to be at least 45 minutes, got %v", usage)
	}
}

func TestThrottleService_RecordActivity(t *testing.T) {
	quotaEngine, _ := NewQuotaEngine(&MockPolarClient{}, NewMemoryQuotaPersistence())
	throttleService := NewThrottleService(quotaEngine)

	instanceID := "test-instance-4"
	orgID := "test-org"

	// Register instance
	throttleService.RegisterInstance(instanceID, orgID)

	// Get initial last active time
	throttleService.mu.RLock()
	initialTime := throttleService.instances[instanceID].LastActiveTime
	throttleService.mu.RUnlock()

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Record activity
	throttleService.RecordActivity(instanceID)

	// Verify last active time was updated
	throttleService.mu.RLock()
	newTime := throttleService.instances[instanceID].LastActiveTime
	throttleService.mu.RUnlock()

	if !newTime.After(initialTime) {
		t.Error("Expected last active time to be updated")
	}
}

func TestThrottleService_UnregisterInstance(t *testing.T) {
	quotaEngine, _ := NewQuotaEngine(&MockPolarClient{}, NewMemoryQuotaPersistence())
	throttleService := NewThrottleService(quotaEngine)

	instanceID := "test-instance-5"
	orgID := "test-org"

	// Register instance
	throttleService.RegisterInstance(instanceID, orgID)

	// Verify instance exists
	throttleService.mu.RLock()
	_, exists := throttleService.instances[instanceID]
	throttleService.mu.RUnlock()

	if !exists {
		t.Fatal("Expected instance to be registered")
	}

	// Unregister instance
	throttleService.UnregisterInstance(instanceID)

	// Verify instance was removed
	throttleService.mu.RLock()
	_, exists = throttleService.instances[instanceID]
	throttleService.mu.RUnlock()

	if exists {
		t.Error("Expected instance to be unregistered")
	}
}

func TestThrottleService_StartStop(t *testing.T) {
	quotaEngine, _ := NewQuotaEngine(&MockPolarClient{}, NewMemoryQuotaPersistence())
	throttleService := NewThrottleService(quotaEngine)
	throttleService.checkInterval = 10 * time.Millisecond

	ctx := context.Background()

	// Start service
	throttleService.Start(ctx)

	// Let it run briefly
	time.Sleep(50 * time.Millisecond)

	// Stop service
	throttleService.Stop()

	// If Stop() doesn't work properly, the test will timeout
}
