package billing

import (
	"context"
	"log"
	"sync"
	"time"
)

// ThrottleReason represents why an instance was throttled
type ThrottleReason string

const (
	ThrottleReasonContinuousLimit ThrottleReason = "continuous_limit" // 8h continuous run
	ThrottleReasonDailyLimit      ThrottleReason = "daily_limit"      // 24h daily cap
	ThrottleReasonIdle            ThrottleReason = "idle"             // Idle timeout
)

// InstanceUsageTracker tracks instance usage for throttling
type InstanceUsageTracker struct {
	InstanceID     string
	OrgID          string
	StartTime      time.Time
	LastActiveTime time.Time
	DailyUsage     time.Duration
	DayStartTime   time.Time
	IsPaused       bool
	PauseReason    ThrottleReason
}

// ThrottleService manages instance throttling and daily caps
type ThrottleService struct {
	quotaEngine   *QuotaEngine
	instances     map[string]*InstanceUsageTracker    // instanceID -> tracker
	orgDailyUsage map[string]map[string]time.Duration // orgID -> date -> duration
	mu            sync.RWMutex
	checkInterval time.Duration
	stopCh        chan struct{}
	wg            sync.WaitGroup

	// Configurable limits
	continuousLimit time.Duration // Default: 8 hours
	dailyLimit      time.Duration // Default: 24 hours
	idleTimeout     time.Duration // Default: 30 minutes

	// Callback for pausing instances
	pauseCallback func(instanceID string, reason ThrottleReason) error
}

// NewThrottleService creates a new throttle service
func NewThrottleService(quotaEngine *QuotaEngine) *ThrottleService {
	return &ThrottleService{
		quotaEngine:     quotaEngine,
		instances:       make(map[string]*InstanceUsageTracker),
		orgDailyUsage:   make(map[string]map[string]time.Duration),
		checkInterval:   1 * time.Minute,
		stopCh:          make(chan struct{}),
		continuousLimit: 8 * time.Hour,
		dailyLimit:      24 * time.Hour,
		idleTimeout:     30 * time.Minute,
	}
}

// SetPauseCallback sets the callback function for pausing instances
func (t *ThrottleService) SetPauseCallback(callback func(instanceID string, reason ThrottleReason) error) {
	t.pauseCallback = callback
}

// SetLimits configures the throttle limits
func (t *ThrottleService) SetLimits(continuous, daily, idle time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.continuousLimit = continuous
	t.dailyLimit = daily
	t.idleTimeout = idle
}

// Start begins the throttle checking routine
func (t *ThrottleService) Start(ctx context.Context) {
	t.wg.Add(1)
	go t.checkRoutine(ctx)
}

// Stop stops the throttle service
func (t *ThrottleService) Stop() {
	close(t.stopCh)
	t.wg.Wait()
}

// RegisterInstance registers a new instance for tracking
func (t *ThrottleService) RegisterInstance(instanceID, orgID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	t.instances[instanceID] = &InstanceUsageTracker{
		InstanceID:     instanceID,
		OrgID:          orgID,
		StartTime:      now,
		LastActiveTime: now,
		DayStartTime:   now,
		IsPaused:       false,
	}

	// Initialize daily usage map for org if needed
	if t.orgDailyUsage[orgID] == nil {
		t.orgDailyUsage[orgID] = make(map[string]time.Duration)
	}
}

// UnregisterInstance removes an instance from tracking
func (t *ThrottleService) UnregisterInstance(instanceID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if tracker, exists := t.instances[instanceID]; exists {
		// Record final usage for the day
		if !tracker.IsPaused {
			t.recordDailyUsage(tracker, time.Now())
		}
		delete(t.instances, instanceID)
	}
}

// RecordActivity records activity for an instance (heartbeat)
func (t *ThrottleService) RecordActivity(instanceID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if tracker, exists := t.instances[instanceID]; exists {
		tracker.LastActiveTime = time.Now()
	}
}

// checkRoutine periodically checks all instances for throttling
func (t *ThrottleService) checkRoutine(ctx context.Context) {
	defer t.wg.Done()

	ticker := time.NewTicker(t.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stopCh:
			return
		case <-ticker.C:
			t.checkAllInstances()
		}
	}
}

