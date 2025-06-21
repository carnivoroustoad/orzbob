package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"orzbob/internal/auth"
	"orzbob/internal/cloud/provider"
	"orzbob/internal/metrics"
	"orzbob/internal/tunnel"
)

var (
	version = "0.1.0"
)

// API types
type CreateInstanceRequest struct {
	Tier    string   `json:"tier,omitempty"`
	Program string   `json:"program,omitempty"`
	RepoURL string   `json:"repo_url,omitempty"`
	Branch  string   `json:"branch,omitempty"`
	Secrets []string `json:"secrets,omitempty"`
}

type CreateInstanceResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	AttachURL string    `json:"attach_url"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateSecretRequest struct {
	Name string            `json:"name"`
	Data map[string]string `json:"data"`
}

type SecretResponse struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	CreatedAt time.Time `json:"created_at"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// Server represents the control plane server
type Server struct {
	provider     provider.Provider
	router       chi.Router
	wsProxy      *tunnel.WSProxy
	heartbeats   map[string]time.Time
	heartbeatMu  sync.RWMutex
	tokenManager *auth.TokenManager
	baseURL      string
	
	// Quota tracking: orgID -> instance count
	instanceCounts map[string]int
	quotaMu        sync.RWMutex
	freeQuota      int // Max instances for free tier
}

// NewServer creates a new control plane server
func NewServer(p provider.Provider) *Server {
	// Create token manager
	tokenManager, err := auth.NewTokenManager("orzbob-cloud")
	if err != nil {
		log.Fatalf("Failed to create token manager: %v", err)
	}

	s := &Server{
		provider:       p,
		router:         chi.NewRouter(),
		wsProxy:        tunnel.NewWSProxy(),
		heartbeats:     make(map[string]time.Time),
		tokenManager:   tokenManager,
		baseURL:        "http://localhost:8080", // Default, can be overridden
		instanceCounts: make(map[string]int),
		freeQuota:      2, // Free tier allows 2 instances
	}
	s.setupRoutes()
	
	// Start idle reaper
	go s.startIdleReaper()
	
	return s
}

