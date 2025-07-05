# GitHub Authentication Implementation Guide

## Overview

This document provides a complete implementation guide for adding GitHub OAuth authentication to Orzbob Cloud. The authentication system will replace the current stub implementation with a production-ready GitHub OAuth device flow for CLI authentication.

## Current State Analysis

### Existing Components
- **Stub login command** in `cloud.go` that saves fake tokens to `~/.config/orzbob/token.json`
- **Cloud commands** (new, attach, list, kill) that check for token presence
- **JWT infrastructure** in `internal/auth/jwt.go` for instance tokens
- **Basic token storage** structure already defined

### Missing Components
- Real GitHub OAuth flow implementation
- API authentication middleware
- User management system
- Token exchange endpoint
- Proper API integration in CLI commands

## Architecture Overview

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   CLI       │────▶│   GitHub    │────▶│  Orzbob API │
│  (orz)      │     │   OAuth     │     │ (cloud-cp)  │
└─────────────┘     └─────────────┘     └─────────────┘
      │                                         │
      │                                         │
      ▼                                         ▼
┌─────────────┐                        ┌─────────────┐
│Local Config │                        │User Storage │
│~/.config/   │                        │   (Simple)  │
└─────────────┘                        └─────────────┘
```

## Implementation Steps

### Step 1: Add GitHub OAuth Dependencies ✅

```bash
go get github.com/cli/oauth@latest
```

Update `go.mod` to include the OAuth library.

**Status**: ✅ Completed - Added `github.com/cli/oauth v1.2.0` to dependencies

### Step 2: Create GitHub OAuth App

1. Navigate to https://github.com/settings/developers
2. Click "New OAuth App"
3. Configure with:
   - **Application name**: Orzbob Cloud
   - **Homepage URL**: https://orzbob.cloud
   - **Authorization callback URL**: http://127.0.0.1:8899/callback
4. Note the **Client ID** (Client Secret not needed for device flow)

### Step 3: Environment Configuration ✅

Add to `.env.example`:
```env
# GitHub OAuth Configuration
GITHUB_CLIENT_ID=Ov23liABCDEF123456789
GITHUB_ORG_NAME=orzbob-cloud  # Optional: restrict to organization members

# Orzbob API Configuration  
ORZBOB_API_URL=http://54.224.5.131  # Production API URL
```

**Status**: ✅ Completed - Added GitHub OAuth and API configuration to `.env.example`

### Step 4: Implement Real GitHub Login ✅

Replace the stub `loginCmd` in `cloud.go`:

**Status**: ✅ Completed - Implemented full GitHub OAuth device flow with:
- Real GitHub authentication using `github.com/cli/oauth`
- Token exchange with Orzbob API
- Secure token storage
- Updated cloud commands to use API tokens
- Added `logout` and `whoami` commands

```go
package main

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "path/filepath"
    "time"
    
    "github.com/cli/oauth"
    "github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
    Use:   "login",
    Short: "Authenticate with Orzbob Cloud using GitHub",
    Long: `Authenticate with Orzbob Cloud using GitHub OAuth device flow.
This will open your browser to enter a verification code.`,
    RunE: doLogin,
}

