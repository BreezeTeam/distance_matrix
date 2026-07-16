package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"distance-matrix/internal/config"
)

// User-story tests exercise handler → engine → cache → stub provider without a second HTTP server.

func TestStory01_ColdMatrixSymmetric(t *testing.T) {
	env := newStoryEnv(t, nil)
	rr := env.postMatrix(t, map[string]any{
		"points": [][]float32{
			{116.40, 39.90},
			{116.41, 39.91},
			{116.42, 39.92},
		},
		"coordinate": "gcj02",
		"strict":     true,
	}, map[string]string{"X-Tenant-Id": "story-01"})
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	out := parseMatrixBody(t, rr)
	assertSymmetricMatrix(t, out.Data.Distances)
	if env.Stub.Calls.Load() == 0 {
		t.Fatal("expected provider calls on cold matrix")
	}
}

func TestStory02_CacheWriteThroughRetry(t *testing.T) {
	env := newStoryEnv(t, nil)
	body := map[string]any{
		"points":     [][]float32{{116.55, 39.85}, {116.56, 39.86}},
		"coordinate": "gcj02",
		"strict":     true,
	}
	headers := map[string]string{"X-Tenant-Id": "story-02"}

	rr1 := env.postMatrix(t, body, headers)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first: %s", rr1.Body.String())
	}
	calls1 := env.Stub.Calls.Load()

	rr2 := env.postMatrix(t, body, headers)
	if rr2.Code != http.StatusOK {
		t.Fatalf("second: %s", rr2.Body.String())
	}
	if env.Stub.Calls.Load() != calls1 {
		t.Fatalf("retry should not call provider again: calls1=%d calls2=%d", calls1, env.Stub.Calls.Load())
	}
	a := parseMatrixBody(t, rr1)
	b := parseMatrixBody(t, rr2)
	if a.Data.Distances[0][1] != b.Data.Distances[0][1] {
		t.Fatalf("cache mismatch: %f vs %f", a.Data.Distances[0][1], b.Data.Distances[0][1])
	}
	if len(redisKeysWithPrefix(env.MR, "story-02:")) == 0 {
		t.Fatal("expected redis edge keys for tenant story-02")
	}
}

func TestStory03_Deadline504PartialWriteThrough(t *testing.T) {
	env := newStoryEnv(t, func(c *config.Config) {
		c.Timeout = 80
	})
	stubDelay(env.Stub, 200*time.Millisecond)

	body := map[string]any{
		"points": [][]float32{
			{116.70, 39.60},
			{116.71, 39.61},
			{116.72, 39.62},
			{116.73, 39.63},
		},
		"coordinate": "gcj02",
		"strict":     true,
	}
	headers := map[string]string{"X-Tenant-Id": "story-03"}

	rr1 := env.postMatrix(t, body, headers)
	if rr1.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504, got %d body=%s", rr1.Code, rr1.Body.String())
	}
	out1 := parseMatrixBody(t, rr1)
	if out1.Code != 504 || out1.Msg != "MATRIX_DEADLINE" {
		t.Fatalf("unexpected 504 payload: %+v", out1)
	}

	stubDelay(env.Stub, 0)
	rr2 := env.postMatrix(t, body, headers)
	if rr2.Code != http.StatusOK {
		t.Fatalf("retry failed: %d body=%s", rr2.Code, rr2.Body.String())
	}
}

func TestStory04_MethodIsolation(t *testing.T) {
	env := newStoryEnv(t, nil)
	points := [][]float32{{116.80, 39.50}, {116.81, 39.51}}
	headers := map[string]string{"X-Tenant-Id": "story-04"}

	env.postMatrix(t, map[string]any{
		"points": points, "coordinate": "gcj02", "method": 0, "strict": true,
	}, headers)
	callsAfterM0 := env.Stub.Calls.Load()

	env.postMatrix(t, map[string]any{
		"points": points, "coordinate": "gcj02", "method": 1, "strict": true,
	}, headers)
	if env.Stub.Calls.Load() <= callsAfterM0 {
		t.Fatal("method=1 should miss method=0 cache and call provider again")
	}
}