// handleCreateSecret creates a new Kubernetes secret
func (s *Server) handleCreateSecret(w http.ResponseWriter, r *http.Request) {
	var req CreateSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Secret name is required")
		return
	}
	if len(req.Data) == 0 {
		writeError(w, http.StatusBadRequest, "Secret data is required")
		return
	}

	// Create secret in provider
	secret, err := s.provider.CreateSecret(r.Context(), req.Name, req.Data)
	if err != nil {
		log.Printf("Failed to create secret: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create secret")
		return
	}

	// Return response
	resp := SecretResponse{
		Name:      secret.Name,
		Namespace: secret.Namespace,
		CreatedAt: secret.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// handleGetSecret retrieves a secret
func (s *Server) handleGetSecret(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	
	secret, err := s.provider.GetSecret(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "Secret not found")
		return
	}

	resp := SecretResponse{
		Name:      secret.Name,
		Namespace: secret.Namespace,
		CreatedAt: secret.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleDeleteSecret deletes a secret
func (s *Server) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	
	if err := s.provider.DeleteSecret(r.Context(), name); err != nil {
		writeError(w, http.StatusNotFound, "Secret not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleListSecrets lists all secrets
func (s *Server) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	secrets, err := s.provider.ListSecrets(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list secrets")
		return
	}

	var resp []SecretResponse
	for _, secret := range secrets {
		resp = append(resp, SecretResponse{
			Name:      secret.Name,
			Namespace: secret.Namespace,
			CreatedAt: secret.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"secrets": resp,
	})
}

// setupRoutes configures the HTTP routes
func (s *Server) setupRoutes() {
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Timeout(60 * time.Second))

	// Health check
	s.router.Get("/health", s.handleHealth)
	
	// Metrics endpoint
	s.router.Handle("/metrics", promhttp.Handler())

	// API routes
	s.router.Route("/v1", func(r chi.Router) {
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
	})
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"version": version,
	})
}

// handleCreateInstance handles instance creation requests
func (s *Server) handleCreateInstance(w http.ResponseWriter, r *http.Request) {
	var req CreateInstanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err.Error() != "EOF" { // Allow empty body
			writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
	}

	// Extract organization ID from context (would come from auth middleware in production)
	// For now, use a default org for testing
	orgID := r.Header.Get("X-Org-ID")
	if orgID == "" {
		orgID = "default-org"
	}

	// Check quota
	s.quotaMu.Lock()
	currentCount := s.instanceCounts[orgID]
	if currentCount >= s.freeQuota {
		s.quotaMu.Unlock()
		metrics.QuotaExceeded.WithLabelValues(orgID).Inc()
		writeError(w, http.StatusTooManyRequests, fmt.Sprintf("Quota exceeded: maximum %d instances allowed for free tier", s.freeQuota))
		return
	}
	// Increment count optimistically
	s.instanceCounts[orgID] = currentCount + 1
	s.quotaMu.Unlock()

	// Default tier if not specified
	tier := req.Tier
	if tier == "" {
		tier = "small"
	}

	// Create instance using provider
	instance, err := s.provider.CreateInstanceWithSecrets(r.Context(), tier, req.Secrets)
	if err != nil {
		// Rollback quota increment on failure
		s.quotaMu.Lock()
		s.instanceCounts[orgID]--
		s.quotaMu.Unlock()
		
		log.Printf("Failed to create instance: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create instance")
		return
	}
	
	// Store org ID in instance metadata for deletion tracking
	instance.Labels["org-id"] = orgID
	
	// Increment metrics
	metrics.InstancesCreated.Inc()

	// Generate JWT token for attachment (valid for 2 minutes)
	token, err := s.tokenManager.GenerateToken(instance.ID, 2*time.Minute)
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to generate access token")
		return
	}

	// Build attach URL with token
	baseURL := s.baseURL
	if baseURL == "" {
		baseURL = "ws://localhost:8080"
	}
	attachURL := fmt.Sprintf("%s/v1/instances/%s/attach?token=%s", 
		baseURL, instance.ID, url.QueryEscape(token))

	// Return response
	resp := CreateInstanceResponse{
		ID:        instance.ID,
		Status:    instance.Status,
		AttachURL: attachURL,
		CreatedAt: instance.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// handleGetInstance handles get instance requests
func (s *Server) handleGetInstance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	
	instance, err := s.provider.GetInstance(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Instance not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(instance)
}

// handleDeleteInstance handles delete instance requests
func (s *Server) handleDeleteInstance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	
	// Get instance to find org ID
	instance, err := s.provider.GetInstance(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Instance not found")
		return
	}
	
	// Delete the instance
	if err := s.provider.DeleteInstance(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete instance")
		return
	}
	
	// Decrement quota count
	if orgID, ok := instance.Labels["org-id"]; ok && orgID != "" {
		s.quotaMu.Lock()
		if count, exists := s.instanceCounts[orgID]; exists && count > 0 {
			s.instanceCounts[orgID]--
		}
		s.quotaMu.Unlock()
	}
	
	// Increment metrics
	metrics.InstancesDeleted.Inc()

	w.WriteHeader(http.StatusNoContent)
}

// handleListInstances handles list instances requests
func (s *Server) handleListInstances(w http.ResponseWriter, r *http.Request) {
	instances, err := s.provider.ListInstances(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list instances")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"instances": instances,
	})
}

// handleWSAttach handles WebSocket attach requests
func (s *Server) handleWSAttach(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	
	// Get token from query parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusUnauthorized, "Missing authentication token")
		return
	}

	// Validate token
	claims, err := s.tokenManager.ValidateToken(token)
	if err != nil {
		log.Printf("Invalid token for instance %s: %v", id, err)
		writeError(w, http.StatusUnauthorized, "Invalid or expired token")
		return
	}

	// Verify token is for the requested instance
	if claims.InstanceID != id {
		log.Printf("Token instance mismatch: token for %s, requested %s", claims.InstanceID, id)
		writeError(w, http.StatusForbidden, "Token not valid for this instance")
		return
	}
	
	// Verify instance exists
	_, err = s.provider.GetInstance(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Instance not found")
		return
	}

	// Handle WebSocket connection
	s.wsProxy.HandleAttach(id)(w, r)
}

