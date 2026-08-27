package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/newtosh/nitpub/internal/config"
	"github.com/newtosh/nitpub/internal/store"
)

// fakeStore is mutex-guarded so tests can flip enabled/identity from the
// main goroutine while Start's resident poll loop reads them concurrently
// (see TestStartPicksUpRuntimeEnableWithoutRestart).
type fakeStore struct {
	mu       sync.Mutex
	enabled  bool
	identity store.TelemetryIdentity
	hasID    bool
}

func (f *fakeStore) TelemetryEnabled() (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.enabled, nil
}

func (f *fakeStore) GetTelemetryIdentity() (store.TelemetryIdentity, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.identity, f.hasID, nil
}

func (f *fakeStore) enableWithIdentity(identity store.TelemetryIdentity) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enabled = true
	f.identity = identity
	f.hasID = true
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

// Regression: Start used to check enabled/identity exactly once at boot
// and no-op forever if disabled then — so enabling telemetry later via
// the admin API/CLI never actually started shipping until a process
// restart. The resident poll loop must pick up the transition on its own.
func TestStartPicksUpRuntimeEnableWithoutRestart(t *testing.T) {
	origPoll := pollInterval
	pollInterval = 10 * time.Millisecond
	defer func() { pollInterval = origPoll }()

	var hits atomic.Int32
	done := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
		select {
		case done <- struct{}{}:
		default:
		}
	}))
	defer srv.Close()

	cfg := config.Config{TelemetryIngestURL: srv.URL}
	st := &fakeStore{enabled: false}

	stop, err := Start(context.Background(), cfg, st)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer stop()

	// Give the loop a couple of polls to confirm it really does nothing
	// while disabled, then flip the switch the way the admin handler
	// does after a successful registration.
	time.Sleep(30 * time.Millisecond)
	if hits.Load() != 0 {
		t.Fatal("expected no export while disabled")
	}

	st.enableWithIdentity(store.TelemetryIdentity{InstanceID: "instance-2", Token: "later-token"})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for export after runtime enable — Start did not react to the transition")
	}
}
