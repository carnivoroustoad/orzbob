package provider

import (
	"context"
	"time"
)

// Instance represents a cloud runner instance
type Instance struct {
	ID        string            `json:"id"`
	Status    string            `json:"status"`
	Tier      string            `json:"tier"`
	CreatedAt time.Time        `json:"created_at"`
	PodName   string            `json:"pod_name"`
	Namespace string            `json:"namespace"`
	Secrets   []string          `json:"secrets,omitempty"`   // Names of secrets to mount
	Labels    map[string]string `json:"labels,omitempty"`    // Additional labels (e.g., org-id)
}

// Secret represents a Kubernetes secret
type Secret struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Data      map[string]string `json:"data"`
	CreatedAt time.Time        `json:"created_at"`
}

// Provider defines the interface for cloud instance providers
type Provider interface {
	// Instance management
	CreateInstance(ctx context.Context, tier string) (*Instance, error)
	CreateInstanceWithSecrets(ctx context.Context, tier string, secrets []string) (*Instance, error)
	GetInstance(ctx context.Context, id string) (*Instance, error)
	ListInstances(ctx context.Context) ([]*Instance, error)
	DeleteInstance(ctx context.Context, id string) error
	GetAttachURL(ctx context.Context, id string) (string, error)
	
	// Secret management
	CreateSecret(ctx context.Context, name string, data map[string]string) (*Secret, error)
	GetSecret(ctx context.Context, name string) (*Secret, error)
	ListSecrets(ctx context.Context) ([]*Secret, error)
	DeleteSecret(ctx context.Context, name string) error
}