func doLogin(cmd *cobra.Command, args []string) error {
    clientID := os.Getenv("GITHUB_CLIENT_ID")
    if clientID == "" {
        // Use default Orzbob Cloud client ID
        clientID = "Ov23liOrzbobCloudProd"
    }
    
    fmt.Println("🔐 Starting GitHub authentication...")
    
    // Create OAuth flow
    flow := &oauth.Flow{
        Host: oauth.GitHubHost("https://github.com"),
        ClientID: clientID,
        Scopes: []string{"read:user", "user:email"},
    }
    
    // Start device flow
    accessToken, err := flow.DetectFlow()
    if err != nil {
        return fmt.Errorf("authentication failed: %w", err)
    }
    
    // Verify token by getting user info
    user, err := getCurrentUser(accessToken.Token)
    if err != nil {
        return fmt.Errorf("failed to verify authentication: %w", err)
    }
    
    // Exchange GitHub token for Orzbob API token
    apiToken, err := exchangeToken(accessToken.Token, user)
    if err != nil {
        return fmt.Errorf("failed to create API session: %w", err)
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
```

### Step 5: User Management Types

Add user types and helper functions:

```go
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
```

### Step 6: Token Exchange Implementation

Add token exchange function:

```go
func exchangeToken(githubToken string, user *OrzbobUser) (string, error) {
    // Call Orzbob API to exchange GitHub token for API token
    apiURL := os.Getenv("ORZBOB_API_URL")
    if apiURL == "" {
        apiURL = "http://54.224.5.131" // Your deployed API
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
```

### Step 7: Secure Token Storage

Update token storage with proper security:

```go
type savedTokens struct {
    GitHubToken string    `json:"github_token"`
    APIToken    string    `json:"api_token"`
    User        OrzbobUser `json:"user"`
    ExpiresAt   time.Time  `json:"expires_at"`
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
```

### Step 8: Update Cloud Commands

Update `cloudNewCmd` to use real API:

```go
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
            apiURL = "http://54.224.5.131"
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
```

### Step 9: API Authentication Middleware ✅

Create `cmd/cloud-cp/auth.go`:

**Status**: ✅ Completed - Created auth.go with:
- Bearer token authentication middleware
- Token validation using JWT
- User context injection
- Auth exchange endpoint for GitHub token → API token
- User info endpoint

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"
    
    "github.com/go-chi/chi/v5"
    "orzbob/internal/auth"
)

type contextKey string

const (
    userContextKey contextKey = "user"
)

type User struct {
    ID       string    `json:"id"`
    GitHubID int64     `json:"github_id"`
    Login    string    `json:"login"`
    Email    string    `json:"email"`
    OrgID    string    `json:"org_id"`
    Plan     string    `json:"plan"`
    Created  time.Time `json:"created_at"`
}

// authMiddleware validates Bearer tokens and sets user context
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Skip auth for health and metrics
        if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
            next.ServeHTTP(w, r)
            return
        }
        
        // Extract Bearer token
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            writeError(w, http.StatusUnauthorized, "Missing authorization header")
            return
        }
        
        parts := strings.Split(authHeader, " ")
        if len(parts) != 2 || parts[0] != "Bearer" {
            writeError(w, http.StatusUnauthorized, "Invalid authorization format")
            return
        }
        
        token := parts[1]
        
        // Validate JWT token
        claims, err := tokenManager.ValidateToken(token)
        if err != nil {
            writeError(w, http.StatusUnauthorized, "Invalid token")
            return
        }
        
        // Load user from database/cache
        user, err := getUserByID(claims.UserID)
        if err != nil {
            writeError(w, http.StatusUnauthorized, "User not found")
            return
        }
        
        // Add user to context
        ctx := context.WithValue(r.Context(), userContextKey, user)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// getUserFromContext retrieves the authenticated user from request context
func getUserFromContext(r *http.Request) (*User, error) {
    user, ok := r.Context().Value(userContextKey).(*User)
    if !ok {
        return nil, fmt.Errorf("user not found in context")
    }
    return user, nil
}

// handleAuthExchange exchanges GitHub token for API token
func (s *Server) handleAuthExchange(w http.ResponseWriter, r *http.Request) {
    var req struct {
        GitHubToken string `json:"github_token"`
        GitHubID    int64  `json:"github_id"`
        GitHubLogin string `json:"github_login"`
        Email       string `json:"email"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "Invalid request")
        return
    }
    
    // Verify GitHub token
    if !verifyGitHubToken(req.GitHubToken, req.GitHubID) {
        writeError(w, http.StatusUnauthorized, "Invalid GitHub token")
        return
    }
    
    // Get or create user
    user, err := getOrCreateUser(req.GitHubID, req.GitHubLogin, req.Email)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "Failed to create user session")
        return
    }
    
    // Generate API token
    token, err := s.tokenManager.GenerateUserToken(user.ID, 90*24*time.Hour)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "Failed to generate token")
        return
    }
    
    // Return token and user info
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "token": token,
        "user": map[string]interface{}{
            "id":     user.ID,
            "login":  user.Login,
            "email":  user.Email,
            "org_id": user.OrgID,
            "plan":   user.Plan,
        },
    })
}

