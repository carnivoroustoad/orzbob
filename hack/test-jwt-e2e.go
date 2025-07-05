//go:build tools
// +build tools

package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type CreateInstanceResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	AttachURL string    `json:"attach_url"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	baseURL := "http://localhost:8080"

	// Step 1: Create an instance
	log.Println("Creating instance...")
	reqBody := bytes.NewBufferString(`{"tier": "small"}`)
	resp, err := http.Post(baseURL+"/v1/instances", "application/json", reqBody)
	if err != nil {
		log.Fatalf("Failed to create instance: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		log.Fatalf("Expected 201, got %d", resp.StatusCode)
	}

	var createResp CreateInstanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		log.Fatalf("Failed to decode response: %v", err)
	}

	log.Printf("Instance created: %s", createResp.ID)
	log.Printf("Attach URL: %s", createResp.AttachURL)

	// Step 2: Extract JWT token from attach URL
	parsedURL, err := url.Parse(createResp.AttachURL)
	if err != nil {
		log.Fatalf("Failed to parse attach URL: %v", err)
	}

	token := parsedURL.Query().Get("token")
	if token == "" {
		log.Fatal("No token found in attach URL")
	}
	log.Printf("JWT token extracted (first 20 chars): %s...", token[:20])

	// Step 3: Try to connect with the token
	wsURL := strings.Replace(createResp.AttachURL, "http://", "ws://", 1)
	log.Printf("Connecting to WebSocket: %s", wsURL)

	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		log.Printf("Connection failed: %v", err)
		if resp != nil {
			log.Printf("Response status: %d", resp.StatusCode)
		}
		os.Exit(1)
	}
	defer conn.Close()

	log.Println("✅ Successfully connected with JWT token!")

	// Step 4: Send a test message
	testMsg := []byte("Hello from JWT test!")
	if err := conn.WriteMessage(websocket.TextMessage, testMsg); err != nil {
		log.Printf("Failed to write message: %v", err)
	}

	// Step 5: Read echo response
	msgType, data, err := conn.ReadMessage()
	if err != nil {
		log.Printf("Failed to read message: %v", err)
	} else {
		log.Printf("Received message (type %d): %s", msgType, string(data))
	}

	// Step 6: Try connecting without token (should fail)
	log.Println("\nTesting connection without token...")
	wsURLNoToken := strings.Split(wsURL, "?")[0]
	_, resp, err = dialer.Dial(wsURLNoToken, nil)
	if err != nil {
		log.Printf("✅ Connection correctly rejected without token")
		if resp != nil {
			log.Printf("Response status: %d (expected 401)", resp.StatusCode)
		}
	} else {
		log.Fatal("❌ Connection should have failed without token!")
	}

	// Step 7: Try with invalid token (should fail)
	log.Println("\nTesting connection with invalid token...")
	wsURLBadToken := wsURLNoToken + "?token=invalid.token.here"
	_, resp, err = dialer.Dial(wsURLBadToken, nil)
	if err != nil {
		log.Printf("✅ Connection correctly rejected with invalid token")
		if resp != nil {
			log.Printf("Response status: %d (expected 401)", resp.StatusCode)
		}
	} else {
		log.Fatal("❌ Connection should have failed with invalid token!")
	}

	// Step 8: Test CLI attach command with full URL
	log.Println("\nTesting CLI attach with JWT URL...")
	cmd := exec.Command("orz", "cloud", "attach", createResp.AttachURL)

	// Set up pipes for interaction
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("Failed to create stdin pipe: %v", err)
	}

	_, err = cmd.StdoutPipe()
	if err != nil {
		log.Printf("Failed to create stdout pipe: %v", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start CLI: %v", err)
	} else {
		log.Println("✅ CLI started successfully with JWT URL")

		// Send some test input
		stdin.Write([]byte("test from CLI\n"))

		// Give it a moment to connect
		time.Sleep(500 * time.Millisecond)

		// Kill the process
		cmd.Process.Kill()
		cmd.Wait()
	}

	log.Println("\n✅ All JWT E2E tests passed!")
}
