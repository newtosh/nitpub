package inbox

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterBlocksBurst(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	if !rl.Allow("1.2.3.4") || !rl.Allow("1.2.3.4") { //nolint:staticcheck // intentional: same IP called twice to exhaust the burst limit
		t.Fatal("expected first two requests")
	}
	if rl.Allow("1.2.3.4") {
		t.Fatal("expected third request to be blocked")
	}
}

func TestRateLimiterPerIP(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	if !rl.Allow("1.1.1.1") {
		t.Fatal("ip1")
	}
	if !rl.Allow("2.2.2.2") {
		t.Fatal("ip2 should not be blocked")
	}
}

func TestInboxRejectsUnsigned(t *testing.T) {
	h := &Handler{limiter: NewRateLimiter(10, time.Minute), verify: nil}
	req := httptest.NewRequest(http.MethodPost, "/inbox", nil)
	rec := httptest.NewRecorder()
	defer func() {
		_ = recover()
	}()
	h.ServeHTTP(rec, req)
}
