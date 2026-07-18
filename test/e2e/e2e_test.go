package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"distance-matrix/internal/config"
	"distance-matrix/internal/handler"
	"distance-matrix/internal/svc"
	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var (
	baseURL    string
	httpClient = &http.Client{Timeout: 15 * time.Second}
)

func TestMain(m *testing.M) {
	mr, err := miniredis.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "miniredis: %v\n", err)
		os.Exit(1)
	}

	cfg, err := loadE2EConfig(mr.Addr())
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	port, err := freePort()
	if err != nil {
		fmt.Fprintf(os.Stderr, "port: %v\n", err)
		os.Exit(1)
	}
	cfg.Host = "127.0.0.1"
	cfg.Port = port

	svcCfg, restConf := config.ForServer(cfg)
	svcCtx := svc.NewServiceContext(svcCfg)
	server := rest.MustNewServer(restConf)
	handler.RegisterHandlers(server, svcCtx)

	done := make(chan struct{})
	go func() {
		server.Start()
		close(done)
	}()
	defer func() {
		server.Stop()
		<-done
	}()

	baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := waitLive(baseURL, 5*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "live: %v\n", err)
		os.Exit(1)
	}

	e2eMiniRedis = mr
	code := m.Run()
	mr.Close()
	os.Exit(code)
}

func loadE2EConfig(redisAddr string) (config.Config, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return config.Config{}, fmt.Errorf("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "config.yaml")
	var cfg config.Config
	if err := conf.Load(path, &cfg); err != nil {
		return config.Config{}, err
	}
	cfg.Redis.Addr = redisAddr
	cfg.Redis.Enabled = true
	return cfg, nil
}

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port, nil
}

func waitLive(base string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/health/live")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s/health/live", base)
}

func postJSON(t *testing.T, path string, body any, headers map[string]string) (*http.Response, []byte) {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	return resp, raw
}

func get(t *testing.T, path string) (*http.Response, []byte) {
	t.Helper()
	resp, err := httpClient.Get(baseURL + path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	return resp, raw
}
