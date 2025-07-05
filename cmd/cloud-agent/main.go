package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"orzbob/internal/cloud/config"
)

var (
	version = "0.1.0"
)

func main() {
	var (
		helpFlag    bool
		versionFlag bool
		sleepTime   int
	)

	flag.BoolVar(&helpFlag, "help", false, "Show help message")
	flag.BoolVar(&helpFlag, "h", false, "Show help message (shorthand)")
	flag.BoolVar(&versionFlag, "version", false, "Show version")
	flag.BoolVar(&versionFlag, "v", false, "Show version (shorthand)")
	flag.IntVar(&sleepTime, "sleep", 3600, "Sleep duration in seconds")
	flag.Parse()

	if helpFlag {
		fmt.Fprintf(os.Stdout, "Orzbob Cloud Agent %s\n\n", version)
		fmt.Fprintf(os.Stdout, "Usage: cloud-agent [OPTIONS]\n\n")
		fmt.Fprintf(os.Stdout, "Options:\n")
		flag.PrintDefaults()
		os.Exit(0)
	}

	if versionFlag {
		fmt.Fprintf(os.Stdout, "cloud-agent version %s\n", version)
		os.Exit(0)
	}

	log.Printf("Starting Orzbob Cloud Agent v%s", version)
	log.Printf("Process ID: %d", os.Getpid())
	
	// Bootstrap repository if configured
	if err := bootstrapRepository(); err != nil {
		log.Printf("Warning: Failed to bootstrap repository: %v", err)
		// Continue anyway - repo might already exist
	}

	// Load cloud config and run init script
	if err := runInitScript(); err != nil {
		log.Printf("Warning: Failed to run init script: %v", err)
		// Continue anyway
	}

	// Start tmux session with program
	if err := startTmuxSession(); err != nil {
		log.Fatalf("Failed to start tmux session: %v", err)
	}

	// Start WebSocket server for attachments
	if err := startWebSocketServer(); err != nil {
		log.Fatalf("Failed to start WebSocket server: %v", err)
	}

	// Start heartbeat sender
	go sendHeartbeats()

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Keep the agent running
	done := make(chan bool)

	// Log status every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Handle signals
	go func() {
		sig := <-sigCh
		log.Printf("Received signal: %v", sig)
		// Kill tmux session
		exec.Command("tmux", "kill-server").Run()
		done <- true
	}()

	for {
		select {
		case <-done:
			log.Printf("Agent shutting down")
			os.Exit(0)
		case <-ticker.C:
			log.Printf("Agent status check - still running")
			// Check if tmux is still running
			if err := exec.Command("tmux", "has-session", "-t", "orzbob").Run(); err != nil {
				log.Printf("Tmux session died, exiting")
				os.Exit(1)
			}
		}
	}
}

// bootstrapRepository clones the repository if REPO_URL is set
func bootstrapRepository() error {
	repoURL := os.Getenv("REPO_URL")
	if repoURL == "" {
		log.Printf("No REPO_URL set, skipping repository bootstrap")
		return nil
	}

	branch := os.Getenv("BRANCH")
	if branch == "" {
		branch = "main"
	}

	// Check if we're already in a git repository
	if _, err := os.Stat(".git"); err == nil {
		log.Printf("Repository already exists, pulling latest changes")
		
		// Fetch latest changes
		cmd := exec.Command("git", "fetch", "origin")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to fetch: %w", err)
		}

		// Checkout branch
		cmd = exec.Command("git", "checkout", branch)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to checkout branch %s: %w", branch, err)
		}

		// Pull latest changes
		cmd = exec.Command("git", "pull", "origin", branch)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to pull: %w", err)
		}

		log.Printf("Repository updated successfully")
		return nil
	}

	// Clone the repository
	log.Printf("Cloning repository %s (branch: %s)", repoURL, branch)
	
	cmd := exec.Command("git", "clone", "--branch", branch, repoURL, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	log.Printf("Repository cloned successfully")

	// List files for verification
	cmd = exec.Command("ls", "-la")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	return nil
}

