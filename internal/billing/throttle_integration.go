package billing

import (
	"fmt"
	"log"
)

// InstanceState represents the state of an instance for throttling
type InstanceState string

const (
	InstanceStateRunning InstanceState = "Running"
	InstanceStatePaused  InstanceState = "Paused"
	InstanceStateStopped InstanceState = "Stopped"
)

// InstancePauseInfo contains information about why an instance was paused
type InstancePauseInfo struct {
	State       InstanceState
	Reason      ThrottleReason
	Message     string
	CanResume   bool
	ResumeAfter string // When the instance can be resumed (e.g., "tomorrow", "next billing period")
}

// GetPauseInfo returns detailed pause information for display
func GetPauseInfo(reason ThrottleReason) InstancePauseInfo {
	info := InstancePauseInfo{
		State:     InstanceStatePaused,
		Reason:    reason,
		CanResume: true,
	}

	switch reason {
	case ThrottleReasonContinuousLimit:
		info.Message = "Instance paused: Exceeded 8-hour continuous run limit"
		info.ResumeAfter = "after a break"
	case ThrottleReasonDailyLimit:
		info.Message = "Instance paused: Exceeded 24-hour daily usage cap"
		info.ResumeAfter = "tomorrow"
		info.CanResume = false // Cannot resume until next day
	case ThrottleReasonIdle:
		info.Message = "Instance paused: Idle timeout (30 minutes)"
		info.ResumeAfter = "anytime"
	default:
		info.Message = "Instance paused"
		info.ResumeAfter = "when limits reset"
	}

	return info
}

// Example integration for control plane
type ControlPlaneIntegration struct {
	billingManager *Manager
}

// NewControlPlaneIntegration creates a new control plane integration
func NewControlPlaneIntegration(billingManager *Manager) *ControlPlaneIntegration {
	integration := &ControlPlaneIntegration{
		billingManager: billingManager,
	}

	// Set the pause callback
	billingManager.SetThrottlePauseCallback(integration.pauseInstance)

	return integration
}

// pauseInstance is called when an instance needs to be paused
func (c *ControlPlaneIntegration) pauseInstance(instanceID string, reason ThrottleReason) error {
	log.Printf("Pausing instance %s due to %s", instanceID, reason)

	// In a real implementation, this would:
	// 1. Update instance state in database
	// 2. Send pause command to the instance provider
	// 3. Notify the user
	// 4. Record metrics

	// Example pseudo-code:
	/*
		// Update database
		err := db.UpdateInstanceState(instanceID, InstanceStatePaused, reason)
		if err != nil {
			return fmt.Errorf("failed to update instance state: %w", err)
		}

		// Pause the actual instance
		provider := getProvider(instanceID)
		err = provider.PauseInstance(instanceID)
		if err != nil {
			return fmt.Errorf("failed to pause instance: %w", err)
		}

		// Send notification
		pauseInfo := GetPauseInfo(reason)
		notifyUser(instanceID, pauseInfo)

		// Record metrics
		metrics.InstancesPaused.WithLabelValues(string(reason)).Inc()
	*/

	return nil
}

// OnInstanceCreate is called when a new instance is created
func (c *ControlPlaneIntegration) OnInstanceCreate(instanceID, orgID string) {
	// Register instance for throttle tracking
	c.billingManager.RegisterInstance(instanceID, orgID)
	log.Printf("Registered instance %s for throttle tracking", instanceID)
}

// OnInstanceDelete is called when an instance is deleted
func (c *ControlPlaneIntegration) OnInstanceDelete(instanceID string) {
	// Unregister instance from throttle tracking
	c.billingManager.UnregisterInstance(instanceID)
	log.Printf("Unregistered instance %s from throttle tracking", instanceID)
}

// OnHeartbeat is called when a heartbeat is received from an instance
func (c *ControlPlaneIntegration) OnHeartbeat(instanceID string) {
	// Record activity for idle timeout tracking
	c.billingManager.RecordInstanceActivity(instanceID)
}

// GetInstanceThrottleStatus returns the throttle status of an instance
func (c *ControlPlaneIntegration) GetInstanceThrottleStatus(instanceID string) (paused bool, info *InstancePauseInfo) {
	paused, reason, _ := c.billingManager.GetThrottleService().GetInstanceStatus(instanceID)
	if paused {
		pauseInfo := GetPauseInfo(reason)
		return true, &pauseInfo
	}
	return false, nil
}

// GetOrgDailyUsage returns the daily usage for an organization
func (c *ControlPlaneIntegration) GetOrgDailyUsage(orgID string) string {
	usage := c.billingManager.GetThrottleService().GetOrgDailyUsage(orgID)
	hours := int(usage.Hours())
	minutes := int(usage.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}
