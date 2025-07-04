package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"orzbob/internal/auth"
	"orzbob/internal/cloud/provider"
	"orzbob/internal/tunnel"
)

// createTestUser creates a test user with the given org ID
func createTestUser(orgID string) *User {
	// Always use the same user ID for a given org to ensure consistency
	return &User{
		ID:       "user-for-" + orgID,
		Email:    orgID + "@example.com",
		Login:    "user-" + orgID,
		OrgID:    orgID,
		GitHubID: 123456,
		Plan:     "free",
		Created:  time.Now(),
	}
}

// addAuthForOrg adds authentication for a specific org
func addAuthForOrg(req *http.Request, server *Server, orgID string) {
	user := createTestUser(orgID)

	// Store test user in the user store
	userStoreMu.Lock()
	userStore[user.ID] = user
	userStoreMu.Unlock()

	// Generate a test token for the user
	token, _ := server.tokenManager.GenerateUserToken(user.ID, 1*time.Hour)
	req.Header.Set("Authorization", "Bearer "+token)
}

func TestQuotaEnforcement(t *testing.T) {
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
		freeQuota:      2, // Free tier allows 2 instances
	}
	server.setupRoutes()

	// Start test server
	ts := httptest.NewServer(server.router)
	defer ts.Close()

	// Update server base URL to match test server
	server.baseURL = ts.URL

	t.Run("FirstInstanceAllowed", func(t *testing.T) {
		reqBody := bytes.NewBufferString(`{"tier": "small"}`)
		req, _ := http.NewRequest("POST", ts.URL+"/v1/instances", reqBody)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Org-ID", "test-org-1")
		addAuthForOrg(req, server, "test-org-1")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to create first instance: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected 201, got %d", resp.StatusCode)
		}

		var createResp CreateInstanceResponse
		_ = json.NewDecoder(resp.Body).Decode(&createResp)
		t.Logf("Created first instance: %s", createResp.ID)
	})

	t.Run("SecondInstanceAllowed", func(t *testing.T) {
		reqBody := bytes.NewBufferString(`{"tier": "small"}`)
		req, _ := http.NewRequest("POST", ts.URL+"/v1/instances", reqBody)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Org-ID", "test-org-1")
		addAuthForOrg(req, server, "test-org-1")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to create second instance: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected 201, got %d", resp.StatusCode)
		}

		var createResp CreateInstanceResponse
		_ = json.NewDecoder(resp.Body).Decode(&createResp)
		t.Logf("Created second instance: %s", createResp.ID)
	})

	t.Run("ThirdInstanceDenied", func(t *testing.T) {
		reqBody := bytes.NewBufferString(`{"tier": "small"}`)
		req, _ := http.NewRequest("POST", ts.URL+"/v1/instances", reqBody)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Org-ID", "test-org-1")
		addAuthForOrg(req, server, "test-org-1")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to attempt third instance: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusTooManyRequests {
			t.Errorf("Expected 429, got %d", resp.StatusCode)
		}

		var errResp ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		t.Logf("Error response: %s", errResp.Error)

		if errResp.Error != "Quota exceeded: maximum 2 instances allowed for free tier" {
			t.Errorf("Unexpected error message: %s", errResp.Error)
		}
	})

	t.Run("DifferentOrgCanCreateInstance", func(t *testing.T) {
		reqBody := bytes.NewBufferString(`{"tier": "small"}`)
		req, _ := http.NewRequest("POST", ts.URL+"/v1/instances", reqBody)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Org-ID", "test-org-2") // Different org
		addAuthForOrg(req, server, "test-org-2")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to create instance for different org: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected 201 for different org, got %d", resp.StatusCode)
		}

		var createResp CreateInstanceResponse
		_ = json.NewDecoder(resp.Body).Decode(&createResp)
		t.Logf("Created instance for different org: %s", createResp.ID)
	})
}

func TestQuotaDecrementOnDelete(t *testing.T) {
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

	// Create two instances to hit quota
	var instanceIDs []string
	for i := 0; i < 2; i++ {
		reqBody := bytes.NewBufferString(`{"tier": "small"}`)
		req, _ := http.NewRequest("POST", ts.URL+"/v1/instances", reqBody)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Org-ID", "test-org-delete")
		addAuthForOrg(req, server, "test-org-delete")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to create instance %d: %v", i+1, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Failed to create instance %d: got status %d", i+1, resp.StatusCode)
		}

		var createResp CreateInstanceResponse
		_ = json.NewDecoder(resp.Body).Decode(&createResp)
		instanceIDs = append(instanceIDs, createResp.ID)
		t.Logf("Created instance %d: %s", i+1, createResp.ID)
	}

	// Try to create third (should fail)
	reqBody := bytes.NewBufferString(`{"tier": "small"}`)
	req, _ := http.NewRequest("POST", ts.URL+"/v1/instances", reqBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Org-ID", "test-org-delete")
	addAuthForOrg(req, server, "test-org-delete")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to attempt third instance: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected 429 before delete, got %d", resp.StatusCode)
	}

	// Delete one instance
	req, _ = http.NewRequest("DELETE", ts.URL+"/v1/instances/"+instanceIDs[0], nil)
	addAuthForOrg(req, server, "test-org-delete")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to delete instance: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected 204 on delete, got %d", resp.StatusCode)
	}

	// Now we should be able to create another instance
	reqBody = bytes.NewBufferString(`{"tier": "small"}`)
	req, _ = http.NewRequest("POST", ts.URL+"/v1/instances", reqBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Org-ID", "test-org-delete")
	addAuthForOrg(req, server, "test-org-delete")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create instance after delete: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201 after delete, got %d", resp.StatusCode)
	}
}