// startTmuxSession starts a tmux session with the configured program
func startTmuxSession() error {
	// Get program from environment or use default
	program := os.Getenv("PROGRAM")
	if program == "" {
		program = "bash -c 'echo hi; sleep infinity'"
		log.Printf("No PROGRAM set, using placeholder: %s", program)
	} else {
		log.Printf("Starting program: %s", program)
	}

	// Create tmux session
	sessionName := "orzbob"
	
	// First, check if session already exists
	if err := exec.Command("tmux", "has-session", "-t", sessionName).Run(); err == nil {
		log.Printf("Tmux session '%s' already exists", sessionName)
		return nil
	}

	// Create new detached session with the program
	cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName, program)
	cmd.Env = os.Environ()
	
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create tmux session: %w\nOutput: %s", err, output)
	}

	log.Printf("Tmux session '%s' created successfully", sessionName)

	// List sessions for debugging
	if output, err := exec.Command("tmux", "list-sessions").CombinedOutput(); err == nil {
		log.Printf("Active tmux sessions:\n%s", output)
	}

	return nil
}

// sendHeartbeats sends heartbeats to the control plane every 20 seconds
func sendHeartbeats() {
	instanceID := os.Getenv("INSTANCE_ID")
	if instanceID == "" {
		log.Printf("No INSTANCE_ID set, heartbeats disabled")
		return
	}

	controlPlaneURL := os.Getenv("CONTROL_PLANE_URL")
	if controlPlaneURL == "" {
		controlPlaneURL = "http://localhost:8080"
	}

	heartbeatURL := fmt.Sprintf("%s/v1/instances/%s/heartbeat", controlPlaneURL, instanceID)
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	// Send initial heartbeat immediately
	sendHeartbeat(client, heartbeatURL)

	for range ticker.C {
		sendHeartbeat(client, heartbeatURL)
	}
}

// sendHeartbeat sends a single heartbeat request
func sendHeartbeat(client *http.Client, url string) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte("{}")))
	if err != nil {
		log.Printf("Failed to create heartbeat request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send heartbeat: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		log.Printf("Heartbeat failed with status: %d", resp.StatusCode)
	} else {
		log.Printf("Heartbeat sent successfully")
	}
}

// runInitScript loads cloud config and executes the init script if not already done
func runInitScript() error {
	workDir := "/workspace"
	initMarker := filepath.Join(workDir, ".orz", ".init_done")

	// Check if init has already been run
	if _, err := os.Stat(initMarker); err == nil {
		log.Printf("Init script already executed, skipping")
		return nil
	}

	// Load cloud config
	cfg, err := config.LoadCloudConfig(workDir)
	if err != nil {
		return fmt.Errorf("failed to load cloud config: %w", err)
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid cloud config: %w", err)
	}

	// Run init script if present
	if cfg.Setup.Init != "" {
		log.Printf("Running init script...")
		
		// Create script file
		scriptPath := "/tmp/init.sh"
		if err := os.WriteFile(scriptPath, []byte(cfg.Setup.Init), 0755); err != nil {
			return fmt.Errorf("failed to write init script: %w", err)
		}

		// Execute script
		cmd := exec.Command("/bin/bash", scriptPath)
		cmd.Dir = workDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()

		// Add cloud config env vars
		for k, v := range cfg.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("init script failed: %w", err)
		}

		log.Printf("Init script completed successfully")
	}

	// Create marker file
	if err := os.MkdirAll(filepath.Dir(initMarker), 0755); err != nil {
		return fmt.Errorf("failed to create .orz directory: %w", err)
	}
	if err := os.WriteFile(initMarker, []byte(time.Now().Format(time.RFC3339)), 0644); err != nil {
		return fmt.Errorf("failed to create init marker: %w", err)
	}

	return nil
}