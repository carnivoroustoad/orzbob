package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/gorilla/websocket"
	"orzbob/internal/cloud/config"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// startWebSocketServer starts a WebSocket server for tmux attachment
func startWebSocketServer() error {
	http.HandleFunc("/attach", handleWebSocketAttach)
	
	port := os.Getenv("WS_PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("Starting WebSocket server on port %s", port)
	go func() {
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Printf("WebSocket server error: %v", err)
		}
	}()

	return nil
}

// handleWebSocketAttach handles WebSocket connections for tmux attachment
func handleWebSocketAttach(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("WebSocket client connected from %s", r.RemoteAddr)

	// Run onAttach script if configured
	if err := runOnAttachScript(); err != nil {
		log.Printf("Warning: Failed to run onAttach script: %v", err)
		// Continue anyway
	}

	// Attach to tmux session
	cmd := exec.Command("tmux", "attach-session", "-t", "orzbob")
	
	// Create pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("Failed to create stdin pipe: %v", err)
		return
	}
	defer stdin.Close()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("Failed to create stdout pipe: %v", err)
		return
	}
	defer stdout.Close()

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("Failed to create stderr pipe: %v", err)
		return
	}
	defer stderr.Close()

	// Start tmux attach
	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start tmux attach: %v", err)
		return
	}

	// Handle I/O between WebSocket and tmux
	done := make(chan bool, 3)

	// WebSocket -> tmux stdin
	go func() {
		defer func() { done <- true }()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			if _, err := stdin.Write(data); err != nil {
				break
			}
		}
	}()

	// tmux stdout -> WebSocket
	go func() {
		defer func() { done <- true }()
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if err != nil {
				break
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				break
			}
		}
	}()

	// tmux stderr -> WebSocket
	go func() {
		defer func() { done <- true }()
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				break
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				break
			}
		}
	}()

	// Wait for any goroutine to finish
	<-done
	
	// Kill the tmux attach process
	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	log.Printf("WebSocket client disconnected")
}

// runOnAttachScript executes the onAttach script from cloud config
func runOnAttachScript() error {
	workDir := "/workspace"

	// Load cloud config
	cfg, err := config.LoadCloudConfig(workDir)
	if err != nil {
		return fmt.Errorf("failed to load cloud config: %w", err)
	}

	// Run onAttach script if present
	if cfg.Setup.OnAttach != "" {
		log.Printf("Running onAttach script...")
		
		// Create script file
		scriptPath := "/tmp/onattach.sh"
		if err := os.WriteFile(scriptPath, []byte(cfg.Setup.OnAttach), 0755); err != nil {
			return fmt.Errorf("failed to write onAttach script: %w", err)
		}

		// Execute script in tmux session
		cmd := exec.Command("tmux", "send-keys", "-t", "orzbob", 
			fmt.Sprintf("bash %s", scriptPath), "Enter")
		if err := cmd.Run(); err != nil {
			// Try running directly if tmux send fails
			cmd = exec.Command("/bin/bash", scriptPath)
			cmd.Dir = workDir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Env = os.Environ()

			// Add cloud config env vars
			for k, v := range cfg.Env {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
			}

			if err := cmd.Run(); err != nil {
				return fmt.Errorf("onAttach script failed: %w", err)
			}
		}

		log.Printf("OnAttach script completed")
	}

	return nil
}