package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
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
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health and metrics
		if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		
		// Skip auth for auth exchange endpoint
		if r.URL.Path == "/v1/auth/exchange" {
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
		claims, err := s.tokenManager.ValidateToken(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Invalid token")
			return
		}
		
		// Check token type
		if claims.Type != "user" {
			writeError(w, http.StatusUnauthorized, "Invalid token type")
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

// handleGetUser returns current user info
func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromContext(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user": user,
	})
}

func verifyGitHubToken(token string, expectedID int64) bool {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return false
	}
	
	req.Header.Set("Authorization", "Bearer " + token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	
	resp, err := client.Do(req)
	if err != nil {
		// Log error but don't print to stdout in production
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