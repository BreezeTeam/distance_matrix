package persist

import (
	"context"
	"strconv"
	"sync"
	"time"

	"distance-matrix/internal/cache"
)

// Memory is an in-process Archive for tests (same Get/Upsert semantics, no MySQL).
type Memory struct {
	mu   sync.RWMutex
	rows map[string]cache.Edge // key: tenant|m|s|start|end|wmt
}

func NewMemory() *Memory {
	return &Memory{rows: make(map[string]cache.Edge)}
}

func memKey(tenant string, method, strategy int, start, end, wmt string) string {
	return tenant + "|" + strconv.Itoa(method) + "|" + strconv.Itoa(strategy) + "|" + start + "|" + end + "|" + wmt
}

func (m *Memory) Close() error { return nil }

func (m *Memory) Upsert(_ context.Context, opts cache.LookupOpts, e cache.Edge) error {
	if e.DistanceM == 0 {
		return nil
	}
	tenant := opts.Tenant
	if tenant == "" {
		tenant = "default"
	}
	wmt := e.WMT
	if wmt == "" {
		wmt = cache.TimeSlotWMH(time.Now())
	}
	start := cache.EncodeGeoHash(e.Origin[0], e.Origin[1])
	end := cache.EncodeGeoHash(e.Destination[0], e.Destination[1])
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := e
	cp.WMT = wmt
	m.rows[memKey(tenant, opts.Method, opts.Strategy, start, end, wmt)] = cp
	return nil
}

func (m *Memory) Get(_ context.Context, opts cache.LookupOpts, origin, destination [2]float32) (cache.Edge, bool, error) {
	tenant := opts.Tenant
	if tenant == "" {
		tenant = "default"
	}
	slot := opts.TimeSlot
	if slot == "" {
		slot = cache.TimeSlotWMH(time.Now())
	}
	start := cache.EncodeGeoHash(origin[0], origin[1])
	end := cache.EncodeGeoHash(destination[0], destination[1])

	slots := []string{slot}
	if !opts.Strict {
		slots = cache.AdjacentSlots(slot)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, sl := range slots {
		if e, ok := m.rows[memKey(tenant, opts.Method, opts.Strategy, start, end, sl)]; ok {
			return e, true, nil
		}
	}
	if opts.Strict {
		return cache.Edge{}, false, nil
	}
	wantT := cache.HourBucket(slot)
	prefix := tenant + "|" + strconv.Itoa(opts.Method) + "|" + strconv.Itoa(opts.Strategy) + "|" + start + "|" + end + "|"
	for k, e := range m.rows {
		if len(k) <= len(prefix) || k[:len(prefix)] != prefix {
			continue
		}
		if cache.HourBucket(e.WMT) == wantT {
			return e, true, nil
		}
	}
	return cache.Edge{}, false, nil
}
