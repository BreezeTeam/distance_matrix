//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"distance-matrix/internal/config"
	"github.com/zeromicro/go-zero/core/conf"
)

// TestLiveAmapKeys probes each key in etc/matrix.yaml against Amap driving API.
// Run: go test ./test/e2e/ -tags=e2e -run TestLiveAmapKeys -v
func TestLiveAmapKeys(t *testing.T) {
	if os.Getenv("AMAP_KEY_CHECK") == "skip" {
		t.Skip("AMAP_KEY_CHECK=skip")
	}

	keys := loadProductionKeys(t)
	if len(keys) == 0 {
		t.Fatal("no amap keys in etc/matrix.yaml")
	}

	client := &http.Client{Timeout: 5 * time.Second}
	var ok, bad int
	for i, key := range keys {
		status, info, code := probeAmapKey(client, key)
		t.Logf("key[%d] %s… status=%s infocode=%s info=%s", i+1, key[:8], status, code, info)
		switch status {
		case "1":
			ok++
		default:
			bad++
		}
	}
	t.Logf("summary: %d ok, %d failed of %d keys", ok, bad, len(keys))
	if ok == 0 {
		t.Fatal("no working amap keys — service will always use haversine fallback")
	}
}

// TestLiveMatrixWithProductionConfig hits a running server (default localhost:8888) with real Amap.
// Requires: docker compose up / go run matrix.go
// Run: go test ./test/e2e/ -tags=e2e -run TestLiveMatrixWithProductionConfig -v
func TestLiveMatrixWithProductionConfig(t *testing.T) {
	base := os.Getenv("E2E_BASE_URL")
	if base == "" {
		base = "http://127.0.0.1:8888"
	}
	if !serverUp(base) {
		t.Skipf("server not running at %s (start with: go run matrix.go -f etc/matrix.yaml)", base)
	}

	body := `{"points":[[116.397428,39.90923],[116.407428,39.91923]],"coordinate":"gcj02","strict":true}`
	req, err := http.NewRequest(http.MethodPost, base+"/v1/matrix", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-Id", "live-e2e")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, raw)
	}

	var out struct {
		Code int `json:"code"`
		Data struct {
			Distances [][]float32 `json:"distances"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	d := out.Data.Distances[0][1]
	// Real Amap driving ~1.9km for this pair; haversine fallback ~1.4km with 1.5 factor ~2.1km
	if d < 500 || d > 5000 {
		t.Fatalf("unexpected live distance %f (amap or fallback broken?)", d)
	}
	t.Logf("live matrix distance[0][1]=%.0fm", d)

	// Retry should succeed (cache write-through)
	resp2, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("retry status=%d", resp2.StatusCode)
	}
}

func loadProductionKeys(t *testing.T) []string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	path := filepath.Join(root, "etc", "matrix.yaml")
	var cfg config.Config
	if err := conf.Load(path, &cfg); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, k := range strings.Split(cfg.Providers.Amap.Keys, ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			keys = append(keys, k)
		}
	}
	return keys
}

func probeAmapKey(client *http.Client, key string) (status, info, infocode string) {
	q := url.Values{}
	q.Set("key", key)
	q.Set("origin", "116.397428,39.90923")
	q.Set("destination", "116.407428,39.91923")
	q.Set("strategy", "11")
	q.Set("output", "json")
	u := fmt.Sprintf("http://restapi.amap.com/v3/direction/driving?%s", q.Encode())
	resp, err := client.Get(u)
	if err != nil {
		return "0", err.Error(), "network"
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var v struct {
		Status   string `json:"status"`
		Info     string `json:"info"`
		Infocode string `json:"infocode"`
	}
	_ = json.Unmarshal(raw, &v)
	return v.Status, v.Info, v.Infocode
}

func serverUp(base string) bool {
	resp, err := http.Get(base + "/health/live")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
