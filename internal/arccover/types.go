package arccover

import (
	"context"
	"time"
)

// VertexID is a dense integer vertex index in [0, N).
type VertexID uint32

// EdgeID indexes into the required edge slice (and Plan coverage bitset).
type EdgeID uint32

// InvalidVertex marks an absent vertex choice.
const InvalidVertex VertexID = ^VertexID(0)

// Arc is a required directed edge.
type Arc struct {
	ID   EdgeID
	From VertexID
	To   VertexID
}

// LegKind distinguishes coverage legs from bridges.
type LegKind uint8

const (
	LegRequired LegKind = iota
	LegBridge
)

// Leg is one hop in a Provider walk.
type Leg struct {
	From   VertexID
	To     VertexID
	Kind   LegKind
	EdgeID EdgeID // valid only when Kind == LegRequired
}

// Route is a continuous directed walk of legs.
type Route struct {
	Legs []Leg
}

// Start returns the first vertex of the route.
func (r Route) Start() VertexID {
	return r.Legs[0].From
}

// End returns the last vertex of the route.
func (r Route) End() VertexID {
	return r.Legs[len(r.Legs)-1].To
}

// PlanStats summarizes planner quality and cost.
type PlanStats struct {
	RequiredEdges          int
	ProviderCalls          int
	BridgeEdges            int
	DuplicateRequiredEdges int
	SinglePairFallbacks    int

	LowerBound    int
	OptimalityGap float64
	Optimal       bool

	PlannerRuntime time.Duration

	LowerBoundRuntime time.Duration
	CandidateRuntime  time.Duration
	RepairRuntime     time.Duration
	ValidationRuntime time.Duration

	Cancelled bool
	Branch    string // "dense"
}

// Plan is the planner output.
type Plan struct {
	Routes []Route
	Stats  PlanStats
}

// Config controls DensePlanner.
type Config struct {
	MaxLegs              int
	DenseCandidateWindow int
}

// DefaultConfig returns production defaults (MaxLegs often set by caller).
func DefaultConfig() Config {
	return Config{
		MaxLegs:              17,
		DenseCandidateWindow: 8,
	}
}

// ArcCoverPlanner plans Provider walks covering required arcs.
type ArcCoverPlanner interface {
	Plan(ctx context.Context, vertexCount int, required []Arc, maxLegs int) (Plan, error)
}

// Fragment is a required-only contiguous trail segment.
type Fragment struct {
	Edges []Arc
	ID    int
}

// Start returns the first vertex of the fragment.
func (f Fragment) Start() VertexID {
	return f.Edges[0].From
}

// End returns the last vertex of the fragment.
func (f Fragment) End() VertexID {
	return f.Edges[len(f.Edges)-1].To
}

// Len returns the number of required edges.
func (f Fragment) Len() int {
	return len(f.Edges)
}
