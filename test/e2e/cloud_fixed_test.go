//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestE2EInstanceOperations tests core instance operations with proper isolation
func TestE2EInstanceOperations(t *testing.T) {
	if os.Getenv("CI") == "" && os.Getenv("RUN_E2E") == "" {
		t.Skip("Skipping e2e tests (set CI or RUN_E2E env var to run)")
	}

	t.Run("CreateAndDeleteInstance", func(t *testing.T) {
		// Use unique org ID to avoid quota conflicts
		orgID := fmt.Sprintf("test-create-%d", time.Now().UnixNano())
		
		// Create instance
		reqBody := bytes.NewBufferString(`{"tier": "small"}`)
		req, _ := http.NewRequest("POST", baseURL+"/v1/instances", reqBody)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Org-ID", orgID)
		
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}
		
		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Fatalf("Expected 201, got %d: %s", resp.StatusCode, body)
		}
		
		var createResp CreateInstanceResponse
		json.NewDecoder(resp.Body).Decode(&createResp)
		resp.Body.Close()
		
		// Verify response
		if createResp.ID == "" {
			t.Error("Instance ID is empty")
		}
		if createResp.AttachURL == "" {
			t.Error("AttachURL is empty")
		}
		if !strings.Contains(createResp.AttachURL, createResp.ID) {
			t.Error("AttachURL should contain instance ID")
		}
		if !strings.Contains(createResp.AttachURL, "token=") {
			t.Error("AttachURL should contain token")
		}
		
		// Get instance details
		resp, err = http.Get(baseURL + "/v1/instances/" + createResp.ID)
		if err != nil {
			t.Fatalf("Failed to get instance: %v", err)
		}
		
		var instance struct {
			ID        string `json:"id"`
			Status    string `json:"status"`
			PodName   string `json:"pod_name"`
			Namespace string `json:"namespace"`
		}
		json.NewDecoder(resp.Body).Decode(&instance)
		resp.Body.Close()
		
		// Wait for running state
		for i := 0; i < 30; i++ {
			resp, _ = http.Get(baseURL + "/v1/instances/" + createResp.ID)
			json.NewDecoder(resp.Body).Decode(&instance)
			resp.Body.Close()
			
			if instance.Status == "Running" {
				break
			}
			time.Sleep(time.Second)
		}
		
		if instance.Status == "Running" {
			// Verify pod exists and can execute commands
			cmd := exec.Command("kubectl", "exec", "-n", instance.Namespace, instance.PodName, 
				"--", "echo", "test")
			output, err := cmd.CombinedOutput()
			if err == nil && strings.Contains(string(output), "test") {
				t.Log("Pod is running and executable")
			}
		}
		
		// Delete instance
		req, _ = http.NewRequest("DELETE", baseURL+"/v1/instances/"+createResp.ID, nil)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Errorf("Failed to delete instance: %v", err)
		} else {
			if resp.StatusCode != http.StatusNoContent {
				t.Errorf("Expected 204 on delete, got %d", resp.StatusCode)
			}
			resp.Body.Close()
		}
		
		// Verify deletion - wait a moment for Kubernetes to process
		time.Sleep(2 * time.Second)
		resp, _ = http.Get(baseURL + "/v1/instances/" + createResp.ID)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404 for deleted instance, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("QuotaEnforcementPerOrg", func(t *testing.T) {
		orgID := fmt.Sprintf("test-quota-%d", time.Now().UnixNano())
		instances := []string{}
		
		// Create 3 instances (quota limit)
		for i := 0; i < 3; i++ {
			reqBody := bytes.NewBufferString(`{"tier": "small"}`)
			req, _ := http.NewRequest("POST", baseURL+"/v1/instances", reqBody)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Org-ID", orgID)
			
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to create instance %d: %v", i+1, err)
			}
			
			if resp.StatusCode != http.StatusCreated {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				t.Fatalf("Expected 201 for instance %d, got %d: %s", i+1, resp.StatusCode, body)
			}
			
			var createResp CreateInstanceResponse
			json.NewDecoder(resp.Body).Decode(&createResp)
			resp.Body.Close()
			instances = append(instances, createResp.ID)
		}
		
		// Try to create fourth instance (should fail)
		reqBody := bytes.NewBufferString(`{"tier": "small"}`)
		req, _ := http.NewRequest("POST", baseURL+"/v1/instances", reqBody)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Org-ID", orgID)
		
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to attempt third instance: %v", err)
		}
		
		if resp.StatusCode != http.StatusTooManyRequests {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected 429 for quota exceeded, got %d: %s", resp.StatusCode, body)
		}
		resp.Body.Close()
		
		// Cleanup
		for _, id := range instances {
			req, _ := http.NewRequest("DELETE", baseURL+"/v1/instances/"+id, nil)
			http.DefaultClient.Do(req)
		}
	})

	t.Run("InstanceWithSecrets", func(t *testing.T) {
		orgID := fmt.Sprintf("test-secrets-%d", time.Now().UnixNano())
		secretName := fmt.Sprintf("test-secret-%d", time.Now().UnixNano())
		
		// Create secret
		secretData := map[string]interface{}{
			"name": secretName,
			"data": map[string]string{
				"API_KEY": "test-key-123",
			},
		}
		
		reqBody, _ := json.Marshal(secretData)
		resp, err := http.Post(baseURL+"/v1/secrets", "application/json", bytes.NewBuffer(reqBody))
		if err != nil {
			t.Fatalf("Failed to create secret: %v", err)
		}
		
		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Fatalf("Failed to create secret: %d - %s", resp.StatusCode, body)
		}
		resp.Body.Close()
		
		// Create instance with secret
		instanceReq := map[string]interface{}{
			"tier":    "small",
			"secrets": []string{secretName},
		}
		
		reqBody, _ = json.Marshal(instanceReq)
		req, _ := http.NewRequest("POST", baseURL+"/v1/instances", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Org-ID", orgID)
		
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}
		
		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Fatalf("Failed to create instance: %d - %s", resp.StatusCode, body)
		}
		
		var createResp CreateInstanceResponse
		json.NewDecoder(resp.Body).Decode(&createResp)
		resp.Body.Close()
		
		// Verify instance has secrets
		resp, _ = http.Get(baseURL + "/v1/instances/" + createResp.ID)
		var instance struct {
			Secrets []string `json:"secrets"`
		}
		json.NewDecoder(resp.Body).Decode(&instance)
		resp.Body.Close()
		
		if len(instance.Secrets) != 1 || instance.Secrets[0] != secretName {
			t.Errorf("Expected secrets [%s], got %v", secretName, instance.Secrets)
		}
		
		// Cleanup
		req, _ = http.NewRequest("DELETE", baseURL+"/v1/instances/"+createResp.ID, nil)
		http.DefaultClient.Do(req)
		
		req, _ = http.NewRequest("DELETE", baseURL+"/v1/secrets/"+secretName, nil)
		http.DefaultClient.Do(req)
	})

	t.Run("InvalidRequests", func(t *testing.T) {
		// Use a unique org to avoid quota issues
		orgID := fmt.Sprintf("test-invalid-%d", time.Now().UnixNano())
		
		tests := []struct {
			name           string
			method         string
			path           string
			body           string
			headers        map[string]string
			expectedStatus int
		}{
			{
				name:           "InvalidTier",
				method:         "POST",
				path:           "/v1/instances",
				body:           `{"tier": "invalid"}`,
				headers:        map[string]string{"X-Org-ID": orgID},
				expectedStatus: http.StatusBadRequest,
			},
			{
				name:           "EmptyBody",
				method:         "POST",
				path:           "/v1/instances",
				body:           `{}`,
				headers:        map[string]string{"X-Org-ID": orgID},
				expectedStatus: http.StatusBadRequest,
			},
			{
				name:           "NonexistentInstance",
				method:         "GET",
				path:           "/v1/instances/nonexistent-123",
				expectedStatus: http.StatusNotFound,
			},
			{
				name:           "EmptySecretName",
				method:         "POST",
				path:           "/v1/secrets",
				body:           `{"name": "", "data": {"key": "value"}}`,
				expectedStatus: http.StatusBadRequest,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var req *http.Request
				if tt.body != "" {
					req, _ = http.NewRequest(tt.method, baseURL+tt.path, bytes.NewBufferString(tt.body))
					req.Header.Set("Content-Type", "application/json")
				} else {
					req, _ = http.NewRequest(tt.method, baseURL+tt.path, nil)
				}
				
				for k, v := range tt.headers {
					req.Header.Set(k, v)
				}
				
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatalf("Request failed: %v", err)
				}
				defer resp.Body.Close()
				
				if resp.StatusCode != tt.expectedStatus {
					body, _ := io.ReadAll(resp.Body)
					t.Errorf("Expected status %d, got %d. Body: %s", 
						tt.expectedStatus, resp.StatusCode, body)
				}
			})
		}
	})
}