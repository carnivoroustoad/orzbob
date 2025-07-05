package billing

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// UsageMeterQueue tracks the number of usage samples waiting to be flushed
	UsageMeterQueue = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "orzbob_usage_meter_queue",
		Help: "Number of usage samples waiting to be sent to Polar",
	})

	// UsageMeterFlushTotal tracks the total number of flush operations
	UsageMeterFlushTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "orzbob_usage_meter_flush_total",
		Help: "Total number of usage meter flush operations",
	})

	// UsageMeterFlushErrors tracks the number of failed flush operations
	UsageMeterFlushErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "orzbob_usage_meter_flush_errors_total",
		Help: "Total number of failed usage meter flush operations",
	})

	// UsageMeterRecordsTotal tracks the total number of usage records sent
	UsageMeterRecordsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "orzbob_usage_meter_records_total",
		Help: "Total number of usage records sent to Polar",
	})
)

// UpdateMetrics updates Prometheus metrics for the metering service
func (m *MeteringService) UpdateMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()
	UsageMeterQueue.Set(float64(len(m.samples)))
}