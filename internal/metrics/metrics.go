package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ActiveSessions tracks the number of active WebSocket sessions
	ActiveSessions = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "orzbob_active_sessions",
		Help: "The number of active WebSocket attachment sessions",
	})

	// InstancesCreated tracks the total number of instances created
	InstancesCreated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "orzbob_instances_created_total",
		Help: "The total number of instances created",
	})

	// InstancesDeleted tracks the total number of instances deleted
	InstancesDeleted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "orzbob_instances_deleted_total",
		Help: "The total number of instances deleted",
	})

	// QuotaExceeded tracks quota exceeded attempts
	QuotaExceeded = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "orzbob_quota_exceeded_total",
		Help: "The total number of quota exceeded attempts",
	}, []string{"org_id"})

	// HeartbeatsReceived tracks heartbeats received
	HeartbeatsReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "orzbob_heartbeats_received_total",
		Help: "The total number of heartbeats received from runners",
	})

	// IdleInstancesReaped tracks instances reaped for being idle
	IdleInstancesReaped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "orzbob_idle_instances_reaped_total",
		Help: "The total number of instances reaped for being idle",
	})

	// HTTPRequestDuration tracks HTTP request durations
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "orzbob_http_request_duration_seconds",
		Help:    "Duration of HTTP requests in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "endpoint", "status"})

	// InstancesPaused tracks instances paused by throttle service
	InstancesPaused = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "orzbob_instances_paused_total",
		Help: "The total number of instances paused by throttle service",
	}, []string{"reason"})

	// ThrottledInstances tracks currently throttled instances
	ThrottledInstances = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "orzbob_throttled_instances",
		Help: "The number of currently throttled instances",
	}, []string{"reason"})

	// DailyUsageHours tracks daily usage hours per organization
	DailyUsageHours = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "orzbob_daily_usage_hours",
		Help: "Daily usage hours per organization",
	}, []string{"org_id"})
)