func verifyGitHubToken(token string, expectedID int64) bool {
    req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
    if err != nil {
        return false
    }
    
    req.Header.Set("Authorization", "Bearer " + token)
    req.Header.Set("Accept", "application/vnd.github.v3+json")
    
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return false
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return false
    }
    
    var user struct {
        ID int64 `json:"id"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
        return false
    }
    
    return user.ID == expectedID
}
```

### Step 10: Simple User Storage ✅

Create `cmd/cloud-cp/users.go`:

**Status**: ✅ Completed - Created users.go with:
- In-memory user storage with file persistence
- Thread-safe operations with mutex
- User CRUD operations
- Automatic user creation on first login

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "sync"
    "time"
)

var (
    userStore     = make(map[string]*User)
    userStoreMu   sync.RWMutex
    userStoreFile = "/tmp/orzbob-users.json"
)

func init() {
    // Load users from file on startup
    loadUserStore()
}

func loadUserStore() {
    file, err := os.Open(userStoreFile)
    if err != nil {
        return // File doesn't exist yet
    }
    defer file.Close()
    
    userStoreMu.Lock()
    defer userStoreMu.Unlock()
    
    json.NewDecoder(file).Decode(&userStore)
}

func saveUserStore() {
    userStoreMu.RLock()
    defer userStoreMu.RUnlock()
    
    file, err := os.Create(userStoreFile)
    if err != nil {
        return
    }
    defer file.Close()
    
    json.NewEncoder(file).Encode(userStore)
}

func getUserByID(id string) (*User, error) {
    userStoreMu.RLock()
    defer userStoreMu.RUnlock()
    
    user, ok := userStore[id]
    if !ok {
        return nil, fmt.Errorf("user not found")
    }
    
    return user, nil
}

func getOrCreateUser(githubID int64, login, email string) (*User, error) {
    userID := fmt.Sprintf("user-%d", githubID)
    
    userStoreMu.Lock()
    defer userStoreMu.Unlock()
    
    // Check if user exists
    if user, ok := userStore[userID]; ok {
        // Update login/email if changed
        user.Login = login
        user.Email = email
        saveUserStore()
        return user, nil
    }
    
    // Create new user
    user := &User{
        ID:       userID,
        GitHubID: githubID,
        Login:    login,
        Email:    email,
        OrgID:    fmt.Sprintf("gh-%d", githubID),
        Plan:     "free",
        Created:  time.Now(),
    }
    
    userStore[userID] = user
    saveUserStore()
    
    return user, nil
}
```

### Step 11: Update JWT Token Manager ✅

Update `internal/auth/jwt.go`:

**Status**: ✅ Completed - Added:
- `Type` field to Claims struct to distinguish between user and instance tokens
- `GenerateUserToken` method for creating API authentication tokens
- Updated `GenerateToken` to set type as "instance"

```go
// Add to Claims struct
type Claims struct {
    jwt.RegisteredClaims
    InstanceID string `json:"instance_id,omitempty"`
    UserID     string `json:"user_id,omitempty"`
    Tier       string `json:"tier,omitempty"`
    Type       string `json:"type"` // "instance" or "user"
}

