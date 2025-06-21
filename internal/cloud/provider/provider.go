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
}

// Provider defines the interface for cloud instance providers
type Provider interface {
	// CreateInstance creates a new runner instance
	CreateInstance(ctx context.Context, tier string) (*Instance, error)
	
	// GetInstance retrieves instance details
	GetInstance(ctx context.Context, id string) (*Instance, error)
	
	// ListInstances lists all instances
	ListInstances(ctx context.Context) ([]*Instance, error)
	
	// DeleteInstance terminates an instance
	DeleteInstance(ctx context.Context, id string) error
	
	// GetAttachURL returns a URL for attaching to the instance
	GetAttachURL(ctx context.Context, id string) (string, error)
}