package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAmapNotReadyWithoutKeys(t *testing.T) {
	p := NewAmapProvider(AmapConfig{Enabled: true, Keys: ""})
	if p.Ready() {
		t.Fatal("should not be ready without keys")
	}
}

func TestAmapFallbackRoute(t *testing.T) {
	p := NewAmapProvider(AmapConfig{Enabled: false})
	wps := [][2]float32{{116.40, 39.90}, {116.41, 39.91}}
	res, err := p.Route(context.Background(), RouteRequest{Waypoints: wps, SpeedMPS: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(res.Steps))
	}
	if res.Steps[0].Source != SourceFallback {
		t.Fatalf("expected fallback source, got %q", res.Steps[0].Source)
	}
	if res.DistanceM <= 0 || res.DurationS <= 0 {
		t.Fatalf("fallback should produce positive distance/duration: d=%f t=%f", res.DistanceM, res.DurationS)
	}
	if !res.Degraded {
		t.Fatal("fallback route should be degraded")
	}
}

func TestAmapStrategyCode(t *testing.T) {
	cases := map[int]string{
		0:  "11",
		1:  "12",
		2:  "1",
		3:  "2",
		99: "11",
	}
	for in, want := range cases {
		if got := strategyCode(in); got != want {
			t.Fatalf("strategyCode(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestAmapRouteTriesNextKeyOnFailure(t *testing.T) {
	okBody, _ := json.Marshal(map[string]any{
		"status": "1",
		"route": map[string]any{
			"paths": []map[string]any{{
				"distance": "1000",
				"duration": "120",
				"steps":    []map[string]any{{"distance": "1000", "duration": "120"}},
			}},
		},
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		switch key {
		case "bad-key":
			_, _ = w.Write([]byte(`{"status":"0","info":"INVALID_USER_IP","infocode":"10005"}`))
		case "good-key":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(okBody)
		default:
			http.Error(w, "unknown key", http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	p := NewAmapProvider(AmapConfig{
		Enabled: true,
		Keys:    "bad-key,good-key",
		BaseURL: srv.URL,
	})
	wps := [][2]float32{{116.40, 39.90}, {116.41, 39.91}}
	res, err := p.Route(context.Background(), RouteRequest{Waypoints: wps})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != SourceProvider {
		t.Fatalf("expected provider result after key failover, got %q degraded=%v", res.Source, res.Degraded)
	}
	if res.DistanceM != 1000 {
		t.Fatalf("distance=%f", res.DistanceM)
	}
	if p.lb.SuccessRate("good-key") <= 0 {
		t.Fatal("successful key should be recorded")
	}
}

func TestAmapRouteFallbackWhenAllKeysFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"0","info":"USER_KEY_RECYCLED","infocode":"10013"}`))
	}))
	defer srv.Close()

	p := NewAmapProvider(AmapConfig{
		Enabled: true,
		Keys:    "k1,k2",
		BaseURL: srv.URL,
	})
	wps := [][2]float32{{116.40, 39.90}, {116.41, 39.91}}
	res, err := p.Route(context.Background(), RouteRequest{Waypoints: wps})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != SourceFallback {
		t.Fatalf("expected haversine fallback when all keys fail, got %q", res.Source)
	}
}

func TestAmapDeadKeyExcludedAfterFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "dead" {
			_, _ = w.Write([]byte(`{"status":"0","info":"USER_KEY_RECYCLED","infocode":"10013"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"1","route":{"paths":[{"distance":"100","duration":"10","steps":[{"distance":"100","duration":"10"}]}]}}`))
	}))
	defer srv.Close()

	p := NewAmapProvider(AmapConfig{Enabled: true, Keys: "dead,good", BaseURL: srv.URL})
	wps := [][2]float32{{116.40, 39.90}, {116.41, 39.91}}
	for i := 0; i < 5; i++ {
		_, _ = p.Route(context.Background(), RouteRequest{Waypoints: wps})
	}
	for i := 0; i < 5; i++ {
		p.lb.RecordFailure("dead")
	}
	for i := 0; i < 10; i++ {
		p.lb.RecordSuccess("good")
	}
	share := map[string]int{}
	for i := 0; i < 500; i++ {
		k, _ := p.lb.Next().(string)
		share[k]++
	}
	if share["dead"] < 1 || share["dead"] > 20 {
		t.Fatalf("dead key should get minimal probe share (~2%%), got %v", share)
	}
	if share["good"] < 400 {
		t.Fatalf("good key should dominate: %v", share)
	}
}

func TestAmapFailureReducesKeyShare(t *testing.T) {
	p := NewAmapProvider(AmapConfig{Enabled: true, Keys: "a,b,c"})
	for i := 0; i < 5; i++ {
		p.lb.RecordFailure("b")
	}
	for i := 0; i < 10; i++ {
		p.lb.RecordSuccess("a")
	}
	weights := make(map[string]int)
	for i := 0; i < 500; i++ {
		k, _ := p.lb.Next().(string)
		if k != "" {
			weights[k]++
		}
	}
	if weights["b"] < 1 || weights["b"] > 20 {
		t.Fatalf("always-failing b should get minimal probe share: %v", weights)
	}
}

func TestAmapAllKeysParsed(t *testing.T) {
	p := NewAmapProvider(AmapConfig{Enabled: true, Keys: " a , b , c "})
	got := strings.Join(p.allKeys(), ",")
	// map iteration order varies; sort for compare
	if len(p.allKeys()) != 3 {
		t.Fatalf("got keys %v", p.allKeys())
	}
	_ = got
}
