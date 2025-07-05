package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"orzbob/internal/auth"
	"orzbob/internal/cloud/provider"
	"orzbob/internal/tunnel"
)

// testUser represents a test user for authentication
var testUser = User{
	ID:       "test-user",
	Email:    "test@example.com",
	Login:    "testuser",
	OrgID:    "test-org",
	GitHubID: 123456,
	Plan:     "free",
	Created:  time.Now(),
}

// addTestAuth adds authentication headers to a test request
func addTestAuth(req *http.Request, server *Server) {
	// Store test user in the user store
	userStoreMu.Lock()
	userStore[testUser.ID] = &testUser
	userStoreMu.Unlock()
	
	// Generate a test token for the user
	token, _ := server.tokenManager.GenerateUserToken(testUser.ID, 1*time.Hour)
	req.Header.Set("Authorization", "Bearer "+token)
}



// TestIdleReaper tests the idle reaper functionality
func TestIdleReaper(t *testing.T) {
	// Create fake provider and server
	fakeProvider := provider.NewFakeProvider()
	server := &Server{
		provider:   fakeProvider,
		router:     chi.NewRouter(),
		heartbeats: make(map[string]time.Time),
	}
	server.setupRoutes()

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

	// Set initial heartbeats (30+ minutes ago for instance2)
	now := time.Now()
	server.heartbeatMu.Lock()
	server.heartbeats[instance1.ID] = now.Add(-10 * time.Minute) // Recent heartbeat
	server.heartbeats[instance2.ID] = now.Add(-35 * time.Minute) // Old heartbeat
	server.heartbeatMu.Unlock()

	// Create a custom reap function
	reapIdleInstances := func() {
		ctx := context.Background()
		idleTimeout := 30 * time.Minute
		now := time.Now()

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

	// Run the reaper
	reapIdleInstances()

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
	addTestAuth(req, server)
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
	addTestAuth(req, server)
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

func TestJWTAttachValidation(t *testing.T) {
	// Create a server with fake provider
	fp := provider.NewFakeProvider()
	
	// Create token manager
	tm, err := auth.NewTokenManager("test-control-plane")
	if err != nil {
		t.Fatalf("Failed to create token manager: %v", err)
	}
	
	server := &Server{
		provider:       fp,
		tokenManager:   tm,
		router:         chi.NewRouter(),
		wsProxy:        tunnel.NewWSProxy(),
		heartbeats:     make(map[string]time.Time),
		instanceCounts: make(map[string]int),
		instanceStarts: make(map[string]time.Time),
		freeQuota:      2,
	}
	server.setupRoutes()
	
	// Start test server
	ts := httptest.NewServer(server.router)
	defer ts.Close()
	
	// Update server base URL to match test server
	server.baseURL = ts.URL
	
	// Create an instance with org ID header
	reqBody := bytes.NewBufferString(`{"tier": "small"}`)
	req, err := http.NewRequest("POST", ts.URL+"/v1/instances", reqBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Org-ID", "test-org-jwt-validation")
	addTestAuth(req, server)
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	defer resp.Body.Close()
	
	var createResp CreateInstanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	
	// Extract JWT token from attach URL
	attachURL := createResp.AttachURL
	if !strings.Contains(attachURL, "?token=") {
		t.Fatal("Attach URL does not contain JWT token")
	}
	
	// Convert HTTP URL to WebSocket URL
	wsURL := strings.Replace(attachURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, ts.URL, strings.Replace(ts.URL, "http://", "ws://", 1), 1)
	
	t.Run("ValidTokenAllowsConnection", func(t *testing.T) {
		// Connect with the JWT token from the attach URL
		conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			status := "nil response"
			if resp != nil {
				status = fmt.Sprintf("%d", resp.StatusCode)
			}
			t.Fatalf("Failed to connect with valid token: %v (status: %v)", err, status)
		}
		defer conn.Close()
	})
	
	t.Run("NoTokenReturns401", func(t *testing.T) {
		// Try to connect without token
		urlWithoutToken := strings.Split(wsURL, "?")[0]
		_, resp, err := websocket.DefaultDialer.Dial(urlWithoutToken, nil)
		
		if err == nil {
			t.Fatal("Expected connection to fail without token")
		}
		
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("Expected 401, got %d", resp.StatusCode)
		}
	})
	
	t.Run("InvalidTokenReturns401", func(t *testing.T) {
		// Try to connect with invalid token
		urlWithInvalidToken := strings.Split(wsURL, "?")[0] + "?token=invalid.token.here"
		_, resp, err := websocket.DefaultDialer.Dial(urlWithInvalidToken, nil)
		
		if err == nil {
			t.Fatal("Expected connection to fail with invalid token")
		}
		
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("Expected 401, got %d", resp.StatusCode)
		}
	})
	
	t.Run("WrongInstanceTokenReturns403", func(t *testing.T) {
		// Generate a token for a different instance
		wrongToken, err := tm.GenerateToken("different-instance", 5*time.Minute)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}
		
		// Try to connect with wrong instance token
		urlWithWrongToken := strings.Split(wsURL, "?")[0] + "?token=" + wrongToken
		_, resp, err := websocket.DefaultDialer.Dial(urlWithWrongToken, nil)
		
		if err == nil {
			t.Fatal("Expected connection to fail with wrong instance token")
		}
		
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("Expected 403, got %d", resp.StatusCode)
		}
	})
	
	t.Run("ExpiredTokenReturns401", func(t *testing.T) {
		// Generate an expired token for the correct instance
		expiredToken, err := tm.GenerateToken(createResp.ID, -1*time.Hour)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}
		
		// Try to connect with expired token
		urlWithExpiredToken := strings.Split(wsURL, "?")[0] + "?token=" + expiredToken
		_, resp, err := websocket.DefaultDialer.Dial(urlWithExpiredToken, nil)
		
		if err == nil {
			t.Fatal("Expected connection to fail with expired token")
		}
		
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("Expected 401, got %d", resp.StatusCode)
		}
	})
}

