package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cli/oauth"
	"github.com/spf13/cobra"
	"orzbob/internal/tunnel"
)

var cloudCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Manage cloud-based runner instances",
	Long:  "Create, attach to, list, and manage remote cloud runner instances",
}

var cloudNewCmd = &cobra.Command{
	Use:   "new [task]",
	Short: "Create a new cloud runner instance",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := loadToken()
		if err != nil {
			return fmt.Errorf("not logged in: %v\nRun 'orz login' first", err)
		}
		
		tier, _ := cmd.Flags().GetString("tier")
		if tier == "" {
			tier = "small"
		}
		
		fmt.Printf("🚀 Creating %s instance...\n", tier)
		
		// Call API
		apiURL := os.Getenv("ORZBOB_API_URL")
		if apiURL == "" {
			apiURL = "http://api.orzbob.com"
		}
		
		reqBody, _ := json.Marshal(map[string]string{
			"tier": tier,
		})
		
		req, err := http.NewRequest("POST", apiURL+"/v1/instances", bytes.NewReader(reqBody))
		if err != nil {
			return err
		}
		
		req.Header.Set("Authorization", "Bearer " + token)
		req.Header.Set("Content-Type", "application/json")
		
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("API request failed: %w", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusCreated {
			var errResp struct {
				Error string `json:"error"`
			}
			json.NewDecoder(resp.Body).Decode(&errResp)
			return fmt.Errorf("failed to create instance: %s", errResp.Error)
		}
		
		var instance struct {
			ID        string `json:"id"`
			Status    string `json:"status"`
			AttachURL string `json:"attach_url"`
		}
		
		if err := json.NewDecoder(resp.Body).Decode(&instance); err != nil {
			return err
		}
		
		fmt.Printf("✅ Instance created: %s\n", instance.ID)
		fmt.Printf("   Status: %s\n", instance.Status)
		fmt.Printf("\nAttaching to instance...\n")
		
		// Auto-attach
		return attachToInstance(instance.AttachURL)
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
				// Treat as instance ID - need to get attach URL from API
				instanceID := arg
				
				// Get instance details from API to get attach URL
				apiURL := os.Getenv("ORZBOB_API_URL")
				if apiURL == "" {
					apiURL = "http://api.orzbob.com"
				}
				
				token, err := loadToken()
				if err != nil {
					return fmt.Errorf("not logged in: %v\nRun 'orz login' first", err)
				}
				
				req, err := http.NewRequest("GET", apiURL+"/v1/instances/"+instanceID, nil)
				if err != nil {
					return err
				}
				req.Header.Set("Authorization", "Bearer " + token)
				
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					return fmt.Errorf("failed to get instance: %w", err)
				}
				defer resp.Body.Close()
				
				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("instance not found")
				}
				
				var instance struct {
					ID        string `json:"id"`
					AttachURL string `json:"attach_url"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&instance); err != nil {
					return fmt.Errorf("failed to decode response: %w", err)
				}
				
				if instance.AttachURL == "" {
					return fmt.Errorf("no attach URL available for instance")
				}
				
				// Use the attach URL from the API
				return attachToInstance(instance.AttachURL)
			}
		} else {
			return fmt.Errorf("instance ID or attach URL required")
		}

		fmt.Printf("Connecting to %s...\n", wsURL)
		
		// Reconstruct full URL with token
		fullURL := fmt.Sprintf("%s?token=%s", wsURL, url.QueryEscape(jwtToken))
		return attachToInstance(fullURL)
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

		// Call API to get instances
		apiURL := os.Getenv("ORZBOB_API_URL")
		if apiURL == "" {
			apiURL = "http://api.orzbob.com"
		}

		req, err := http.NewRequest("GET", apiURL+"/v1/instances", nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer " + token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to list instances: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			var errResp struct {
				Error string `json:"error"`
			}
			json.NewDecoder(resp.Body).Decode(&errResp)
			return fmt.Errorf("failed to list instances: %s", errResp.Error)
		}

		var instances []struct {
			ID        string    `json:"id"`
			Status    string    `json:"status"`
			Tier      string    `json:"tier"`
			CreatedAt time.Time `json:"created_at"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&instances); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		if len(instances) == 0 {
			fmt.Println("No cloud instances found")
			return nil
		}

		fmt.Println("ID                    STATUS    TIER     CREATED")
		for _, inst := range instances {
			fmt.Printf("%-20s  %-8s  %-7s  %s\n", 
				inst.ID, 
				inst.Status, 
				inst.Tier, 
				inst.CreatedAt.Format("2006-01-02 15:04:05"))
		}
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

		// Call API to terminate instance
		apiURL := os.Getenv("ORZBOB_API_URL")
		if apiURL == "" {
			apiURL = "http://api.orzbob.com"
		}

		req, err := http.NewRequest("DELETE", apiURL+"/v1/instances/"+instanceID, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer " + token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to terminate instance: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			var errResp struct {
				Error string `json:"error"`
			}
			json.NewDecoder(resp.Body).Decode(&errResp)
			return fmt.Errorf("failed to terminate instance: %s", errResp.Error)
		}

		fmt.Printf("✅ Instance %s terminated\n", instanceID)
		return nil
	},
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Orzbob Cloud using GitHub",
	Long: `Authenticate with Orzbob Cloud using GitHub OAuth device flow.
This will open your browser to enter a verification code.`,
	RunE: doLogin,
}

