package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileQuotaPersistence implements QuotaPersistence using local file storage
type FileQuotaPersistence struct {
	filepath string
	mu       sync.Mutex
}

// NewFileQuotaPersistence creates a new file-based persistence
func NewFileQuotaPersistence(dataDir string) (*FileQuotaPersistence, error) {
	// Ensure directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	return &FileQuotaPersistence{
		filepath: filepath.Join(dataDir, "quota_usage.json"),
	}, nil
}

// Save persists usage data to file
func (f *FileQuotaPersistence) Save(ctx context.Context, usage map[string]*OrgUsage) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := json.MarshalIndent(usage, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal usage data: %w", err)
	}

	// Write to temp file first
	tmpFile := f.filepath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpFile, f.filepath); err != nil {
		os.Remove(tmpFile) // Clean up temp file
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// Load reads usage data from file
func (f *FileQuotaPersistence) Load(ctx context.Context) (map[string]*OrgUsage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.filepath)
	if err != nil {
		if os.IsNotExist(err) {
			// No data yet
			return make(map[string]*OrgUsage), nil
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var usage map[string]*OrgUsage
	if err := json.Unmarshal(data, &usage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal usage data: %w", err)
	}

	return usage, nil
}

// MemoryQuotaPersistence implements in-memory persistence for testing
type MemoryQuotaPersistence struct {
	mu   sync.RWMutex
	data map[string]*OrgUsage
}

// NewMemoryQuotaPersistence creates a new in-memory persistence
func NewMemoryQuotaPersistence() *MemoryQuotaPersistence {
	return &MemoryQuotaPersistence{
		data: make(map[string]*OrgUsage),
	}
}

// Save stores usage data in memory
func (m *MemoryQuotaPersistence) Save(ctx context.Context, usage map[string]*OrgUsage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Deep copy
	m.data = make(map[string]*OrgUsage)
	for k, v := range usage {
		copied := *v
		m.data[k] = &copied
	}

	return nil
}

// Load retrieves usage data from memory
func (m *MemoryQuotaPersistence) Load(ctx context.Context) (map[string]*OrgUsage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Deep copy
	result := make(map[string]*OrgUsage)
	for k, v := range m.data {
		copied := *v
		result[k] = &copied
	}

	return result, nil
}
