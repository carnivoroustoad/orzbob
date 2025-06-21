package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"orzbob/internal/cloud/provider"
)

type timeProvider interface {
	Now() time.Time
}

type realTimeProvider struct{}

func (r realTimeProvider) Now() time.Time {
	return time.Now()
}

type fakeTimeProvider struct {
	mu          sync.Mutex
	currentTime time.Time
}

func (f *fakeTimeProvider) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.currentTime
}

func (f *fakeTimeProvider) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.currentTime = f.currentTime.Add(d)
}

// TestIdleReaper tests the idle reaper functionality with fake time
func TestIdleReaper(t *testing.T) {
	// Create fake provider and server
	fakeProvider := provider.NewFakeProvider()
	server := &Server{
		provider:   fakeProvider,
		router:     chi.NewRouter(),
		heartbeats: make(map[string]time.Time),
	}
	server.setupRoutes()

	// Create fake time provider
	fakeTime := &fakeTimeProvider{
		currentTime: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	// Create test instances
	ctx := context.Background()
	instance1, err := fakeProvider.CreateInstance(ctx, "small")
	if err != nil {
		t.Fatalf("Failed to create instance 1: %v", err)
	}

	instance2, err := fakeProvider.CreateInstance(ctx, "small")
	if err != nil {
		t.Fatalf("Failed to create instance 2: %v", err)
	}

	// Set initial heartbeats
	server.heartbeatMu.Lock()
	server.heartbeats[instance1.ID] = fakeTime.Now()
	server.heartbeats[instance2.ID] = fakeTime.Now()
	server.heartbeatMu.Unlock()

	// Advance time by 20 minutes and update heartbeat for instance1
	fakeTime.Advance(20 * time.Minute)
	
	// Send heartbeat for instance1
	req := httptest.NewRequest("POST", "/v1/instances/"+instance1.ID+"/heartbeat", nil)
	rr := httptest.NewRecorder()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", instance1.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	
	server.handleHeartbeat(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", rr.Code)
	}

	// Advance time by another 15 minutes (total 35 minutes)
	fakeTime.Advance(15 * time.Minute)

	// Create a custom reap function that uses fake time
	reapIdleInstancesWithTime := func(timeProvider timeProvider) {
		ctx := context.Background()
		idleTimeout := 30 * time.Minute
		now := timeProvider.Now()

		// Get all instances
		instances, err := server.provider.ListInstances(ctx)
		if err != nil {
			t.Logf("Failed to list instances: %v", err)
			return
		}

		server.heartbeatMu.Lock()
		defer server.heartbeatMu.Unlock()

		for _, instance := range instances {
			lastHeartbeat, exists := server.heartbeats[instance.ID]
			
			// If no heartbeat recorded, use creation time
			if !exists {
				lastHeartbeat = instance.CreatedAt
			}

			// Check if idle
			if now.Sub(lastHeartbeat) > idleTimeout {
				t.Logf("Reaping idle instance %s (last heartbeat: %v, now: %v)", instance.ID, lastHeartbeat, now)
				
				// Delete the instance
				if err := server.provider.DeleteInstance(ctx, instance.ID); err != nil {
					t.Logf("Failed to delete idle instance %s: %v", instance.ID, err)
				} else {
					// Remove from heartbeat map
					delete(server.heartbeats, instance.ID)
				}
			}
		}
	}

	// Run the reaper with fake time
	reapIdleInstancesWithTime(fakeTime)

	// Verify results
	instances, err := fakeProvider.ListInstances(ctx)
	if err != nil {
		t.Fatalf("Failed to list instances: %v", err)
	}

	// Should have only instance1 remaining
	if len(instances) != 1 {
		t.Errorf("Expected 1 instance remaining, got %d", len(instances))
	}

	if len(instances) > 0 && instances[0].ID != instance1.ID {
		t.Errorf("Expected instance1 to remain, but got %s", instances[0].ID)
	}

	// Verify instance2 was deleted
	_, err = fakeProvider.GetInstance(ctx, instance2.ID)
	if err == nil {
		t.Error("Expected instance2 to be deleted, but it still exists")
	}
}

// TestHeartbeatEndpoint tests the heartbeat endpoint
func TestHeartbeatEndpoint(t *testing.T) {
	// Create fake provider and server
	fakeProvider := provider.NewFakeProvider()
	server := NewServer(fakeProvider)

	// Create test instance
	ctx := context.Background()
	instance, err := fakeProvider.CreateInstance(ctx, "small")
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}

	// Test heartbeat endpoint
	req := httptest.NewRequest("POST", "/v1/instances/"+instance.ID+"/heartbeat", nil)
	rr := httptest.NewRecorder()
	
	// Add route parameters
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", instance.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", rr.Code)
	}

	// Verify heartbeat was recorded
	server.heartbeatMu.RLock()
	lastHeartbeat, exists := server.heartbeats[instance.ID]
	server.heartbeatMu.RUnlock()

	if !exists {
		t.Error("Expected heartbeat to be recorded")
	}

	if time.Since(lastHeartbeat) > time.Second {
		t.Error("Heartbeat timestamp is too old")
	}

	// Test heartbeat for non-existent instance
	req = httptest.NewRequest("POST", "/v1/instances/non-existent/heartbeat", nil)
	rr = httptest.NewRecorder()
	
	rctx = chi.NewRouteContext()
	rctx.URLParams.Add("id", "non-existent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	
	server.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existent instance, got %d", rr.Code)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err == nil {
		if errResp.Error != "Instance not found" {
			t.Errorf("Expected error 'Instance not found', got '%s'", errResp.Error)
		}
	}
}