type GitHubUser struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type OrzbobUser struct {
	GitHubUser
	OrgID string `json:"org_id"`
	Plan  string `json:"plan"`
}

type savedTokens struct {
	GitHubToken string     `json:"github_token"`
	APIToken    string     `json:"api_token"`
	User        OrzbobUser `json:"user"`
	ExpiresAt   time.Time  `json:"expires_at"`
}

func doLogin(cmd *cobra.Command, args []string) error {
	clientID := os.Getenv("GITHUB_CLIENT_ID")
	if clientID == "" {
		// Use default Orzbob Cloud client ID
		clientID = "Ov23libNOCmmBZNvprW3"
	}
	
	fmt.Println("🔐 Starting GitHub authentication...")
	
	// Create OAuth flow
	flow := &oauth.Flow{
		Host:     oauth.GitHubHost("https://github.com"),
		ClientID: clientID,
		Scopes:   []string{"read:user", "user:email"},
	}
	
	// Start device flow
	accessToken, err := flow.DetectFlow()
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}
	
	// Verify token by getting user info
	fmt.Printf("✅ GitHub authentication successful!\n")
	fmt.Printf("   Getting user info...\n")
	user, err := getCurrentUser(accessToken.Token)
	if err != nil {
		return fmt.Errorf("failed to verify authentication: %w", err)
	}
	fmt.Printf("   GitHub user: %s\n", user.Login)
	
	// Exchange GitHub token for Orzbob API token
	fmt.Printf("   Exchanging token with API...\n")
	apiToken, err := exchangeToken(accessToken.Token, user)
	if err != nil {
		// For now, use the GitHub token as the API token (for testing)
		fmt.Printf("⚠️  Warning: API exchange failed, using fallback mode\n")
		fmt.Printf("   Error: %v\n", err)
		apiToken = accessToken.Token
	} else {
		fmt.Printf("   API token received!\n")
	}
	
	// Save both tokens
	if err := saveTokens(accessToken.Token, apiToken, user); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}
	
	fmt.Printf("✅ Successfully authenticated as %s (%s)\n", user.Login, user.Email)
	fmt.Printf("   Organization: %s\n", user.OrgID)
	fmt.Printf("   Plan: %s\n", user.Plan)
	
	return nil
}

func getCurrentUser(token string) (*OrzbobUser, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Authorization", "Bearer " + token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API error: %s", resp.Status)
	}
	
	var ghUser GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
		return nil, err
	}
	
	// Get primary email if not public
	if ghUser.Email == "" {
		ghUser.Email, _ = getPrimaryEmail(token)
	}
	
	return &OrzbobUser{
		GitHubUser: ghUser,
		OrgID:      fmt.Sprintf("gh-%d", ghUser.ID),
		Plan:       "free", // Will be set by API
	}, nil
}

func getPrimaryEmail(token string) (string, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}
	
	req.Header.Set("Authorization", "Bearer " + token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}
	
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	
	return "", fmt.Errorf("no primary email found")
}

