package provider

import (
	"context"
	"time"
)

// Instance represents a cloud runner instance
type Instance struct {
	ID        string
	Status    string
	Tier      string
	CreatedAt time.Time
	PodName   string
	Namespace string
	Secrets   []string // Names of secrets to mount
}

// Secret represents a Kubernetes secret
type Secret struct {
	Name      string
	Namespace string
	Data      map[string]string
	CreatedAt time.Time
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