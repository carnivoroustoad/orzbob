package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"orzbob/session"
)

func TestCloudInstanceRendering(t *testing.T) {
	// Create a spinner for the renderer
	s := spinner.New()

	// Create a list with cloud instances
	list := NewList(&s, false)
	list.SetSize(80, 24)

	// Add a regular instance
	regularInst := &session.Instance{
		Title:     "Regular Instance",
		Path:      "/home/user/project",
		Branch:    "main",
		Status:    session.Ready,
		Program:   "claude",
		IsCloud:   false,
		CreatedAt: time.Now(),
	}

	// Add a cloud instance
	cloudInst := &session.Instance{
		Title:           "Cloud Instance",
		Path:            "/cloud/test-123",
		Branch:          "",
		Status:          session.Running,
		Program:         "claude",
		IsCloud:         true,
		CloudInstanceID: "test-123",
		CloudTier:       "medium",
		CloudStatus:     "Running",
		CreatedAt:       time.Now(),
	}

	list.AddInstance(regularInst)
	list.AddInstance(cloudInst)

	// Render the list
	output := list.String()

	// Test that cloud icon appears
	if !strings.Contains(output, cloudIcon) {
		t.Error("Expected cloud icon to appear in list output")
	}

	// Test that cloud instance shows tier
	if !strings.Contains(output, "[medium]") {
		t.Error("Expected cloud tier [medium] to appear in list output")
	}

	// Test that regular instance shows branch
	if !strings.Contains(output, "main") {
		t.Error("Expected branch 'main' to appear for regular instance")
	}
}

func TestCloudInstanceWithDifferentStatuses(t *testing.T) {
	s := spinner.New()
	list := NewList(&s, false)
	list.SetSize(80, 24)

	// Test cloud instance with non-running status
	cloudInst := &session.Instance{
		Title:           "Stopped Cloud Instance",
		Path:            "/cloud/test-456",
		Branch:          "",
		Status:          session.Paused,
		Program:         "claude",
		IsCloud:         true,
		CloudInstanceID: "test-456",
		CloudTier:       "small",
		CloudStatus:     "Stopped",
		CreatedAt:       time.Now(),
	}

	list.AddInstance(cloudInst)
	output := list.String()

	// Test that status is shown
	if !strings.Contains(output, "[small - Stopped]") {
		t.Error("Expected cloud status [small - Stopped] to appear in list output")
	}
}

func TestMenuWithCloudOption(t *testing.T) {
	menu := NewMenu()

	// Test default menu has cloud option
	menu.SetState(StateEmpty)
	output := menu.String()

	// The menu should show the cloud key
	if !strings.Contains(output, "C") {
		t.Error("Expected 'C' key for cloud in menu")
	}

	// Test with instance selected
	inst := &session.Instance{
		Title:   "Test Instance",
		Status:  session.Ready,
		IsCloud: true,
	}
	menu.SetInstance(inst)

	output = menu.String()
	// Should still have cloud option
	if !strings.Contains(output, "C") {
		t.Error("Expected 'C' key for cloud in menu with instance")
	}
}
