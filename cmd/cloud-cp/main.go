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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"orzbob/internal/auth"
	"orzbob/internal/billing"
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

	// Billing
	meteringService *billing.MeteringService

	// Instance tracking for usage calculation
	instanceStarts map[string]time.Time
	startsMu       sync.RWMutex
}

// NewServer creates a new control plane server
func NewServer(p provider.Provider) *Server {
	// Create token manager
	tokenManager, err := auth.NewTokenManager("orzbob-cloud")
	if err != nil {
		log.Fatalf("Failed to create token manager: %v", err)
	}

	// Get base URL from environment
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	s := &Server{
		provider:       p,
		router:         chi.NewRouter(),
		wsProxy:        tunnel.NewWSProxy(),
		heartbeats:     make(map[string]time.Time),
		tokenManager:   tokenManager,
		baseURL:        baseURL,
		instanceCounts: make(map[string]int),
		freeQuota:      3, // Free tier allows 3 instances for testing
		instanceStarts: make(map[string]time.Time),
	}

	// Initialize billing if configured
	billingConfig := billing.LoadConfigOptional()
	if billingConfig.IsConfigured() {
		meteringService, err := billing.NewMeteringService(billingConfig)
		if err != nil {
			log.Printf("Failed to create metering service: %v", err)
		} else {
			s.meteringService = meteringService
			s.meteringService.Start(context.Background())
			log.Println("Billing metering service started")
		}
	} else {
		log.Println("Billing not configured, running without metering")
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
	_ = json.NewEncoder(w).Encode(resp)
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
	_ = json.NewEncoder(w).Encode(resp)
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
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
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

	// Auth endpoints (no auth required)
	s.router.Post("/v1/auth/exchange", s.handleAuthExchange)

	// WebSocket endpoints (token auth handled internally)
	s.router.Get("/v1/instances/{id}/attach", s.handleWSAttach)

	// API routes (auth required)
	s.router.Route("/v1", func(r chi.Router) {
		// Apply auth middleware
		r.Use(s.authMiddleware)

		// Instance management
		r.Post("/instances", s.handleCreateInstance)
		r.Get("/instances/{id}", s.handleGetInstance)
		r.Delete("/instances/{id}", s.handleDeleteInstance)
		r.Get("/instances", s.handleListInstances)
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

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
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

	// Get authenticated user from context
	user, err := getUserFromContext(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	orgID := user.OrgID

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

	// Validate and default tier
	tier := req.Tier
	if tier == "" {
		writeError(w, http.StatusBadRequest, "Tier is required")
		// Rollback quota increment
		s.quotaMu.Lock()
		s.instanceCounts[orgID]--
		s.quotaMu.Unlock()
		return
	}

	// Validate tier is one of the allowed values
	validTiers := map[string]bool{"small": true, "medium": true, "large": true}
	if !validTiers[tier] {
		writeError(w, http.StatusBadRequest, "Invalid tier. Must be one of: small, medium, large")
		// Rollback quota increment
		s.quotaMu.Lock()
		s.instanceCounts[orgID]--
		s.quotaMu.Unlock()
		return
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

	// Track instance start time for billing
	s.startsMu.Lock()
	s.instanceStarts[instance.ID] = time.Now()
	s.startsMu.Unlock()

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
	// Convert http to ws scheme
	baseURL := s.baseURL
	if strings.HasPrefix(baseURL, "http://") {
		baseURL = "ws://" + strings.TrimPrefix(baseURL, "http://")
	} else if strings.HasPrefix(baseURL, "https://") {
		baseURL = "wss://" + strings.TrimPrefix(baseURL, "https://")
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
	_ = json.NewEncoder(w).Encode(resp)
}

// handleGetInstance handles get instance requests
func (s *Server) handleGetInstance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	instance, err := s.provider.GetInstance(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Instance not found")
		return
	}

	// Generate JWT token for attachment (valid for 2 minutes)
	token, err := s.tokenManager.GenerateToken(instance.ID, 2*time.Minute)
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to generate access token")
		return
	}

	// Build attach URL with token
	// Convert http to ws scheme
	baseURL := s.baseURL
	if strings.HasPrefix(baseURL, "http://") {
		baseURL = "ws://" + strings.TrimPrefix(baseURL, "http://")
	} else if strings.HasPrefix(baseURL, "https://") {
		baseURL = "wss://" + strings.TrimPrefix(baseURL, "https://")
	}
	attachURL := fmt.Sprintf("%s/v1/instances/%s/attach?token=%s",
		baseURL, instance.ID, url.QueryEscape(token))

	// Create response with instance data and attach URL
	resp := map[string]interface{}{
		"id":         instance.ID,
		"status":     instance.Status,
		"tier":       instance.Tier,
		"created_at": instance.CreatedAt,
		"attach_url": attachURL,
		"labels":     instance.Labels,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
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

	// Record usage before deletion
	s.recordInstanceUsage(instance)

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
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"instances": instances,
	})
}

// handleWSAttach handles WebSocket attach requests
func (s *Server) handleWSAttach(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	log.Printf("WebSocket attach request for instance %s", id)

	// Get token from query parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		log.Printf("Missing token for WebSocket attach to instance %s", id)
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

// handleGetBilling handles billing information requests
func (s *Server) handleGetBilling(w http.ResponseWriter, r *http.Request) {
	// For now, return mock data
	// TODO: Integrate with actual billing manager when available

	// Get authenticated user from context
	user, err := getUserFromContext(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	orgID := user.OrgID

	// Mock billing data
	billingInfo := map[string]interface{}{
		"organization":    orgID,
		"plan":            "Base + Usage ($20/mo)",
		"hours_used":      142.5,
		"hours_included":  200.0,
		"percent_used":    71,
		"in_overage":      false,
		"reset_date":      time.Now().AddDate(0, 0, 9).Format(time.RFC3339),
		"estimated_bill":  20.00,
		"daily_usage":     "5h 23m",
		"throttle_status": "OK - No limits exceeded",
	}

	// TODO: When billing manager is available, use:
	// if s.billingManager != nil {
	//     usage, err := s.billingManager.GetUsage(orgID)
	//     if err == nil {
	//         billingInfo["hours_used"] = usage.UsedHours
	//         billingInfo["hours_included"] = usage.IncludedHours
	//         billingInfo["percent_used"] = int(usage.PercentUsed)
	//         billingInfo["in_overage"] = usage.InOverage
	//     }
	// }

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(billingInfo)
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

		// Record usage before deletion
		s.recordInstanceUsage(&instance)

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

// recordInstanceUsage records usage when an instance is stopped/deleted
func (s *Server) recordInstanceUsage(instance *provider.Instance) {
	// Skip if billing is not configured
	if s.meteringService == nil {
		return
	}

	// Get start time
	s.startsMu.RLock()
	startTime, exists := s.instanceStarts[instance.ID]
	s.startsMu.RUnlock()

	if !exists {
		// Use creation time if start time not tracked
		startTime = instance.CreatedAt
	}

	// Calculate runtime in minutes
	runtime := time.Since(startTime)
	minutes := int(runtime.Minutes())

	// Don't record if less than 1 minute
	if minutes < 1 {
		return
	}

	// Get org ID and customer ID
	orgID, ok := instance.Labels["org-id"]
	if !ok || orgID == "" {
		log.Printf("Instance %s missing org-id label, skipping usage recording", instance.ID)
		return
	}

	// For now, use org ID as customer ID
	// TODO: Map org ID to Polar customer ID from subscription
	customerID := orgID

	// Record usage
	s.meteringService.RecordUsage(orgID, customerID, minutes, instance.Tier)
	log.Printf("Recorded usage for instance %s: %d minutes of %s tier", instance.ID, minutes, instance.Tier)

	// Clean up start time tracking
	s.startsMu.Lock()
	delete(s.instanceStarts, instance.ID)
	s.startsMu.Unlock()
}

// writeError writes an error response
func writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: message})
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

	// Stop billing service and flush pending usage
	if server.meteringService != nil {
		log.Println("Stopping billing service...")
		server.meteringService.Stop()
	}

	log.Println("Shutdown complete")
}
