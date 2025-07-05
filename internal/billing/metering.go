package billing

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// UsageSample represents a usage event
type UsageSample struct {
	OrgID      string
	CustomerID string
	Minutes    int
	Tier       string
	Timestamp  time.Time
}

// MeteringService handles batching and sending usage data to Polar
type MeteringService struct {
	client      PolarClientInterface
	samples     []UsageSample
	mu          sync.Mutex
	stopCh      chan struct{}
	flushTimer  *time.Timer
	config      *Config
	quotaEngine *QuotaEngine
}

// NewMeteringService creates a new metering service
func NewMeteringService(config *Config) (*MeteringService, error) {
	if !config.IsConfigured() {
		return nil, fmt.Errorf("billing configuration is not complete")
	}

	client := NewPolarClient(config.PolarAPIKey, config.PolarOrgID)
	return NewMeteringServiceWithClient(config, client)
}

// NewMeteringServiceWithClient creates a new metering service with a specific client
func NewMeteringServiceWithClient(config *Config, client PolarClientInterface) (*MeteringService, error) {
	// Create quota engine with file persistence
	var quotaEngine *QuotaEngine
	dataDir := os.Getenv("ORZBOB_DATA_DIR")
	if dataDir == "" {
		dataDir = "/var/lib/orzbob"
	}

	var persistence QuotaPersistence
	persistence, err := NewFileQuotaPersistence(filepath.Join(dataDir, "billing"))
	if err != nil {
		log.Printf("Failed to create quota persistence, using memory: %v", err)
		persistence = NewMemoryQuotaPersistence()
	}

	quotaEngine, err = NewQuotaEngine(client, persistence)
	if err != nil {
		return nil, fmt.Errorf("failed to create quota engine: %w", err)
	}

	return &MeteringService{
		client:      client,
		samples:     make([]UsageSample, 0),
		stopCh:      make(chan struct{}),
		config:      config,
		quotaEngine: quotaEngine,
	}, nil
}

// Start begins the background flush process
func (m *MeteringService) Start(ctx context.Context) {
	go m.flushLoop(ctx)
}

// Stop gracefully shuts down the metering service
func (m *MeteringService) Stop() {
	close(m.stopCh)
	// Flush any remaining samples
	m.Flush(context.Background())
}

// RecordUsage adds a usage sample to the batch
func (m *MeteringService) RecordUsage(orgID, customerID string, minutes int, tier string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sample := UsageSample{
		OrgID:      orgID,
		CustomerID: customerID,
		Minutes:    minutes,
		Tier:       tier,
		Timestamp:  time.Now(),
	}

	m.samples = append(m.samples, sample)

	// Update metrics
	UsageMeterQueue.Set(float64(len(m.samples)))

	// Reset flush timer when new sample is added
	if m.flushTimer != nil {
		m.flushTimer.Stop()
	}
	m.flushTimer = time.AfterFunc(60*time.Second, func() {
		m.Flush(context.Background())
	})
}

// flushLoop runs the periodic flush process
func (m *MeteringService) flushLoop(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.Flush(ctx); err != nil {
				log.Printf("Failed to flush usage data: %v", err)
			}
		case <-m.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Flush sends all pending usage samples to Polar
func (m *MeteringService) Flush(ctx context.Context) error {
	m.mu.Lock()
	if len(m.samples) == 0 {
		m.mu.Unlock()
		return nil
	}

	// Copy samples and clear the buffer
	samplesToFlush := make([]UsageSample, len(m.samples))
	copy(samplesToFlush, m.samples)
	m.samples = m.samples[:0]

	// Update metrics
	UsageMeterQueue.Set(0)
	UsageMeterFlushTotal.Inc()
	m.mu.Unlock()

	// Aggregate usage by customer and tier
	aggregated := make(map[string]float64)
	metadata := make(map[string]Metadata)

	for _, sample := range samplesToFlush {
		key := fmt.Sprintf("%s:%s", sample.CustomerID, sample.Tier)
		hours := UsageToHours(sample.Minutes, sample.Tier)
		aggregated[key] += hours
		metadata[key] = Metadata{
			OrgID: sample.OrgID,
			Tier:  sample.Tier,
		}
	}

	// Send aggregated usage to Polar
	var errors []error
	for key, hours := range aggregated {
		customerID := key[:len(key)-len(metadata[key].Tier)-1]

		record := MeterUsageRecord{
			CustomerID: customerID,
			Usage:      hours,
			Timestamp:  time.Now(),
			Metadata:   metadata[key],
		}

		if err := m.client.RecordUsage(ctx, record); err != nil {
			errors = append(errors, fmt.Errorf("failed to record usage for %s: %w", customerID, err))
			UsageMeterFlushErrors.Inc()
		} else {
			UsageMeterRecordsTotal.Inc()

			// Update quota tracking
			if m.quotaEngine != nil {
				if err := m.quotaEngine.RecordUsage(metadata[key].OrgID, customerID, hours); err != nil {
					log.Printf("Failed to update quota for %s: %v", metadata[key].OrgID, err)
				}
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("flush completed with %d errors: %v", len(errors), errors)
	}

	log.Printf("Successfully flushed %d usage samples (aggregated to %d records)", len(samplesToFlush), len(aggregated))
	return nil
}

// GetQueueSize returns the number of samples waiting to be flushed
func (m *MeteringService) GetQueueSize() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.samples)
}

// GetQuotaStatus returns the current quota status for an organization
func (m *MeteringService) GetQuotaStatus(orgID string) (*UsageStatus, error) {
	if m.quotaEngine == nil {
		return nil, fmt.Errorf("quota engine not initialized")
	}
	return m.quotaEngine.GetUsageStatus(orgID)
}