// checkAllInstances checks all instances for throttling conditions
func (t *ThrottleService) checkAllInstances() {
	t.mu.Lock()
	instances := make([]*InstanceUsageTracker, 0, len(t.instances))
	for _, tracker := range t.instances {
		instances = append(instances, tracker)
	}
	t.mu.Unlock()

	now := time.Now()
	today := now.Format("2006-01-02")

	for _, tracker := range instances {
		if tracker.IsPaused {
			continue // Already paused
		}

		// Check continuous run limit (8 hours)
		continuousRun := now.Sub(tracker.StartTime)
		if continuousRun > t.continuousLimit {
			log.Printf("Instance %s exceeded continuous limit (%v > %v)",
				tracker.InstanceID, continuousRun, t.continuousLimit)
			t.pauseInstance(tracker.InstanceID, ThrottleReasonContinuousLimit)
			continue
		}

		// Check idle timeout
		idleTime := now.Sub(tracker.LastActiveTime)
		if idleTime > t.idleTimeout {
			log.Printf("Instance %s idle timeout (%v > %v)",
				tracker.InstanceID, idleTime, t.idleTimeout)
			t.pauseInstance(tracker.InstanceID, ThrottleReasonIdle)
			continue
		}

		// Check daily cap
		t.mu.RLock()
		dailyUsage := t.getOrgDailyUsage(tracker.OrgID, today)
		t.mu.RUnlock()

		// Add current session time
		sessionTime := now.Sub(tracker.DayStartTime)
		totalDailyUsage := dailyUsage + sessionTime

		if totalDailyUsage > t.dailyLimit {
			log.Printf("Org %s exceeded daily limit (%v > %v)",
				tracker.OrgID, totalDailyUsage, t.dailyLimit)
			t.pauseInstance(tracker.InstanceID, ThrottleReasonDailyLimit)
		}
	}
}

// pauseInstance pauses an instance
func (t *ThrottleService) pauseInstance(instanceID string, reason ThrottleReason) {
	t.mu.Lock()
	tracker, exists := t.instances[instanceID]
	if !exists || tracker.IsPaused {
		t.mu.Unlock()
		return
	}

	// Record usage before pausing
	t.recordDailyUsage(tracker, time.Now())
	tracker.IsPaused = true
	tracker.PauseReason = reason
	t.mu.Unlock()

	// Call pause callback
	if t.pauseCallback != nil {
		if err := t.pauseCallback(instanceID, reason); err != nil {
			log.Printf("Failed to pause instance %s: %v", instanceID, err)
			// Revert pause state on error
			t.mu.Lock()
			tracker.IsPaused = false
			t.mu.Unlock()
		}
	}
}

// recordDailyUsage records usage for daily tracking
func (t *ThrottleService) recordDailyUsage(tracker *InstanceUsageTracker, endTime time.Time) {
	duration := endTime.Sub(tracker.DayStartTime)
	today := tracker.DayStartTime.Format("2006-01-02")

	if t.orgDailyUsage[tracker.OrgID] == nil {
		t.orgDailyUsage[tracker.OrgID] = make(map[string]time.Duration)
	}

	t.orgDailyUsage[tracker.OrgID][today] += duration

	// If we've crossed into a new day, reset the day start time
	if endTime.Format("2006-01-02") != today {
		tracker.DayStartTime = time.Date(endTime.Year(), endTime.Month(), endTime.Day(), 0, 0, 0, 0, endTime.Location())
	}
}

// getOrgDailyUsage gets the total usage for an org on a specific day
func (t *ThrottleService) getOrgDailyUsage(orgID, date string) time.Duration {
	if orgUsage, exists := t.orgDailyUsage[orgID]; exists {
		return orgUsage[date]
	}
	return 0
}

// GetInstanceStatus returns the status of an instance
func (t *ThrottleService) GetInstanceStatus(instanceID string) (paused bool, reason ThrottleReason, continuousRuntime time.Duration) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if tracker, exists := t.instances[instanceID]; exists {
		return tracker.IsPaused, tracker.PauseReason, time.Since(tracker.StartTime)
	}

	return false, "", 0
}

// GetOrgDailyUsage returns the daily usage for an organization
func (t *ThrottleService) GetOrgDailyUsage(orgID string) time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	today := time.Now().Format("2006-01-02")
	return t.getOrgDailyUsage(orgID, today)
}

// ResetDailyUsage resets daily usage (for testing or manual reset)
func (t *ThrottleService) ResetDailyUsage() {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Clear all daily usage
	t.orgDailyUsage = make(map[string]map[string]time.Duration)

	// Reset day start times for all instances
	now := time.Now()
	for _, tracker := range t.instances {
		tracker.DayStartTime = now
	}
}
