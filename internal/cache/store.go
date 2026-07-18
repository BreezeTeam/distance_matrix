package cache

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Store persists directed edges in Redis (GEO + HASH + ZSET).
type Store struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

func NewStore(client *redis.Client, prefix string, edgeTTLSec int) *Store {
	if prefix == "" {
		prefix = "distance_matrix"
	}
	if edgeTTLSec <= 0 {
		edgeTTLSec = 1209600
	}
	return &Store{
		client: client,
		prefix: prefix,
		ttl:    time.Duration(edgeTTLSec) * time.Second,
	}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *Store) tenantPrefix(tenant string) string {
	if tenant == "" {
		tenant = "default"
	}
	return tenant + ":" + s.prefix
}

func (s *Store) geoKey(tenant string) string {
	return s.tenantPrefix(tenant) + ":geo"
}

func (s *Store) edgeKeysZSet(tenant string) string {
	return s.tenantPrefix(tenant) + ":edge_keys"
}

func (s *Store) hashKey(tenant string, method, strategy int, bGeo, eGeo string) string {
	return fmt.Sprintf("%s:edge:%s:%s:%s", s.tenantPrefix(tenant), ContextKey(method, strategy), bGeo, eGeo)
}

// Put writes one edge (write-through).
func (s *Store) Put(ctx context.Context, opts LookupOpts, e Edge) error {
	if opts.Tenant == "" {
		opts.Tenant = "default"
	}
	if e.WMT == "" {
		e.WMT = TimeSlotWMH(time.Now())
	}
	if e.ComputedAt.IsZero() {
		e.ComputedAt = time.Now()
	}
	bGeo := EncodeGeoHash(e.Origin[0], e.Origin[1])
	eGeo := EncodeGeoHash(e.Destination[0], e.Destination[1])
	hkey := s.hashKey(opts.Tenant, opts.Method, opts.Strategy, bGeo, eGeo)

	pipe := s.client.Pipeline()
	pipe.GeoAdd(ctx, s.geoKey(opts.Tenant),
		&redis.GeoLocation{Name: bGeo, Longitude: float64(e.Origin[0]), Latitude: float64(e.Origin[1])},
		&redis.GeoLocation{Name: eGeo, Longitude: float64(e.Destination[0]), Latitude: float64(e.Destination[1])},
	)
	pipe.HSet(ctx, hkey, e.WMT, e.JSON())
	pipe.Expire(ctx, hkey, s.ttl)
	pipe.ZAdd(ctx, s.edgeKeysZSet(opts.Tenant), redis.Z{Score: float64(time.Now().Unix()), Member: hkey})
	_, err := pipe.Exec(ctx)
	return err
}

// GetResult is a cache lookup outcome.
type GetResult struct {
	Edge   Edge
	Hit    bool
	Miss   bool // true when caller should fetch from provider
	BGeo   string
	EGeo   string
}

// Get loads an edge for origin→destination.
func (s *Store) Get(ctx context.Context, opts LookupOpts, origin, destination [2]float32) (GetResult, error) {
	if opts.Tenant == "" {
		opts.Tenant = "default"
	}
	if opts.GeoWideM <= 0 {
		opts.GeoWideM = 200
	}
	slot := opts.TimeSlot
	if slot == "" {
		slot = TimeSlotWMH(time.Now())
	}

	bExact := EncodeGeoHash(origin[0], origin[1])
	eExact := EncodeGeoHash(destination[0], destination[1])

	if edge, ok, err := s.tryHash(ctx, opts, bExact, eExact, slot); err != nil {
		return GetResult{}, err
	} else if ok {
		return GetResult{Edge: edge, Hit: true, BGeo: bExact, EGeo: eExact}, nil
	}

	if !opts.Strict {
		if edge, ok, err := s.tryFuzzySpatial(ctx, opts, origin, destination, slot); err != nil {
			return GetResult{}, err
		} else if ok {
			return GetResult{Edge: edge, Hit: true}, nil
		}
	}

	// Same geohash cell → zero-distance hit.
	if bExact == eExact {
		return GetResult{
			Edge: Edge{Origin: origin, Destination: destination, DistanceM: 0, DurationS: 0, WMT: slot},
			Hit:  true,
			BGeo: bExact, EGeo: eExact,
		}, nil
	}

	return GetResult{Miss: true, BGeo: bExact, EGeo: eExact}, nil
}

func (s *Store) tryHash(ctx context.Context, opts LookupOpts, bGeo, eGeo, slot string) (Edge, bool, error) {
	hkey := s.hashKey(opts.Tenant, opts.Method, opts.Strategy, bGeo, eGeo)
	fields := []string{slot}
	if !opts.Strict {
		fields = AdjacentSlots(slot)
	}
	vals, err := s.client.HMGet(ctx, hkey, fields...).Result()
	if err != nil {
		return Edge{}, false, err
	}
	for _, v := range vals {
		if v == nil {
			continue
		}
		raw, ok := v.(string)
		if !ok {
			continue
		}
		e, err := ParseEdge(raw)
		if err != nil {
			continue
		}
		return e, true, nil
	}
	return Edge{}, false, nil
}

func (s *Store) tryFuzzySpatial(ctx context.Context, opts LookupOpts, origin, destination [2]float32, slot string) (Edge, bool, error) {
	radius := float64(opts.GeoWideM) / 2
	if radius < 1 {
		radius = 100
	}
	gkey := s.geoKey(opts.Tenant)
	bs, err := s.client.GeoRadius(ctx, gkey, float64(origin[0]), float64(origin[1]), &redis.GeoRadiusQuery{
		Radius: radius, Unit: "m", Count: 3, Sort: "ASC",
	}).Result()
	if err != nil {
		return Edge{}, false, err
	}
	es, err := s.client.GeoRadius(ctx, gkey, float64(destination[0]), float64(destination[1]), &redis.GeoRadiusQuery{
		Radius: radius, Unit: "m", Count: 3, Sort: "ASC",
	}).Result()
	if err != nil {
		return Edge{}, false, err
	}
	bestDist := float64(opts.GeoWideM) + 1
	var best Edge
	found := false
	for _, b := range bs {
		for _, e := range es {
			if b.Name == "" || e.Name == "" {
				continue
			}
			sum := b.Dist + e.Dist
			if sum > float64(opts.GeoWideM) {
				continue
			}
			if edge, ok, err := s.tryHash(ctx, opts, b.Name, e.Name, slot); err != nil {
				return Edge{}, false, err
			} else if ok && sum < bestDist {
				bestDist = sum
				best = edge
				found = true
			}
			if edge, ok, err := s.tryHash(ctx, opts, e.Name, b.Name, slot); err != nil {
				return Edge{}, false, err
			} else if ok && sum < bestDist {
				bestDist = sum
				best = edge
				found = true
			}
		}
	}
	return best, found, nil
}

// ParseTenant normalizes tenant header value.
func ParseTenant(h string) string {
	h = strings.TrimSpace(h)
	if h == "" {
		return "default"
	}
	return h
}

// FormatCoordKey builds a stable map key for float32 pairs.
func FormatCoordKey(p [2]float32) string {
	return strconv.FormatFloat(float64(p[0]), 'f', 6, 32) + "," +
		strconv.FormatFloat(float64(p[1]), 'f', 6, 32)
}