// handleHeartbeat handles heartbeat requests from instances
func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	
	// Verify instance exists
	_, err := s.provider.GetInstance(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Instance not found")
		return
	}

	// Update heartbeat timestamp
	s.heartbeatMu.Lock()
	s.heartbeats[id] = time.Now()
	s.heartbeatMu.Unlock()
	
	// Increment metrics
	metrics.HeartbeatsReceived.Inc()

	w.WriteHeader(http.StatusNoContent)
}

// startIdleReaper periodically checks for idle instances and deletes them
func (s *Server) startIdleReaper() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.reapIdleInstances()
	}
}

// reapIdleInstances deletes instances that haven't sent a heartbeat in over 30 minutes
func (s *Server) reapIdleInstances() {
	ctx := context.Background()
	idleTimeout := 30 * time.Minute
	now := time.Now()

	// Get all instances
	instances, err := s.provider.ListInstances(ctx)
	if err != nil {
		log.Printf("Failed to list instances for reaping: %v", err)
		return
	}

	// Collect instances to delete to avoid holding lock during deletion
	toDelete := []provider.Instance{}
	
	s.heartbeatMu.RLock()
	for _, instance := range instances {
		lastHeartbeat, exists := s.heartbeats[instance.ID]
		
		// If no heartbeat recorded, use creation time
		if !exists {
			lastHeartbeat = instance.CreatedAt
		}

		// Check if idle
		if now.Sub(lastHeartbeat) > idleTimeout {
			toDelete = append(toDelete, *instance)
		}
	}
	s.heartbeatMu.RUnlock()
	
	// Delete idle instances
	for _, instance := range toDelete {
		log.Printf("Reaping idle instance %s", instance.ID)
		
		// Delete the instance
		if err := s.provider.DeleteInstance(ctx, instance.ID); err != nil {
			log.Printf("Failed to delete idle instance %s: %v", instance.ID, err)
		} else {
			// Remove from heartbeat map
			s.heartbeatMu.Lock()
			delete(s.heartbeats, instance.ID)
			s.heartbeatMu.Unlock()
			
			// Decrement quota count
			if orgID, ok := instance.Labels["org-id"]; ok && orgID != "" {
				s.quotaMu.Lock()
				if count, exists := s.instanceCounts[orgID]; exists && count > 0 {
					s.instanceCounts[orgID]--
				}
				s.quotaMu.Unlock()
			}
			
			// Increment metrics
			metrics.IdleInstancesReaped.Inc()
			metrics.InstancesDeleted.Inc()
		}
	}
}

// writeError writes an error response
func writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

func main() {
	var (
		port         int
		providerType string
		kubeconfig   string
	)

	flag.IntVar(&port, "port", 8080, "HTTP server port")
	flag.StringVar(&providerType, "provider", "fake", "Provider type (fake, kind)")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig (for kind provider)")
	flag.Parse()

	log.Printf("Starting Orzbob Cloud Control Plane v%s", version)
	
	// Log environment variables for debugging
	runnerImage := os.Getenv("RUNNER_IMAGE")
	if runnerImage != "" {
		log.Printf("RUNNER_IMAGE environment variable: %s", runnerImage)
	} else {
		log.Println("RUNNER_IMAGE environment variable not set")
	}

	// Create provider
	var p provider.Provider
	var err error

	switch providerType {
	case "kind":
		p, err = provider.NewLocalKind(kubeconfig)
		if err != nil {
			log.Fatalf("Failed to create kind provider: %v", err)
		}
		if kubeconfig == "" {
			log.Println("Using LocalKind provider with in-cluster config")
		} else {
			log.Printf("Using LocalKind provider with kubeconfig: %s", kubeconfig)
		}
	case "fake":
		p = provider.NewFakeProvider()
		log.Println("Using fake provider")
	default:
		log.Fatalf("Unknown provider type: %s", providerType)
	}

	// Create server
	server := NewServer(p)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: server.router,
	}

	// Start server
	go func() {
		log.Printf("HTTP server listening on :%d", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	
	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Shutdown complete")
}