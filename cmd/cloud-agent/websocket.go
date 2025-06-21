package main

import (
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/gorilla/websocket"
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
	cmd.Process.Kill()
	cmd.Wait()

	log.Printf("WebSocket client disconnected")
}