// GenerateUserToken creates a JWT token for API authentication
func (tm *TokenManager) GenerateUserToken(userID string, duration time.Duration) (string, error) {
    now := time.Now()
    claims := Claims{
        RegisteredClaims: jwt.RegisteredClaims{
            Issuer:    tm.issuer,
            Subject:   userID,
            ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
            NotBefore: jwt.NewNumericDate(now),
            IssuedAt:  jwt.NewNumericDate(now),
            ID:        fmt.Sprintf("user-%d", now.UnixNano()),
        },
        UserID: userID,
        Type:   "user",
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
    return token.SignedString(tm.privateKey)
}
```

### Step 12: Update Main Server Routes ✅

Update `cmd/cloud-cp/main.go`:

**Status**: ✅ Completed - Updated:
- Added `/v1/auth/exchange` endpoint (no auth required)
- Applied auth middleware to all v1 routes
- Added `/v1/user` endpoint
- Updated `handleCreateInstance` to use authenticated user's OrgID
- Updated `handleGetBilling` to use authenticated user's OrgID

```go
func (s *Server) setupRoutes() {
    s.router.Use(middleware.Logger)
    s.router.Use(middleware.Recoverer)
    s.router.Use(middleware.RequestID)
    s.router.Use(middleware.RealIP)
    s.router.Use(middleware.Timeout(60 * time.Second))

    // Public endpoints
    s.router.Get("/health", s.handleHealth)
    s.router.Handle("/metrics", promhttp.Handler())
    
    // Auth endpoints (no auth required)
    s.router.Post("/v1/auth/exchange", s.handleAuthExchange)

    // API routes (auth required)
    s.router.Route("/v1", func(r chi.Router) {
        // Apply auth middleware
        r.Use(authMiddleware)
        
        // Instance management
        r.Post("/instances", s.handleCreateInstance)
        r.Get("/instances/{id}", s.handleGetInstance)
        r.Delete("/instances/{id}", s.handleDeleteInstance)
        r.Get("/instances", s.handleListInstances)
        r.Get("/instances/{id}/attach", s.handleWSAttach)
        r.Post("/instances/{id}/heartbeat", s.handleHeartbeat)
        
        // Secrets management  
        r.Post("/secrets", s.handleCreateSecret)
        r.Get("/secrets/{name}", s.handleGetSecret)
        r.Delete("/secrets/{name}", s.handleDeleteSecret)
        r.Get("/secrets", s.handleListSecrets)
        
        // Billing
        r.Get("/billing", s.handleGetBilling)
        
        // User info
        r.Get("/user", s.handleGetUser)
    })
}

// Update handleCreateInstance to use authenticated user
func (s *Server) handleCreateInstance(w http.ResponseWriter, r *http.Request) {
    user, err := getUserFromContext(r)
    if err != nil {
        writeError(w, http.StatusUnauthorized, "Authentication required")
        return
    }
    
    // Use user.OrgID instead of X-Org-ID header
    orgID := user.OrgID
    
    // Continue with existing logic...
}
```

### Step 13: Add Logout Command

Add to `cloud.go`:

```go
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

// Add whoami command
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

// Update init()
func init() {
    cloudCmd.AddCommand(cloudNewCmd)
    cloudCmd.AddCommand(cloudAttachCmd)
    cloudCmd.AddCommand(cloudListCmd)
    cloudCmd.AddCommand(cloudKillCmd)
    cloudCmd.AddCommand(cloudBillingCmd)
    cloudCmd.AddCommand(logoutCmd)
    cloudCmd.AddCommand(whoamiCmd)
    
    // Add flags
    cloudNewCmd.Flags().StringP("tier", "t", "small", "Instance tier (small, medium, large)")
}
```

## Testing Guide

### Local Development Testing ✅

**Status**: ✅ Basic testing completed:
- CLI builds successfully with auth commands
- API server builds and runs with auth middleware  
- Health endpoint accessible without auth
- Protected endpoints return 401 without Bearer token
- Auth exchange endpoint available at `/v1/auth/exchange`

1. **Set up environment**:
   ```bash
   export GITHUB_CLIENT_ID="your-client-id"
   export ORZBOB_API_URL="http://localhost:8080"
   ```

2. **Build and test CLI**:
   ```bash
   go build -o bin/orz .
   
   # Test login flow
   ./bin/orz login
   
   # Test authenticated commands
   ./bin/orz cloud whoami
   ./bin/orz cloud new
   ./bin/orz cloud list
   ./bin/orz cloud billing
   
   # Test logout
   ./bin/orz logout
   ```

3. **Test API server**:
   ```bash
   cd cmd/cloud-cp
   go run .
   
   # In another terminal, test endpoints
   curl http://localhost:8080/health
   curl -X POST http://localhost:8080/v1/auth/exchange -d '{"github_token":"...","github_id":123}'
   ```

### Production Deployment

1. **Build and push Docker image**:
   ```bash
   docker build -f docker/control-plane.Dockerfile -t orzbob-cloud-cp:auth .
   docker tag orzbob-cloud-cp:auth your-registry/orzbob-cloud-cp:auth
   docker push your-registry/orzbob-cloud-cp:auth
   ```

2. **Update Kubernetes deployment**:
   ```bash
   kubectl set image deployment/orzbob-cloud-orzbob-cp \
     orzbob-cp=your-registry/orzbob-cloud-cp:auth \
     -n orzbob-system
   
   kubectl rollout status deployment/orzbob-cloud-orzbob-cp -n orzbob-system
   ```

3. **Verify deployment**:
   ```bash
   # Check logs
   kubectl logs -f deployment/orzbob-cloud-orzbob-cp -n orzbob-system
   
   # Test auth endpoint
   curl -X POST http://your-api-url/v1/auth/exchange
   ```

## Security Considerations

### Token Security
- GitHub tokens are never stored by the API server
- API tokens are short-lived JWTs (90 days expiry)
- Local token file has 0600 permissions (owner read/write only)
- Tokens are stored in `~/.config/orzbob/token.json`

### API Security
- All API endpoints except `/health`, `/metrics`, and `/v1/auth/exchange` require authentication
- Bearer token format: `Authorization: Bearer <jwt-token>`
- Tokens are validated on every request
- User context is added to authenticated requests

### User Privacy
- Users are automatically created on first login
- Organization IDs are derived from GitHub user ID: `gh-<github-id>`
- Email addresses are only stored if provided by GitHub
- No passwords are stored (GitHub OAuth only)

## Troubleshooting

### Common Issues

1. **"Client ID not found" error**:
   - Ensure `GITHUB_CLIENT_ID` is set in environment
   - Check that the OAuth app is created in GitHub

2. **"Authentication failed" during login**:
   - Check internet connectivity
   - Ensure GitHub OAuth app is not suspended
   - Try clearing browser cookies for github.com

3. **"Token expired" errors**:
   - Run `orz logout` then `orz login` again
   - Check system time is correct

4. **API returns 401 Unauthorized**:
   - Ensure token is included in Authorization header
   - Check token hasn't expired (90 day expiry)
   - Verify API server has correct JWT keys

### Debug Mode

Enable debug logging:
```bash
export ORZBOB_DEBUG=true
orz login
```

Check API server logs:
```bash
kubectl logs -f deployment/orzbob-cloud-orzbob-cp -n orzbob-system
```

## Future Enhancements

1. **OAuth Scopes**:
   - Add organization membership verification
   - Request additional scopes for advanced features

2. **Token Refresh**:
   - Implement automatic token refresh before expiry
   - Add refresh token support

3. **Multi-Factor Authentication**:
   - Support GitHub's 2FA requirements
   - Add additional security layers

4. **User Management UI**:
   - Web dashboard for user profile management
   - API key generation for CI/CD

5. **Persistent User Storage**:
   - Replace file-based storage with database
   - Add user metadata and preferences

## Migration Guide

For users currently using the stub authentication:

1. **Clear old tokens**:
   ```bash
   rm -f ~/.config/orzbob/token.json
   ```

2. **Update CLI**:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/carnivoroustoad/orzbob/main/install.sh | bash
   ```

3. **Authenticate**:
   ```bash
   orz login
   ```

All existing cloud commands will continue to work with the new authentication system.