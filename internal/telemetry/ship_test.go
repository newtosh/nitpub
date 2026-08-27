package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/newtosh/nitpub/internal/config"
	"github.com/newtosh/nitpub/internal/store"
)

type fakeStore struct {
	enabled  bool
	identity store.TelemetryIdentity
	hasID    bool
}

func (f *fakeStore) TelemetryEnabled() (bool, error) { return f.enabled, nil }
func (f *fakeStore) GetTelemetryIdentity() (store.TelemetryIdentity, bool, error) {
	return f.identity, f.hasID, nil
}

func TestStartDisabledIsNoop(t *testing.T) {
	var hit atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit.Store(true)
	}))
	defer srv.Close()

	cfg := config.Config{TelemetryIngestURL: srv.URL}
	st := &fakeStore{enabled: false}

	stop, err := Start(context.Background(), cfg, st)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	stop()

	if hit.Load() {
		t.Fatal("expected no HTTP call when telemetry disabled")
	}
}

func TestStartEnabledWithoutIdentityIsNoop(t *testing.T) {
	var hit atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit.Store(true)
	}))
	defer srv.Close()

	cfg := config.Config{TelemetryIngestURL: srv.URL}
	st := &fakeStore{enabled: true, hasID: false}

	stop, err := Start(context.Background(), cfg, st)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	stop()

	if hit.Load() {
		t.Fatal("expected no HTTP call when instance never registered")
	}
}

func TestStartEnabledSendsStartupPingWithBearerToken(t *testing.T) {
	var gotAuth atomic.Value
	done := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		select {
		case done <- struct{}{}:
		default:
		}
	}))
	defer srv.Close()

	cfg := config.Config{TelemetryIngestURL: srv.URL}
	st := &fakeStore{
		enabled: true,
		hasID:   true,
		identity: store.TelemetryIdentity{
			InstanceID: "instance-1",
			Token:      "secret-token",
		},
	}

	stop, err := Start(context.Background(), cfg, st)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for startup export")
	}

	auth, _ := gotAuth.Load().(string)
	if auth != "Bearer secret-token" {
		t.Fatalf("Authorization header = %q, want Bearer secret-token", auth)
	}
}
