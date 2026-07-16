package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestE2E_HealthLive(t *testing.T) {
	resp, body := get(t, "/health/live")
	if resp.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Fatalf("live: status=%d body=%q", resp.StatusCode, body)
	}
}

func TestE2E_HealthReady(t *testing.T) {
	resp, body := get(t, "/health/ready")
	if resp.StatusCode != http.StatusOK || string(body) != "ready" {
		t.Fatalf("ready: status=%d body=%q", resp.StatusCode, body)
	}
}

func TestE2E_Providers(t *testing.T) {
	resp, raw := get(t, "/v1/providers")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, raw)
	}
	var out struct {
		Code int      `json:"code"`
		Data []string `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Code != 200 || len(out.Data) != 1 || out.Data[0] != "amap" {
		t.Fatalf("providers: %+v", out)
	}
}

func TestE2E_MatrixFallback(t *testing.T) {
	resp, raw := postJSON(t, "/v1/matrix", map[string]any{
		"points":     [][]float32{{116.40, 39.90}, {116.41, 39.91}},
		"coordinate": "gcj02",
		"strict":     true,
	}, map[string]string{"X-Tenant-Id": "e2e"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, raw)
	}
	var out struct {
		Code int `json:"code"`
		Data struct {
			Distances [][]float32 `json:"distances"`
			Durations [][]float32 `json:"durations"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Code != 200 {
		t.Fatalf("code=%d body=%s", out.Code, raw)
	}
	d := out.Data.Distances[0][1]
	if d <= 0 {
		t.Fatalf("expected positive fallback distance, got %f", d)
	}
	if out.Data.Distances[0][0] != 0 || out.Data.Durations[0][1] <= 0 {
		t.Fatalf("matrix shape invalid: d=%v t=%v", out.Data.Distances, out.Data.Durations)
	}
}

func TestE2E_MatrixCacheRetry(t *testing.T) {
	body := map[string]any{
		"points":     [][]float32{{116.50, 39.80}, {116.51, 39.81}},
		"coordinate": "gcj02",
		"strict":     true,
	}
	headers := map[string]string{"X-Tenant-Id": "e2e-cache"}

	resp1, raw1 := postJSON(t, "/v1/matrix", body, headers)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first: status=%d body=%s", resp1.StatusCode, raw1)
	}
	resp2, raw2 := postJSON(t, "/v1/matrix", body, headers)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("second: status=%d body=%s", resp2.StatusCode, raw2)
	}

	var a, b struct {
		Data struct {
			Distances [][]float32 `json:"distances"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw1, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw2, &b); err != nil {
		t.Fatal(err)
	}
	if a.Data.Distances[0][1] != b.Data.Distances[0][1] {
		t.Fatalf("retry distance mismatch: %f vs %f", a.Data.Distances[0][1], b.Data.Distances[0][1])
	}
}

func TestE2E_RouteFallback(t *testing.T) {
	resp, raw := postJSON(t, "/v1/route", map[string]any{
		"points":     [][]float32{{116.40, 39.90}, {116.41, 39.91}, {116.42, 39.92}},
		"coordinate": "gcj02",
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, raw)
	}
	var out struct {
		Code int `json:"code"`
		Data struct {
			Distance float32 `json:"distance"`
			Steps    []any   `json:"steps"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Code != 200 || len(out.Data.Steps) != 2 || out.Data.Distance <= 0 {
		t.Fatalf("route: %+v raw=%s", out, raw)
	}
}

func TestE2E_MatrixInvalidPoint(t *testing.T) {
	resp, _ := postJSON(t, "/v1/matrix", map[string]any{
		"points": [][]float32{{116.40}},
	}, nil)
	if resp.StatusCode == http.StatusOK {
		t.Fatal("expected error for invalid point")
	}
}

func TestE2E_TenantIsolation(t *testing.T) {
	body := map[string]any{
		"points":     [][]float32{{116.60, 39.70}, {116.61, 39.71}},
		"coordinate": "gcj02",
		"strict":     true,
	}
	_, _ = postJSON(t, "/v1/matrix", body, map[string]string{"X-Tenant-Id": "tenant-a"})
	_, _ = postJSON(t, "/v1/matrix", body, map[string]string{"X-Tenant-Id": "tenant-b"})
	resp, _ := postJSON(t, "/v1/matrix", body, map[string]string{"X-Tenant-Id": "tenant-a"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tenant-a retry failed: %d", resp.StatusCode)
	}
}
