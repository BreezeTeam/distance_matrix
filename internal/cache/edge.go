package cache

import (
	"encoding/json"
	"time"
)

// Edge is a directed road segment stored in Redis.
type Edge struct {
	Origin      [2]float32 `json:"origin"`
	Destination [2]float32 `json:"destination"`
	DistanceM   float32    `json:"distance_m"`
	DurationS   float32    `json:"duration_s"`
	Polyline    string     `json:"polyline,omitempty"`
	WMT         string     `json:"w_m_t"`
	Provider    string     `json:"provider"`
	ComputedAt  time.Time  `json:"computed_at"`
}

func (e Edge) JSON() string {
	b, _ := json.Marshal(e)
	return string(b)
}

func ParseEdge(raw string) (Edge, error) {
	var e Edge
	err := json.Unmarshal([]byte(raw), &e)
	return e, err
}

// LookupOpts controls edge cache reads.
type LookupOpts struct {
	Tenant     string
	Method     int
	Strategy   int
	TimeSlot   string // hour bucket "14" or full w_m_h; empty = now
	Strict     bool
	GeoWideM   int
	EdgeTTLSec int
}

// ContextKey is method:strategy for HASH key segment.
func ContextKey(method, strategy int) string {
	return itoa(method) + ":" + itoa(strategy)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
