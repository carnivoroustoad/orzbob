package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"orzbob/internal/cloud/provider"
	"orzbob/internal/tunnel"
)

var (
	version = "0.1.0"
)

// API types
type CreateInstanceRequest struct {
	Tier    string `json:"tier,omitempty"`
	Program string `json:"program,omitempty"`
	RepoURL string `json:"repo_url,omitempty"`
	Branch  string `json:"branch,omitempty"`
}

type CreateInstanceResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	AttachURL string    `json:"attach_url"`
	CreatedAt time.Time `json:"created_at"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// Server represents the control plane server
type Server struct {
	provider    provider.Provider
	router      chi.Router
	wsProxy     *tunnel.WSProxy
	heartbeats  map[string]time.Time
	heartbeatMu sync.RWMutex
}

// NewServer creates a new control plane server
func NewServer(p provider.Provider) *Server {
	s := &Server{
		provider:   p,
		router:     chi.NewRouter(),
		wsProxy:    tunnel.NewWSProxy(),
		heartbeats: make(map[string]time.Time),
	}
	s.setupRoutes()
	
	// Start idle reaper
	go s.startIdleReaper()
	
	return s
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

	// API routes
	s.router.Route("/v1", func(r chi.Router) {
		r.Post("/instances", s.handleCreateInstance)
		r.Get("/instances/{id}", s.handleGetInstance)
		r.Delete("/instances/{id}", s.handleDeleteInstance)
		r.Get("/instances", s.handleListInstances)
		r.Get("/instances/{id}/attach", s.handleWSAttach)
		r.Post("/instances/{id}/heartbeat", s.handleHeartbeat)
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

	// Default tier if not specified
	tier := req.Tier
	if tier == "" {
		tier = "small"
	}

	// Create instance using provider
	instance, err := s.provider.CreateInstance(r.Context(), tier)
	if err != nil {
		log.Printf("Failed to create instance: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create instance")
		return
	}

	// Get attach URL
	attachURL, err := s.provider.GetAttachURL(r.Context(), instance.ID)
	if err != nil {
		log.Printf("Failed to get attach URL: %v", err)
		attachURL = fmt.Sprintf("ws://localhost:8080/v1/instances/%s/attach", instance.ID)
	}

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
	
	if err := s.provider.DeleteInstance(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "Instance not found")
		return
	}

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
	
	// Verify instance exists
	_, err := s.provider.GetInstance(r.Context(), id)
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

	s.heartbeatMu.RLock()
	defer s.heartbeatMu.RUnlock()

	for _, instance := range instances {
		lastHeartbeat, exists := s.heartbeats[instance.ID]
		
		// If no heartbeat recorded, use creation time
		if !exists {
			lastHeartbeat = instance.CreatedAt
		}

		// Check if idle
		if now.Sub(lastHeartbeat) > idleTimeout {
			log.Printf("Reaping idle instance %s (last heartbeat: %v)", instance.ID, lastHeartbeat)
			
			// Delete the instance
			if err := s.provider.DeleteInstance(ctx, instance.ID); err != nil {
				log.Printf("Failed to delete idle instance %s: %v", instance.ID, err)
			} else {
				// Remove from heartbeat map
				delete(s.heartbeats, instance.ID)
			}
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

	// Create provider
	var p provider.Provider
	var err error

	switch providerType {
	case "kind":
		if kubeconfig == "" {
			kubeconfig = os.Getenv("HOME") + "/.kube/config"
		}
		p, err = provider.NewLocalKind(kubeconfig)
		if err != nil {
			log.Fatalf("Failed to create kind provider: %v", err)
		}
		log.Printf("Using LocalKind provider with kubeconfig: %s", kubeconfig)
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