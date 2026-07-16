package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTenantRateLimit(t *testing.T) {
	limit := TenantRateLimit(2)
	called := 0
	h := limit(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/matrix", nil)
	req.Header.Set("X-Tenant-Id", "tenant-x")

	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		h(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: status=%d", i+1, rr.Code)
		}
	}
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("third request should be 429, got %d body=%s", rr.Code, rr.Body.String())
	}
	if called != 2 {
		t.Fatalf("handler called %d times, want 2", called)
	}
}

func TestTenantRateLimitSeparateTenants(t *testing.T) {
	limit := TenantRateLimit(1)
	h := limit(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	reqA := httptest.NewRequest(http.MethodPost, "/v1/matrix", nil)
	reqA.Header.Set("X-Tenant-Id", "a")
	reqB := httptest.NewRequest(http.MethodPost, "/v1/matrix", nil)
	reqB.Header.Set("X-Tenant-Id", "b")

	rr := httptest.NewRecorder()
	h(rr, reqA)
	if rr.Code != http.StatusOK {
		t.Fatal("tenant a first")
	}
	rr = httptest.NewRecorder()
	h(rr, reqB)
	if rr.Code != http.StatusOK {
		t.Fatal("tenant b should have separate bucket")
	}
}
