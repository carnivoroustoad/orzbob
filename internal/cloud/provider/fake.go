package provider

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// FakeProvider implements Provider with in-memory storage for testing
type FakeProvider struct {
	mu        sync.RWMutex
	instances map[string]*Instance
}

// NewFakeProvider creates a new fake provider
func NewFakeProvider() *FakeProvider {
	return &FakeProvider{
		instances: make(map[string]*Instance),
	}
}

// CreateInstance creates a fake instance
func (f *FakeProvider) CreateInstance(ctx context.Context, tier string) (*Instance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Generate instance ID
	id := fmt.Sprintf("runner-%d", time.Now().UnixNano())
	
	instance := &Instance{
		ID:        id,
		Status:    "Running",
		Tier:      tier,
		CreatedAt: time.Now(),
		PodName:   id,
		Namespace: "fake-namespace",
	}

	f.instances[id] = instance
	return instance, nil
}

// GetInstance retrieves a fake instance
func (f *FakeProvider) GetInstance(ctx context.Context, id string) (*Instance, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	instance, exists := f.instances[id]
	if !exists {
		return nil, fmt.Errorf("instance not found: %s", id)
	}

	return instance, nil
}

// ListInstances lists all fake instances
func (f *FakeProvider) ListInstances(ctx context.Context) ([]*Instance, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	instances := make([]*Instance, 0, len(f.instances))
	for _, instance := range f.instances {
		instances = append(instances, instance)
	}

	return instances, nil
}

// DeleteInstance deletes a fake instance
func (f *FakeProvider) DeleteInstance(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.instances[id]; !exists {
		return fmt.Errorf("instance not found: %s", id)
	}

	delete(f.instances, id)
	return nil
}

// GetAttachURL returns a fake attach URL
func (f *FakeProvider) GetAttachURL(ctx context.Context, id string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if _, exists := f.instances[id]; !exists {
		return "", fmt.Errorf("instance not found: %s", id)
	}

	return fmt.Sprintf("ws://localhost:8080/v1/instances/%s/attach", id), nil
}