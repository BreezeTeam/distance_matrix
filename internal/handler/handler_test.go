package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"distance-matrix/internal/config"
	"distance-matrix/internal/engine"
	"distance-matrix/internal/planner"
	"distance-matrix/internal/svc"
	"distance-matrix/internal/testutil"
	"distance-matrix/internal/types"
	"github.com/zeromicro/go-zero/rest"
)

func newTestServiceContext(t *testing.T, stub *testutil.StubProvider) *svc.ServiceContext {
	t.Helper()
	_, store := testutil.SetupRedis(t)
	reg := testutil.NewRegistry(stub)
	return &svc.ServiceContext{
		Config: config.Config{
			RestConf: rest.RestConf{Timeout: 30000},
			Redis:    config.RedisConf{Enabled: true},
			Engine:   config.EngineConf{DefaultGeoWideM: 200, MaxPoints: 100, TenantQPS: 50},
		},
		Cache:    store,
		Registry: reg,
		Planner:  planner.NewPlanner(12),
		Matrix: &engine.Engine{
			Cache:     store,
			Planner:   planner.NewPlanner(12),
			Registry:  reg,
			MaxPoints: 100,
		},
	}
}

func TestMatrixHandlerOK(t *testing.T) {
	svcCtx := newTestServiceContext(t, &testutil.StubProvider{})
	body := `{"points":[[116.40,39.90],[116.41,39.91]],"coordinate":"gcj02"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/matrix", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	matrixHandler(svcCtx)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp types.MatrixResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 200 || resp.Data == nil {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Data.Distances[0][1] != 1000 {
		t.Fatalf("distance=%v", resp.Data.Distances)
	}
}

func TestMatrixHandlerInvalidPoint(t *testing.T) {
	svcCtx := newTestServiceContext(t, &testutil.StubProvider{})
	body := `{"points":[[116.40]]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/matrix", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	matrixHandler(svcCtx)(rr, req)
	if rr.Code == http.StatusOK {
		t.Fatalf("expected error status, got 200 body=%s", rr.Body.String())
	}
}

func TestMatrixHandlerDeadline504(t *testing.T) {
	stub := &testutil.StubProvider{Delay: 300 * time.Millisecond}
	svcCtx := newTestServiceContext(t, stub)
	svcCtx.Config.Timeout = 50

	body := `{"points":[[116.40,39.90],[116.41,39.91],[116.42,39.92],[116.43,39.93]]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/matrix", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	matrixHandler(svcCtx)(rr, req)
	if rr.Code != http.StatusGatewayTimeout {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp types.MatrixResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 504 || resp.Msg != "MATRIX_DEADLINE" {
		t.Fatalf("unexpected 504 payload: %+v", resp)
	}
}

func TestMatrixHandlerCacheRetry(t *testing.T) {
	stub := &testutil.StubProvider{}
	svcCtx := newTestServiceContext(t, stub)
	body := `{"points":[[116.40,39.90],[116.41,39.91]],"strict":true}`
	do := func() int32 {
		req := httptest.NewRequest(http.MethodPost, "/v1/matrix", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		matrixHandler(svcCtx)(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
		}
		return stub.Calls.Load()
	}
	callsFirst := do()
	callsSecond := do()
	if callsFirst == 0 {
		t.Fatal("first request should call provider")
	}
	if callsSecond != callsFirst {
		t.Fatalf("second request should hit cache: calls first=%d second=%d", callsFirst, callsSecond)
	}
}

func TestRouteHandlerOK(t *testing.T) {
	svcCtx := newTestServiceContext(t, &testutil.StubProvider{})
	body := `{"points":[[116.40,39.90],[116.41,39.91],[116.42,39.92]]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/route", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	routeHandler(svcCtx)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp types.RouteResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data == nil || len(resp.Data.Steps) != 2 {
		t.Fatalf("unexpected route: %+v", resp)
	}
}

func TestRouteHandlerTooFewPoints(t *testing.T) {
	svcCtx := newTestServiceContext(t, &testutil.StubProvider{})
	body := `{"points":[[116.40,39.90]]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/route", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	routeHandler(svcCtx)(rr, req)
	if rr.Code == http.StatusOK {
		t.Fatal("expected error for single point")
	}
}

func TestProvidersHandler(t *testing.T) {
	svcCtx := newTestServiceContext(t, &testutil.StubProvider{})
	req := httptest.NewRequest(http.MethodGet, "/v1/providers", nil)
	rr := httptest.NewRecorder()

	providersHandler(svcCtx)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	var resp types.ProvidersResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 1 || resp.Data[0] != "amap" {
		t.Fatalf("providers=%v", resp.Data)
	}
}

func TestHealthHandlers(t *testing.T) {
	svcCtx := newTestServiceContext(t, &testutil.StubProvider{})

	liveReq := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	liveRR := httptest.NewRecorder()
	healthLiveHandler(svcCtx)(liveRR, liveReq)
	if liveRR.Code != http.StatusOK || liveRR.Body.String() != "ok" {
		t.Fatalf("live: code=%d body=%q", liveRR.Code, liveRR.Body.String())
	}

	readyReq := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	readyRR := httptest.NewRecorder()
	healthReadyHandler(svcCtx)(readyRR, readyReq)
	if readyRR.Code != http.StatusOK {
		t.Fatalf("ready: code=%d body=%q", readyRR.Code, readyRR.Body.String())
	}
}

func TestHealthReadyWithoutCacheWhenRedisRequired(t *testing.T) {
	stub := &testutil.StubProvider{}
	reg := testutil.NewRegistry(stub)
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Redis: config.RedisConf{Enabled: true},
		},
		Registry: reg,
		Matrix: &engine.Engine{
			Planner:  planner.NewPlanner(12),
			Registry: reg,
			MaxPoints: 0,
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rr := httptest.NewRecorder()
	healthReadyHandler(svcCtx)(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when redis enabled but cache nil, got %d", rr.Code)
	}
}

func TestTenantIDHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if tenantID(req) != "default" {
		t.Fatal("default tenant")
	}
	req.Header.Set("X-Tenant-Id", "acme")
	if tenantID(req) != "acme" {
		t.Fatal("header tenant")
	}
}
