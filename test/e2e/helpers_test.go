package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"distance-matrix/internal/config"
	"distance-matrix/internal/handler"
	"distance-matrix/internal/cache"
	"distance-matrix/internal/engine"
	"distance-matrix/internal/planner"
	"distance-matrix/internal/provider"
	"distance-matrix/internal/svc"
	"distance-matrix/internal/testutil"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

var e2eMiniRedis *miniredis.Miniredis

type storyEnv struct {
	MR            *miniredis.Miniredis
	Stub          *testutil.StubProvider
	SvcCtx        *svc.ServiceContext
	matrixHandler http.HandlerFunc
	routeHandler  http.HandlerFunc
}

func newStoryEnv(t *testing.T, mutate func(*config.Config)) *storyEnv {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := loadE2EConfig(mr.Addr())
	if err != nil {
		mr.Close()
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(&cfg)
	}
	stub := &testutil.StubProvider{}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svcCtx := newStubServiceContext(cfg, stub, rdb)
	t.Cleanup(func() {
		_ = rdb.Close()
		mr.Close()
	})
	return &storyEnv{
		MR:            mr,
		Stub:          stub,
		SvcCtx:        svcCtx,
		matrixHandler: handler.MatrixHTTP(svcCtx),
		routeHandler:  handler.RouteHTTP(svcCtx),
	}
}

func (e *storyEnv) postMatrix(t *testing.T, body any, headers map[string]string) *httptest.ResponseRecorder {
	return e.postHandler(t, e.matrixHandler, body, headers)
}

func (e *storyEnv) postRoute(t *testing.T, body any, headers map[string]string) *httptest.ResponseRecorder {
	return e.postHandler(t, e.routeHandler, body, headers)
}

func (e *storyEnv) postHandler(t *testing.T, h http.HandlerFunc, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	h(rr, req)
	return rr
}

func parseMatrixBody(t *testing.T, rr *httptest.ResponseRecorder) matrixPayload {
	t.Helper()
	var out matrixPayload
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode matrix: %v body=%s", err, rr.Body.String())
	}
	return out
}

func redisKeysWithPrefix(mr *miniredis.Miniredis, prefix string) []string {
	var keys []string
	for _, k := range mr.Keys() {
		if len(prefix) == 0 || hasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

type matrixPayload struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Distances [][]float32 `json:"distances"`
		Durations [][]float32 `json:"durations"`
	} `json:"data"`
}

func assertSymmetricMatrix(t *testing.T, d [][]float32) {
	t.Helper()
	n := len(d)
	for i := 0; i < n; i++ {
		if len(d[i]) != n {
			t.Fatalf("row %d width %d != n %d", i, len(d[i]), n)
		}
		if d[i][i] != 0 {
			t.Fatalf("diagonal [%d][%d] should be 0, got %f", i, i, d[i][i])
		}
		for j := i + 1; j < n; j++ {
			if d[i][j] <= 0 || d[j][i] <= 0 {
				t.Fatalf("off-diagonal should be positive: [%d][%d]=%f [%d][%d]=%f", i, j, d[i][j], j, i, d[j][i])
			}
		}
	}
}

func stubDelay(stub *testutil.StubProvider, d time.Duration) { stub.Delay = d }

func newStubServiceContext(c config.Config, stub provider.Provider, rdb *redis.Client) *svc.ServiceContext {
	reg := testutil.NewRegistry(stub)
	batch := c.Providers.Amap.BatchSize
	if batch <= 0 {
		batch = 12
	}
	ctx := &svc.ServiceContext{
		Config:   c,
		Registry: reg,
		Planner:  planner.NewPlanner(batch),
		Matrix: &engine.Engine{
			Planner:   planner.NewPlanner(batch),
			Registry:  reg,
			MaxPoints: c.Engine.MaxPoints,
		},
	}
	if c.Redis.Enabled && rdb != nil {
		ctx.Redis = rdb
		ctx.Cache = cache.NewStore(rdb, c.Redis.Prefix, c.Redis.EdgeTTL)
		ctx.Matrix.Cache = ctx.Cache
	}
	return ctx
}
