package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"orzbob/internal/tunnel"
	"orzbob/log"
	"orzbob/session"
)

// Manager handles cloud instance operations
type Manager struct {
	apiURL     string
	httpClient *http.Client
	tokenPath  string
	
	// WebSocket connections
	connections map[string]*tunnel.Client
	connMu      sync.RWMutex
}

// NewManager creates a new cloud manager
func NewManager() *Manager {
	apiURL := os.Getenv("ORZBOB_API_URL")
	if apiURL == "" {
		apiURL = "http://api.orzbob.com"
	}
	
	return &Manager{
		apiURL:      apiURL,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		tokenPath:   filepath.Join(os.Getenv("HOME"), ".config", "orzbob", "token.json"),
		connections: make(map[string]*tunnel.Client),
	}
}

// CloudInstance represents a cloud instance from the API
type CloudInstance struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Tier      string    `json:"tier"`
	CreatedAt time.Time `json:"created_at"`
	AttachURL string    `json:"attach_url"`
}

// ListInstances fetches all cloud instances from the API
func (m *Manager) ListInstances(ctx context.Context) ([]CloudInstance, error) {
	token, err := m.loadToken()
	if err != nil {
		return nil, fmt.Errorf("not authenticated: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", m.apiURL+"/v1/instances", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}
	
	var response struct {
		Instances []CloudInstance `json:"instances"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return response.Instances, nil
}

// CreateInstance creates a new cloud instance
func (m *Manager) CreateInstance(ctx context.Context, tier string) (*CloudInstance, error) {
	token, err := m.loadToken()
	if err != nil {
		return nil, fmt.Errorf("not authenticated: %w", err)
	}
	
	reqBody, _ := json.Marshal(map[string]string{
		"tier": tier,
	})
	
	req, err := http.NewRequestWithContext(ctx, "POST", m.apiURL+"/v1/instances", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}
	
	var instance CloudInstance
	if err := json.NewDecoder(resp.Body).Decode(&instance); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return &instance, nil
}

// DeleteInstance deletes a cloud instance
func (m *Manager) DeleteInstance(ctx context.Context, instanceID string) error {
	token, err := m.loadToken()
	if err != nil {
		return fmt.Errorf("not authenticated: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "DELETE", m.apiURL+"/v1/instances/"+instanceID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete instance: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s", string(body))
	}
	
	// Close any existing connection
	m.CloseConnection(instanceID)
	
	return nil
}

// GetInstanceWithAttachURL fetches instance details including fresh attach URL
func (m *Manager) GetInstanceWithAttachURL(ctx context.Context, instanceID string) (*CloudInstance, error) {
	token, err := m.loadToken()
	if err != nil {
		return nil, fmt.Errorf("not authenticated: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", m.apiURL+"/v1/instances/"+instanceID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}
	
	var instance CloudInstance
	if err := json.NewDecoder(resp.Body).Decode(&instance); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return &instance, nil
}

// Connect establishes a WebSocket connection to a cloud instance
func (m *Manager) Connect(ctx context.Context, instanceID string, attachURL string) (*tunnel.Client, error) {
	// Close any existing connection
	m.CloseConnection(instanceID)
	
	// Parse URL to extract token
	parsedURL, err := url.Parse(attachURL)
	if err != nil {
		return nil, fmt.Errorf("invalid attach URL: %w", err)
	}
	
	token := parsedURL.Query().Get("token")
	if token == "" {
		return nil, fmt.Errorf("no token found in attach URL")
	}
	
	// Convert to WebSocket URL
	wsURL := attachURL
	if strings.HasPrefix(wsURL, "http://") {
		wsURL = "ws://" + strings.TrimPrefix(wsURL, "http://")
	} else if strings.HasPrefix(wsURL, "https://") {
		wsURL = "wss://" + strings.TrimPrefix(wsURL, "https://")
	}
	
	// Remove token from URL, we'll pass it separately
	parsedURL, _ = url.Parse(wsURL)
	parsedURL.RawQuery = ""
	wsURL = parsedURL.String()
	
	// Create WebSocket client
	client, err := tunnel.NewClientWithToken(wsURL, token)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	
	// Store connection
	m.connMu.Lock()
	m.connections[instanceID] = client
	m.connMu.Unlock()
	
	return client, nil
}

// GetConnection returns an existing connection for an instance
func (m *Manager) GetConnection(instanceID string) *tunnel.Client {
	m.connMu.RLock()
	defer m.connMu.RUnlock()
	return m.connections[instanceID]
}

// CloseConnection closes the WebSocket connection for an instance
func (m *Manager) CloseConnection(instanceID string) {
	m.connMu.Lock()
	defer m.connMu.Unlock()
	
	if client, exists := m.connections[instanceID]; exists {
		if err := client.Close(); err != nil {
			log.ErrorLog.Printf("Failed to close connection for %s: %v", instanceID, err)
		}
		delete(m.connections, instanceID)
	}
}

// CloseAllConnections closes all WebSocket connections
func (m *Manager) CloseAllConnections() {
	m.connMu.Lock()
	defer m.connMu.Unlock()
	
	for id, client := range m.connections {
		if err := client.Close(); err != nil {
			log.ErrorLog.Printf("Failed to close connection for %s: %v", id, err)
		}
	}
	m.connections = make(map[string]*tunnel.Client)
}

// IsAuthenticated checks if the user is authenticated
func (m *Manager) IsAuthenticated() bool {
	_, err := m.loadToken()
	return err == nil
}

// loadToken loads the API token from disk
func (m *Manager) loadToken() (string, error) {
	file, err := os.Open(m.tokenPath)
	if err != nil {
		return "", fmt.Errorf("not authenticated")
	}
	defer file.Close()
	
	var data struct {
		APIToken  string    `json:"api_token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return "", err
	}
	
	if time.Now().After(data.ExpiresAt) {
		return "", fmt.Errorf("session expired")
	}
	
	return data.APIToken, nil
}

// ConvertToSessionInstances converts cloud instances to session instances
func ConvertToSessionInstances(cloudInstances []CloudInstance) []*session.Instance {
	instances := make([]*session.Instance, len(cloudInstances))
	
	for i, ci := range cloudInstances {
		instances[i] = &session.Instance{
			Title:           fmt.Sprintf("cloud-%s", ci.ID[:8]),
			Path:            "/cloud/" + ci.ID,
			Branch:          "",
			Status:          session.Ready,
			Program:         "claude",
			IsCloud:         true,
			CloudInstanceID: ci.ID,
			AttachURL:       ci.AttachURL,
			CloudTier:       ci.Tier,
			CloudStatus:     ci.Status,
			CreatedAt:       ci.CreatedAt,
			UpdatedAt:       time.Now(),
		}
	}
	
	return instances
}