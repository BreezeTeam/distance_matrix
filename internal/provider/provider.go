package provider

import (
	"context"

	"distance-matrix/internal/cache"
)

// Step is one directed segment.
type Step struct {
	Origin      [2]float32
	Destination [2]float32
	DistanceM   float32
	DurationS   float32
	Polyline    string
	Source      Source
}

type Source string

const (
	SourceProvider Source = "provider"
	SourceCache    Source = "cache"
	SourceFallback Source = "fallback"
)

// RouteRequest is a multi-waypoint driving query.
type RouteRequest struct {
	Waypoints [][2]float32
	Strategy  int
	Method    int
	SpeedMPS  int
}

// RouteResult is merged route output.
type RouteResult struct {
	Steps      []Step
	DistanceM  float32
	DurationS  float32
	Source     Source
	Degraded   bool
}

// Provider computes road routes.
type Provider interface {
	Name() string
	Route(ctx context.Context, req RouteRequest) (*RouteResult, error)
	Ready() bool
}

// StepToEdge converts a step for cache storage.
func StepToEdge(s Step, wmt, providerName string) cache.Edge {
	return cache.Edge{
		Origin:      s.Origin,
		Destination: s.Destination,
		DistanceM:   s.DistanceM,
		DurationS:   s.DurationS,
		Polyline:    s.Polyline,
		WMT:         wmt,
		Provider:    providerName,
	}
}
