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
	secrets   map[string]*Secret
}

// NewFakeProvider creates a new fake provider
func NewFakeProvider() *FakeProvider {
	return &FakeProvider{
		instances: make(map[string]*Instance),
		secrets:   make(map[string]*Secret),
	}
}

// CreateInstance creates a fake instance
func (f *FakeProvider) CreateInstance(ctx context.Context, tier string) (*Instance, error) {
	return f.CreateInstanceWithSecrets(ctx, tier, nil)
}

// CreateInstanceWithSecrets creates a fake instance with secrets
func (f *FakeProvider) CreateInstanceWithSecrets(ctx context.Context, tier string, secrets []string) (*Instance, error) {
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
		Secrets:   secrets,
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

// CreateSecret creates a fake secret
func (f *FakeProvider) CreateSecret(ctx context.Context, name string, data map[string]string) (*Secret, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if secret already exists
	if _, exists := f.secrets[name]; exists {
		return nil, fmt.Errorf("secret already exists: %s", name)
	}

	secret := &Secret{
		Name:      name,
		Namespace: "fake-namespace",
		Data:      data,
		CreatedAt: time.Now(),
	}

	f.secrets[name] = secret
	return secret, nil
}

// GetSecret retrieves a fake secret
func (f *FakeProvider) GetSecret(ctx context.Context, name string) (*Secret, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	secret, exists := f.secrets[name]
	if !exists {
		return nil, fmt.Errorf("secret not found: %s", name)
	}

	return secret, nil
}

// ListSecrets lists all fake secrets
func (f *FakeProvider) ListSecrets(ctx context.Context) ([]*Secret, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	secrets := make([]*Secret, 0, len(f.secrets))
	for _, secret := range f.secrets {
		secrets = append(secrets, secret)
	}

	return secrets, nil
}

// DeleteSecret deletes a fake secret
func (f *FakeProvider) DeleteSecret(ctx context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.secrets[name]; !exists {
		return fmt.Errorf("secret not found: %s", name)
	}

	delete(f.secrets, name)
	return nil
}