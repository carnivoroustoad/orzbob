//go:build tools
// +build tools

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"time"
)

func main() {
	ctx := context.Background()

	log.Println("Testing heartbeat functionality...")

	// Start control plane in background
	cpCmd := exec.CommandContext(ctx, "go", "run", "./cmd/cloud-cp", "-provider", "fake")
	if err := cpCmd.Start(); err != nil {
		log.Fatalf("Failed to start control plane: %v", err)
	}
	defer cpCmd.Process.Kill()

	// Wait for control plane to start
	time.Sleep(2 * time.Second)

	// Create an instance
	createReq := map[string]string{
		"tier": "small",
	}
	createBody, _ := json.Marshal(createReq)

	resp, err := http.Post("http://localhost:8080/v1/instances", "application/json", bytes.NewBuffer(createBody))
	if err != nil {
		log.Fatalf("Failed to create instance: %v", err)
	}
	defer resp.Body.Close()

	var createResp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		log.Fatalf("Failed to decode response: %v", err)
	}

	instanceID := createResp.ID
	log.Printf("Created instance: %s", instanceID)

	// Send heartbeats
	heartbeatURL := fmt.Sprintf("http://localhost:8080/v1/instances/%s/heartbeat", instanceID)

	for i := 0; i < 3; i++ {
		resp, err := http.Post(heartbeatURL, "application/json", bytes.NewBuffer([]byte("{}")))
		if err != nil {
			log.Printf("Failed to send heartbeat: %v", err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusNoContent {
			log.Printf("Heartbeat %d sent successfully", i+1)
		} else {
			log.Printf("Heartbeat %d failed with status: %d", i+1, resp.StatusCode)
		}

		time.Sleep(5 * time.Second)
	}

	// Verify instance still exists
	getResp, err := http.Get(fmt.Sprintf("http://localhost:8080/v1/instances/%s", instanceID))
	if err != nil {
		log.Fatalf("Failed to get instance: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode == http.StatusOK {
		log.Println("✓ Instance still exists after heartbeats")
	} else {
		log.Fatalf("✗ Instance not found after heartbeats")
	}

	// Delete instance
	deleteReq, _ := http.NewRequest("DELETE", fmt.Sprintf("http://localhost:8080/v1/instances/%s", instanceID), nil)
	deleteResp, err := http.DefaultClient.Do(deleteReq)
	if err != nil {
		log.Fatalf("Failed to delete instance: %v", err)
	}
	deleteResp.Body.Close()

	log.Println("✓ Heartbeat test completed successfully")
}