func TestStory05_TenantCacheIsolation(t *testing.T) {
	env := newStoryEnv(t, nil)
	body := map[string]any{
		"points":     [][]float32{{116.90, 39.40}, {116.91, 39.41}},
		"coordinate": "gcj02",
		"strict":     true,
	}

	env.postMatrix(t, body, map[string]string{"X-Tenant-Id": "tenant-a"})
	env.postMatrix(t, body, map[string]string{"X-Tenant-Id": "tenant-b"})

	keysA := redisKeysWithPrefix(env.MR, "tenant-a:")
	keysB := redisKeysWithPrefix(env.MR, "tenant-b:")
	if len(keysA) == 0 || len(keysB) == 0 {
		t.Fatalf("expected keys for both tenants: a=%d b=%d", len(keysA), len(keysB))
	}
	for _, ka := range keysA {
		for _, kb := range keysB {
			if ka == kb {
				t.Fatalf("shared redis key across tenants: %s", ka)
			}
		}
	}

	callsBefore := env.Stub.Calls.Load()
	rr := env.postMatrix(t, body, map[string]string{"X-Tenant-Id": "tenant-a"})
	if rr.Code != http.StatusOK {
		t.Fatalf("tenant-a retry status=%d", rr.Code)
	}
	if env.Stub.Calls.Load() != callsBefore {
		t.Fatal("tenant-a retry should hit cache without provider calls")
	}
}

func TestStory06_RateLimit429(t *testing.T) {
	env := newStoryEnv(t, func(c *config.Config) {
		c.Engine.TenantQPS = 2
	})
	body := map[string]any{
		"points":     [][]float32{{117.00, 39.30}, {117.01, 39.31}},
		"coordinate": "gcj02",
	}
	headers := map[string]string{"X-Tenant-Id": "story-06"}

	var got429 bool
	for i := 0; i < 5; i++ {
		rr := env.postMatrix(t, body, headers)
		if rr.Code == http.StatusTooManyRequests {
			got429 = true
			if rr.Body.String() != `{"code":429,"msg":"RATE_LIMIT"}` {
				t.Fatalf("unexpected 429 body: %s", rr.Body.String())
			}
		}
	}
	if !got429 {
		t.Fatal("expected at least one 429 RATE_LIMIT")
	}

	rr := env.postMatrix(t, body, map[string]string{"X-Tenant-Id": "story-06-other"})
	if rr.Code != http.StatusOK {
		t.Fatalf("other tenant should not inherit rate limit: %d", rr.Code)
	}
}

func TestStory07_MaxPointsRejected(t *testing.T) {
	env := newStoryEnv(t, func(c *config.Config) {
		c.Engine.MaxPoints = 5
	})
	points := make([][]float32, 6)
	for i := range points {
		points[i] = []float32{116.40 + float32(i)*0.01, 39.90}
	}
	rr := env.postMatrix(t, map[string]any{
		"points": points, "coordinate": "gcj02",
	}, nil)
	if rr.Code == http.StatusOK {
		t.Fatal("expected error when points exceed MaxPoints")
	}
}

func TestStory08_CachedStrictRetry(t *testing.T) {
	env := newStoryEnv(t, nil)
	body := map[string]any{
		"points":     [][]float32{{117.10, 39.20}, {117.11, 39.21}},
		"coordinate": "gcj02",
		"strict":     true,
	}
	headers := map[string]string{"X-Tenant-Id": "story-08"}

	rr1 := env.postMatrix(t, body, headers)
	out := parseMatrixBody(t, rr1)
	if out.Data.Distances[0][1] != 1000 {
		t.Fatalf("expected stub distance 1000, got %f", out.Data.Distances[0][1])
	}
	calls := env.Stub.Calls.Load()
	rr2 := env.postMatrix(t, body, headers)
	if rr2.Code != http.StatusOK {
		t.Fatal("cached retry failed")
	}
	if env.Stub.Calls.Load() != calls {
		t.Fatal("strict retry should use cache")
	}
}

func TestStory09_RouteMultiWaypoint(t *testing.T) {
	env := newStoryEnv(t, nil)
	rr := env.postRoute(t, map[string]any{
		"points": [][]float32{
			{116.40, 39.90},
			{116.41, 39.91},
			{116.42, 39.92},
			{116.43, 39.93},
		},
		"coordinate": "gcj02",
	}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var out struct {
		Code int `json:"code"`
		Data struct {
			Distance float32 `json:"distance"`
			Duration float32 `json:"duration"`
			Steps    []any   `json:"steps"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Data.Steps) != 3 || out.Data.Distance <= 0 || out.Data.Duration <= 0 {
		t.Fatalf("route: %+v", out)
	}
}

func TestStory10_InvalidProvider(t *testing.T) {
	env := newStoryEnv(t, nil)
	rr := env.postMatrix(t, map[string]any{
		"points":     [][]float32{{116.40, 39.90}, {116.41, 39.91}},
		"provider":   "nonexistent",
		"coordinate": "gcj02",
	}, nil)
	if rr.Code == http.StatusOK {
		t.Fatal("expected error for unknown provider")
	}
}