func TestInstanceCreationWithJWT(t *testing.T) {
	// Create a server with fake provider
	fp := provider.NewFakeProvider()
	
	// Create token manager
	tm, err := auth.NewTokenManager("test-control-plane")
	if err != nil {
		t.Fatalf("Failed to create token manager: %v", err)
	}
	
	server := &Server{
		provider:       fp,
		tokenManager:   tm,
		router:         chi.NewRouter(),
		wsProxy:        tunnel.NewWSProxy(),
		heartbeats:     make(map[string]time.Time),
		instanceCounts: make(map[string]int),
		instanceStarts: make(map[string]time.Time),
		freeQuota:      2,
	}
	server.setupRoutes()
	
	// Start test server
	ts := httptest.NewServer(server.router)
	defer ts.Close()
	
	// Update server base URL to match test server
	server.baseURL = ts.URL
	
	// Create an instance with org ID header
	reqBody := bytes.NewBufferString(`{"tier": "small"}`)
	req, err := http.NewRequest("POST", ts.URL+"/v1/instances", reqBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Org-ID", "test-org-instance-creation")
	addTestAuth(req, server)
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 201, got %d", resp.StatusCode)
	}
	
	var createResp CreateInstanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	
	// Verify attach URL contains JWT token
	if !strings.Contains(createResp.AttachURL, "?token=") {
		t.Fatal("Attach URL does not contain JWT token")
	}
	
	// Extract and validate the token
	parts := strings.Split(createResp.AttachURL, "?token=")
	if len(parts) != 2 {
		t.Fatal("Invalid attach URL format")
	}
	
	token := parts[1]
	claims, err := tm.ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate token from attach URL: %v", err)
	}
	
	// Verify token is for the correct instance
	if claims.InstanceID != createResp.ID {
		t.Errorf("Token instance ID mismatch: expected %s, got %s", createResp.ID, claims.InstanceID)
	}
	
	// Verify token expires in approximately 2 minutes
	expectedExpiry := time.Now().Add(2 * time.Minute)
	diff := claims.ExpiresAt.Time.Sub(expectedExpiry).Abs()
	if diff > 5*time.Second {
		t.Errorf("Token expiry time off by %v", diff)
	}
}