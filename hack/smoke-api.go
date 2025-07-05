//go:build tools
// +build tools

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"time"
)

const baseURL = "http://localhost:8080"

type CreateInstanceResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	AttachURL string    `json:"attach_url"`
	CreatedAt time.Time `json:"created_at"`
}

type Instance struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Tier      string    `json:"tier"`
	CreatedAt time.Time `json:"created_at"`
	PodName   string    `json:"pod_name"`
	Namespace string    `json:"namespace"`
}

func main() {
	// Check health
	fmt.Println("Checking control plane health...")
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		log.Fatalf("Failed to reach control plane: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Control plane unhealthy: %d", resp.StatusCode)
	}
	fmt.Println("Control plane is healthy!")

	// Create instance via API
	fmt.Println("Creating instance via API...")
	reqBody := bytes.NewBufferString(`{"tier": "small"}`)
	resp, err = http.Post(baseURL+"/v1/instances", "application/json", reqBody)
	if err != nil {
		log.Fatalf("Failed to create instance: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		log.Fatalf("Failed to create instance: %d - %s", resp.StatusCode, body.String())
	}

	var createResp CreateInstanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		log.Fatalf("Failed to decode response: %v", err)
	}

	fmt.Printf("Created instance: %s\n", createResp.ID)
	fmt.Printf("Attach URL: %s\n", createResp.AttachURL)

	// Poll instance status
	fmt.Println("Waiting for pod to be running...")
	var instance Instance
	for i := 0; i < 60; i++ {
		resp, err := http.Get(baseURL + "/v1/instances/" + createResp.ID)
		if err != nil {
			log.Fatalf("Failed to get instance: %v", err)
		}

		if err := json.NewDecoder(resp.Body).Decode(&instance); err != nil {
			resp.Body.Close()
			log.Fatalf("Failed to decode instance: %v", err)
		}
		resp.Body.Close()

		if instance.Status == "Running" {
			fmt.Println("Pod is running!")
			break
		}

		fmt.Printf("Pod status: %s, waiting...\n", instance.Status)

		// If pending for too long, check pod events
		if i > 5 && instance.Status == "Pending" {
			cmd := exec.Command("kubectl", "describe", "pod", instance.PodName, "-n", instance.Namespace)
			output, _ := cmd.CombinedOutput()
			if i == 6 { // Only print once
				fmt.Printf("Pod describe output:\n%s\n", output)
			}
		}

		time.Sleep(2 * time.Second)
	}

	if instance.Status != "Running" {
		log.Fatalf("Pod did not reach Running status")
	}

	// Check pod details
	fmt.Printf("\nPod details:\n")
	fmt.Printf("  Name: %s\n", instance.PodName)
	fmt.Printf("  Namespace: %s\n", instance.Namespace)

	// Check the image used
	fmt.Println("\nChecking pod image...")
	cmd := exec.Command("kubectl", "get", "pod", instance.PodName, "-n", instance.Namespace,
		"-o", "jsonpath={.spec.containers[0].image}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to get pod image: %v", err)
	}
	fmt.Printf("Pod is using image: %s\n", string(output))

	// Delete instance
	fmt.Println("\nDeleting instance...")
	req, _ := http.NewRequest("DELETE", baseURL+"/v1/instances/"+createResp.ID, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Warning: Failed to delete instance: %v", err)
	} else {
		resp.Body.Close()
		if resp.StatusCode == http.StatusNoContent {
			fmt.Println("Instance deleted successfully")
		}
	}

	// Check final image
	expectedImage := "orzbob/cloud-agent:e2e"
	actualImage := string(output)
	if actualImage != expectedImage {
		log.Fatalf("❌ Wrong image! Expected %s, got %s", expectedImage, actualImage)
	} else {
		fmt.Printf("✅ Correct image used: %s\n", actualImage)
	}
}