func exchangeToken(githubToken string, user *OrzbobUser) (string, error) {
	// Call Orzbob API to exchange GitHub token for API token
	apiURL := os.Getenv("ORZBOB_API_URL")
	if apiURL == "" {
		apiURL = "http://api.orzbob.com" // Custom domain
	}
	
	reqBody, _ := json.Marshal(map[string]interface{}{
		"github_token": githubToken,
		"github_id":    user.ID,
		"github_login": user.Login,
		"email":        user.Email,
	})
	
	req, err := http.NewRequest("POST", apiURL+"/v1/auth/exchange", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return "", fmt.Errorf("API error: %s", errResp.Error)
	}
	
	var result struct {
		Token string `json:"token"`
		User  struct {
			OrgID string `json:"org_id"`
			Plan  string `json:"plan"`
		} `json:"user"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	
	// Update user info from API
	user.OrgID = result.User.OrgID
	user.Plan = result.User.Plan
	
	return result.Token, nil
}

func saveTokens(githubToken, apiToken string, user *OrzbobUser) error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "orzbob")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}
	
	data := savedTokens{
		GitHubToken: githubToken,
		APIToken:    apiToken,
		User:        *user,
		ExpiresAt:   time.Now().Add(90 * 24 * time.Hour), // 90 days
	}
	
	file, err := os.OpenFile(
		filepath.Join(configDir, "token.json"),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0600, // Read/write for owner only
	)
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
		return "", fmt.Errorf("not authenticated")
	}
	defer file.Close()

	var data savedTokens
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return "", err
	}

	if time.Now().After(data.ExpiresAt) {
		return "", fmt.Errorf("session expired, please run 'orz login' again")
	}

	return data.APIToken, nil
}

// logoutCmd logs out from Orzbob Cloud
var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out from Orzbob Cloud",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := filepath.Join(os.Getenv("HOME"), ".config", "orzbob", "token.json")
		
		if err := os.Remove(configPath); err != nil {
			if os.IsNotExist(err) {
				fmt.Println("Not logged in")
				return nil
			}
			return fmt.Errorf("failed to logout: %w", err)
		}
		
		fmt.Println("Successfully logged out")
		return nil
	},
}

// whoamiCmd shows current authenticated user
var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current authenticated user",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := filepath.Join(os.Getenv("HOME"), ".config", "orzbob", "token.json")
		
		file, err := os.Open(configPath)
		if err != nil {
			return fmt.Errorf("not logged in")
		}
		defer file.Close()
		
		var data savedTokens
		if err := json.NewDecoder(file).Decode(&data); err != nil {
			return err
		}
		
		if time.Now().After(data.ExpiresAt) {
			return fmt.Errorf("session expired")
		}
		
		fmt.Printf("Logged in as: %s (%s)\n", data.User.Login, data.User.Email)
		fmt.Printf("Organization: %s\n", data.User.OrgID)
		fmt.Printf("Plan: %s\n", data.User.Plan)
		fmt.Printf("Session expires: %s\n", data.ExpiresAt.Format("2006-01-02 15:04:05"))
		
		return nil
	},
}

func init() {
	// Add cloud subcommands
	cloudCmd.AddCommand(cloudNewCmd)
	cloudCmd.AddCommand(cloudAttachCmd)
	cloudCmd.AddCommand(cloudListCmd)
	cloudCmd.AddCommand(cloudKillCmd)
	cloudCmd.AddCommand(logoutCmd)
	cloudCmd.AddCommand(whoamiCmd)
	
	// Add flags
	cloudNewCmd.Flags().StringP("tier", "t", "small", "Instance tier (small, medium, large)")
}

// attachToInstance connects to a cloud instance via WebSocket
func attachToInstance(urlStr string) error {
	// Parse URL to extract token
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid attach URL: %w", err)
	}
	
	// Extract token from query params
	token := parsedURL.Query().Get("token")
	if token == "" {
		return fmt.Errorf("no token found in attach URL")
	}
	
	// Convert to WebSocket URL
	wsScheme := "ws"
	if parsedURL.Scheme == "https" {
		wsScheme = "wss"
	}
	parsedURL.Scheme = wsScheme
	parsedURL.RawQuery = "" // Remove query params from WebSocket URL
	
	wsURL := parsedURL.String()
	
	// Create WebSocket client with JWT token
	client, err := tunnel.NewClientWithToken(wsURL, token)
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