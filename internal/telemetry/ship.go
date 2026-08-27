package telemetry

import (
	"context"
	"log"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/newtosh/nitpub/internal/config"
	"github.com/newtosh/nitpub/internal/store"
	"github.com/newtosh/nitpub/internal/updatecheck"
	"github.com/newtosh/nitpub/internal/version"
)

// heartbeatInterval is the minimum gap between two exports once telemetry
// is enabled and registered. pollInterval is how often the resident loop
// rechecks enabled/identity state — independent of heartbeatInterval so
// that an admin enabling telemetry after startup gets picked up promptly
// (within one pollInterval) instead of only on the next process restart.
// Both are vars, not consts, so tests can shrink them.
var (
	heartbeatInterval = 7 * 24 * time.Hour
	pollInterval      = 5 * time.Minute
)

// IdentityStore is the subset of *store.Store telemetry needs. Accepting an
// interface keeps Start testable without a real bbolt file.
type IdentityStore interface {
	TelemetryEnabled() (bool, error)
	GetTelemetryIdentity() (store.TelemetryIdentity, bool, error)
}

// Start runs a resident loop for the life of ctx that ships a heartbeat
// export whenever telemetry is enabled and registered, and stays fully
// inert (no network calls) otherwise. If no ingest endpoint is
// configured, Start is a no-op and returns a stop func that does
// nothing — there is nowhere to ever send data (R2).
//
// The loop reacts to enable/disable transitions without a process
// restart: it lazily builds the OTel exporter/reader the moment telemetry
// becomes enabled+registered (sending immediately, same as the old
// startup ping), and tears it down again when disabled. This matters
// because the admin API/CLI toggle mutates store state at runtime — a
// Start that only checked once at boot would leave "enable via admin
// while the server never restarts" permanently inert.
//
// Reports instance ID, version, and OS/arch only. The plan's "feature
// flags in use" field (federation/moderation mode) was dropped: neither
// is a runtime config.Config value today (federation is install-scaffold
// only, moderation has no mode concept) — wiring one up would mean
// inventing config surface this plan doesn't otherwise need.
func Start(ctx context.Context, cfg config.Config, st IdentityStore) (stop func(), err error) {
	noop := func() {}
	if cfg.TelemetryIngestURL == "" {
		return noop, nil
	}

	archSuffix, err := updatecheck.ArchSuffix()
	if err != nil {
		archSuffix = "unknown"
	}

	l := &loop{
		cfg:        cfg,
		st:         st,
		archSuffix: archSuffix,
	}

	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		defer l.shutdown()
		l.check(ctx)
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stopCh:
				return
			case <-ticker.C:
				l.check(ctx)
			}
		}
	}()

	// stop blocks until the loop goroutine has actually exited (and shut
	// down its exporter), not just signaled — callers (server.Close(),
	// tests) rely on "stopped" meaning no more in-flight network calls.
	return func() {
		close(stopCh)
		<-doneCh
	}, nil
}

// loop holds the lazily-built OTel plumbing for one telemetry session.
// Rebuilt whenever the registered token changes (rare — tokens don't
// rotate today, but this stays correct if that ever changes); torn down
// whenever telemetry is disabled.
type loop struct {
	cfg        config.Config
	st         IdentityStore
	archSuffix string

	provider *sdkmetric.MeterProvider
	reader   *sdkmetric.PeriodicReader
	token    string
	lastSent time.Time
}

func (l *loop) check(ctx context.Context) {
	enabled, err := l.st.TelemetryEnabled()
	if err != nil {
		log.Printf("telemetry: status check failed: %v", err)
		return
	}
	if !enabled {
		l.shutdown()
		return
	}

	identity, ok, err := l.st.GetTelemetryIdentity()
	if err != nil {
		log.Printf("telemetry: identity check failed: %v", err)
		return
	}
	if !ok {
		// Enabled but never successfully registered — nothing to
		// authenticate exports with. Stay inert rather than sending
		// unauthenticated traffic.
		l.shutdown()
		return
	}

	if l.provider == nil || l.token != identity.Token {
		l.shutdown()
		if err := l.build(ctx, identity); err != nil {
			log.Printf("telemetry: build exporter failed: %v", err)
			return
		}
		l.lastSent = time.Time{}
	}

	if !l.lastSent.IsZero() && time.Since(l.lastSent) < heartbeatInterval {
		return
	}
	if err := l.reader.ForceFlush(ctx); err != nil {
		log.Printf("telemetry: export failed: %v", err)
		return
	}
	l.lastSent = time.Now()
}

func (l *loop) build(ctx context.Context, identity store.TelemetryIdentity) error {
	exporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpointURL(l.cfg.TelemetryIngestURL),
		otlpmetrichttp.WithHeaders(map[string]string{
			"Authorization": "Bearer " + identity.Token,
		}),
	)
	if err != nil {
		return err
	}

	reader := sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(heartbeatInterval))
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	meter := provider.Meter("nitpub/telemetry")
	attrs := attribute.NewSet(
		attribute.String("instance_id", identity.InstanceID),
		attribute.String("version", version.Version),
		attribute.String("os_arch", l.archSuffix),
	)
	_, err = meter.Int64ObservableGauge("nitpub.instance.info",
		metric.WithDescription("Reports 1 alongside instance version and os_arch; the value itself carries no information."),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			o.Observe(1, metric.WithAttributeSet(attrs))
			return nil
		}),
	)
	if err != nil {
		_ = provider.Shutdown(ctx)
		return err
	}

	l.provider = provider
	l.reader = reader
	l.token = identity.Token
	return nil
}

func (l *loop) shutdown() {
	if l.provider == nil {
		return
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := l.provider.Shutdown(shutdownCtx); err != nil {
		log.Printf("telemetry: shutdown: %v", err)
	}
	l.provider = nil
	l.reader = nil
	l.token = ""
}
