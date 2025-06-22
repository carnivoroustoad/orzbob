//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

const (
	baseURL = "http://localhost:8080"
)

type CreateInstanceResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	AttachURL string    `json:"attach_url"`
	CreatedAt time.Time `json:"created_at"`
}

func TestE2ECloudWorkflow(t *testing.T) {
	// Skip if not in CI or explicit e2e mode
	if os.Getenv("CI") == "" && os.Getenv("RUN_E2E") == "" {
		t.Skip("Skipping e2e tests (set CI or RUN_E2E env var to run)")
	}

	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/health")
		if err != nil {
			t.Fatalf("Failed to reach control plane: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("CreateInstance", func(t *testing.T) {
		reqBody := bytes.NewBufferString(`{"tier": "small"}`)
		req, _ := http.NewRequest("POST", baseURL+"/v1/instances", reqBody)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Org-ID", fmt.Sprintf("test-create-%d", time.Now().UnixNano()))
		
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected 201, got %d", resp.StatusCode)
		}

		var createResp CreateInstanceResponse
		if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if createResp.ID == "" {
			t.Error("Expected instance ID, got empty string")
		}

		if createResp.AttachURL == "" {
			t.Error("Expected attach URL, got empty string")
		}

		// Clean up - delete the instance
		delReq, _ := http.NewRequest("DELETE", baseURL+"/v1/instances/"+createResp.ID, nil)
		resp, err = http.DefaultClient.Do(delReq)
		if err != nil {
			t.Logf("Failed to delete instance: %v", err)
		}
		resp.Body.Close()
	})

	t.Run("ListInstances", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/v1/instances")
		if err != nil {
			t.Fatalf("Failed to list instances: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}

		var listResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if _, ok := listResp["instances"]; !ok {
			t.Error("Expected 'instances' field in response")
		}
	})

	t.Run("MetricsEndpoint", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/metrics")
		if err != nil {
			t.Fatalf("Failed to get metrics: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}

		// Read response body
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		body := buf.String()

		// Check for custom metrics
		expectedMetrics := []string{
			"orzbob_active_sessions",
			"orzbob_instances_created_total",
			"orzbob_instances_deleted_total",
		}

		for _, metric := range expectedMetrics {
			if !bytes.Contains([]byte(body), []byte(metric)) {
				t.Errorf("Expected metric %s not found", metric)
			}
		}
	})

	t.Run("QuotaEnforcement", func(t *testing.T) {
		// Create instances up to quota limit
		var instances []string
		
		// Create 3 instances (quota limit)
		for i := 0; i < 3; i++ {
			reqBody := bytes.NewBufferString(`{"tier": "small"}`)
			req, _ := http.NewRequest("POST", baseURL+"/v1/instances", reqBody)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Org-ID", "e2e-test-org")
			
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to create instance %d: %v", i+1, err)
			}
			
			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("Expected 201 for instance %d, got %d", i+1, resp.StatusCode)
			}
			
			var createResp CreateInstanceResponse
			json.NewDecoder(resp.Body).Decode(&createResp)
			instances = append(instances, createResp.ID)
			resp.Body.Close()
		}
		
		// Try to create fourth instance (should fail)
		reqBody := bytes.NewBufferString(`{"tier": "small"}`)
		thirdReq, _ := http.NewRequest("POST", baseURL+"/v1/instances", reqBody)
		thirdReq.Header.Set("Content-Type", "application/json")
		thirdReq.Header.Set("X-Org-ID", "e2e-test-org")
		
		resp, err := http.DefaultClient.Do(thirdReq)
		if err != nil {
			t.Fatalf("Failed to attempt third instance: %v", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusTooManyRequests {
			t.Errorf("Expected 429 for quota exceeded, got %d", resp.StatusCode)
		}
		
		// Clean up - delete instances
		for _, id := range instances {
			delReq, _ := http.NewRequest("DELETE", baseURL+"/v1/instances/"+id, nil)
			resp, _ := http.DefaultClient.Do(delReq)
			resp.Body.Close()
		}
	})
}

func TestE2ESecrets(t *testing.T) {
	// Skip if not in CI or explicit e2e mode
	if os.Getenv("CI") == "" && os.Getenv("RUN_E2E") == "" {
		t.Skip("Skipping e2e tests (set CI or RUN_E2E env var to run)")
	}

	t.Run("CreateAndListSecrets", func(t *testing.T) {
		// Create a secret
		secretData := map[string]interface{}{
			"name": "test-secret",
			"data": map[string]string{
				"API_KEY": "test-key-123",
				"DB_URL":  "postgres://test",
			},
		}
		
		reqBody, _ := json.Marshal(secretData)
		resp, err := http.Post(baseURL+"/v1/secrets", "application/json", bytes.NewBuffer(reqBody))
		if err != nil {
			t.Fatalf("Failed to create secret: %v", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected 201, got %d", resp.StatusCode)
		}
		
		// List secrets
		resp, err = http.Get(baseURL + "/v1/secrets")
		if err != nil {
			t.Fatalf("Failed to list secrets: %v", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
		
		// Clean up - delete secret
		delSecretReq, _ := http.NewRequest("DELETE", baseURL+"/v1/secrets/test-secret", nil)
		resp, _ = http.DefaultClient.Do(delSecretReq)
		resp.Body.Close()
	})
}