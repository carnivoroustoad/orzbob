package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"orzbob/internal/cloud/provider"
)

func TestSecretEndpoints(t *testing.T) {
	// Create test server
	fakeProvider := provider.NewFakeProvider()
	server := NewServer(fakeProvider)
	
	// Test 1: Create a secret
	t.Run("CreateSecret", func(t *testing.T) {
		secretData := CreateSecretRequest{
			Name: "test-secret",
			Data: map[string]string{
				"DATABASE_URL": "postgres://user:pass@localhost/db",
				"API_KEY":      "secret-key-123",
			},
		}
		
		body, _ := json.Marshal(secretData)
		req := httptest.NewRequest("POST", "/v1/secrets", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		addTestAuth(req, server)
		rr := httptest.NewRecorder()
		
		server.router.ServeHTTP(rr, req)
		
		if rr.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", rr.Code)
		}
		
		var resp SecretResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		
		if resp.Name != "test-secret" {
			t.Errorf("Expected name 'test-secret', got %s", resp.Name)
		}
	})
	
	// Test 2: Get a secret
	t.Run("GetSecret", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/secrets/test-secret", nil)
		addTestAuth(req, server)
		rr := httptest.NewRecorder()
		
		server.router.ServeHTTP(rr, req)
		
		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
		
		var resp SecretResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		
		if resp.Name != "test-secret" {
			t.Errorf("Expected name 'test-secret', got %s", resp.Name)
		}
	})
	
	// Test 3: List secrets
	t.Run("ListSecrets", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/secrets", nil)
		addTestAuth(req, server)
		rr := httptest.NewRecorder()
		
		server.router.ServeHTTP(rr, req)
		
		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
		
		var resp map[string][]SecretResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		
		if len(resp["secrets"]) != 1 {
			t.Errorf("Expected 1 secret, got %d", len(resp["secrets"]))
		}
	})
	
	// Test 4: Create instance with secrets
	t.Run("CreateInstanceWithSecrets", func(t *testing.T) {
		instanceData := CreateInstanceRequest{
			Tier:    "small",
			Secrets: []string{"test-secret"},
		}
		
		body, _ := json.Marshal(instanceData)
		req := httptest.NewRequest("POST", "/v1/instances", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		addTestAuth(req, server)
		rr := httptest.NewRecorder()
		
		server.router.ServeHTTP(rr, req)
		
		if rr.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", rr.Code)
		}
		
		var resp CreateInstanceResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		
		// Verify instance was created
		if resp.ID == "" {
			t.Error("Expected instance ID to be set")
		}
		
		// Get instance details
		instance, err := fakeProvider.GetInstance(context.Background(), resp.ID)
		if err != nil {
			t.Fatalf("Failed to get instance: %v", err)
		}
		
		// Verify secrets are attached
		if len(instance.Secrets) != 1 || instance.Secrets[0] != "test-secret" {
			t.Errorf("Expected instance to have secret 'test-secret', got %v", instance.Secrets)
		}
	})
	
	// Test 5: Delete secret
	t.Run("DeleteSecret", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/v1/secrets/test-secret", nil)
		addTestAuth(req, server)
		rr := httptest.NewRecorder()
		
		server.router.ServeHTTP(rr, req)
		
		if rr.Code != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", rr.Code)
		}
		
		// Verify secret is deleted
		req = httptest.NewRequest("GET", "/v1/secrets/test-secret", nil)
		addTestAuth(req, server)
		rr = httptest.NewRecorder()
		server.router.ServeHTTP(rr, req)
		
		if rr.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for deleted secret, got %d", rr.Code)
		}
	})
}

func TestSecretValidation(t *testing.T) {
	fakeProvider := provider.NewFakeProvider()
	server := NewServer(fakeProvider)
	
	tests := []struct {
		name       string
		request    CreateSecretRequest
		wantStatus int
	}{
		{
			name: "missing name",
			request: CreateSecretRequest{
				Name: "",
				Data: map[string]string{"key": "value"},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing data",
			request: CreateSecretRequest{
				Name: "test",
				Data: map[string]string{},
			},
			wantStatus: http.StatusBadRequest,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/v1/secrets", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			addTestAuth(req, server)
			rr := httptest.NewRecorder()
			
			server.router.ServeHTTP(rr, req)
			
			if rr.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, rr.Code)
			}
		})
	}
}