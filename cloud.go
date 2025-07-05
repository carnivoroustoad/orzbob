package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"orzbob/internal/tunnel"
)

var cloudCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Manage cloud-based runner instances",
	Long:  "Create, attach to, list, and manage remote cloud runner instances",
}

var cloudNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new cloud runner instance",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := loadToken()
		if err != nil {
			return fmt.Errorf("not logged in, run 'orz login' first")
		}

		// Fake response for now
		fmt.Printf("Creating new cloud instance...\n")
		fmt.Printf("Instance created: runner-abc123\n")
		fmt.Printf("Token: %s...\n", token[:10])
		return nil
	},
}

var cloudAttachCmd = &cobra.Command{
	Use:   "attach [instance-id-or-url]",
	Short: "Attach to a cloud runner instance",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// We might not need OAuth token if using JWT URL
		var wsURL string
		var jwtToken string
		
		if len(args) > 0 {
			arg := args[0]
			// Check if it's a full attach URL with JWT token
			if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
				// Parse the URL to extract the token
				parsedURL, err := url.Parse(arg)
				if err != nil {
					return fmt.Errorf("invalid URL: %w", err)
				}
				
				// Extract JWT token from query params
				jwtToken = parsedURL.Query().Get("token")
				if jwtToken == "" {
					return fmt.Errorf("no token found in attach URL")
				}
				
				// Convert HTTP(S) to WebSocket URL
				wsScheme := "ws"
				if parsedURL.Scheme == "https" {
					wsScheme = "wss"
				}
				parsedURL.Scheme = wsScheme
				parsedURL.RawQuery = "" // Remove query params, we'll add token separately
				wsURL = parsedURL.String()
			} else {
				// Treat as instance ID
				instanceID := arg
				wsURL = fmt.Sprintf("ws://localhost:8080/v1/instances/%s/attach", instanceID)
				// For direct instance ID, we'll need to get a JWT token somehow
				// For now, we'll pass empty token and let the server handle it
				jwtToken = ""
			}
		} else {
			return fmt.Errorf("instance ID or attach URL required")
		}

		fmt.Printf("Connecting to %s...\n", wsURL)
		
		return attachToInstance(wsURL, jwtToken)
	},
}

var cloudListCmd = &cobra.Command{
	Use:   "list",
	Short: "List cloud runner instances",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := loadToken()
		if err != nil {
			return fmt.Errorf("not logged in, run 'orz login' first")
		}

		// Fake response - hardcoded stub
		fmt.Printf("Cloud instances (token: %s...):\n", token[:10])
		fmt.Println("ID              STATUS    TIER     CREATED")
		fmt.Println("runner-abc123   running   small    2025-06-21T10:00:00Z")
		fmt.Println("runner-def456   stopped   medium   2025-06-21T09:00:00Z")
		return nil
	},
}

var cloudKillCmd = &cobra.Command{
	Use:   "kill [instance-id]",
	Short: "Terminate a cloud runner instance",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := loadToken()
		if err != nil {
			return fmt.Errorf("not logged in, run 'orz login' first")
		}

		instanceID := args[0]

		// Fake response
		fmt.Printf("Terminating instance %s...\n", instanceID)
		fmt.Printf("Token: %s...\n", token[:10])
		fmt.Printf("Instance %s terminated\n", instanceID)
		return nil
	},
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to cloud service via GitHub OAuth",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Fake OAuth flow
		fmt.Println("Opening browser for GitHub OAuth...")
		fmt.Println("Please authorize the application...")
		
		// Simulate delay
		time.Sleep(1 * time.Second)
		
		// Generate fake token
		token := "ghp_faketoken1234567890abcdef"
		
		// Save token
		if err := saveToken(token); err != nil {
			return fmt.Errorf("failed to save token: %w", err)
		}
		
		fmt.Println("Successfully logged in!")
		return nil
	},
}

type tokenData struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func saveToken(token string) error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "orzbob")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	data := tokenData{
		Token:     token,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour), // 30 days
	}

	file, err := os.Create(filepath.Join(configDir, "token.json"))
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewEncoder(file).Encode(data)
}

func loadToken() (string, error) {
	configPath := filepath.Join(os.Getenv("HOME"), ".config", "orzbob", "token.json")
	
	file, err := os.Open(configPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var data tokenData
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return "", err
	}

	if time.Now().After(data.ExpiresAt) {
		return "", fmt.Errorf("token expired")
	}

	return data.Token, nil
}

func init() {
	// Add cloud subcommands
	cloudCmd.AddCommand(cloudNewCmd)
	cloudCmd.AddCommand(cloudAttachCmd)
	cloudCmd.AddCommand(cloudListCmd)
	cloudCmd.AddCommand(cloudKillCmd)
}

// attachToInstance connects to a cloud instance via WebSocket
func attachToInstance(url string, token string) error {
	// Create WebSocket client with JWT token
	client, err := tunnel.NewClientWithToken(url, token)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	fmt.Println("Connected! Press Ctrl+C to disconnect.")
	fmt.Println()

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the client with stdin/stdout
	return client.Start(ctx, os.Stdin, os.Stdout, os.Stderr)
}