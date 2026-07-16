package metrics

import (
	"github.com/zeromicro/go-zero/core/metric"
)

var (
	RequestsTotal = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: "matrix",
		Subsystem: "api",
		Name:      "requests_total",
		Help:      "Matrix API requests by tenant and status.",
		Labels:    []string{"tenant", "status"},
	})
	RequestDuration = metric.NewHistogramVec(&metric.HistogramVecOpts{
		Namespace: "matrix",
		Subsystem: "api",
		Name:      "request_duration_ms",
		Help:      "Matrix request duration in milliseconds.",
		Labels:    []string{"tenant"},
		Buckets:   []float64{50, 100, 250, 500, 1000, 2500, 5000, 10000, 30000},
	})
	FallbackEdges = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: "matrix",
		Subsystem: "engine",
		Name:      "fallback_edges_total",
		Help:      "Edges filled with haversine fallback.",
		Labels:    []string{"tenant"},
	})
	ProviderCalls = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: "matrix",
		Subsystem: "engine",
		Name:      "provider_calls_total",
		Help:      "Provider HTTP call batches.",
		Labels:    []string{"tenant", "provider"},
	})
)

func RecordMatrix(tenant, status string, elapsedMs int64, fallbackEdges, providerCalls int) {
	RequestsTotal.Inc(tenant, status)
	RequestDuration.Observe(elapsedMs, tenant)
	if fallbackEdges > 0 {
		FallbackEdges.Add(float64(fallbackEdges), tenant)
	}
	if providerCalls > 0 {
		ProviderCalls.Add(float64(providerCalls), tenant, "amap")
	}
}
