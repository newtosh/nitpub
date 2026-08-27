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

const heartbeatInterval = 7 * 24 * time.Hour

// IdentityStore is the subset of *store.Store telemetry needs. Accepting an
// interface keeps Start testable without a real bbolt file.
type IdentityStore interface {
	TelemetryEnabled() (bool, error)
	GetTelemetryIdentity() (store.TelemetryIdentity, bool, error)
}

// Start wires up the OTel meter provider and ships a startup ping plus a
// weekly heartbeat while ctx is live. If telemetry is disabled or no
// ingest endpoint is configured, Start is a no-op and returns a stop func
// that does nothing — telemetry must stay fully inert by default (R2).
//
// Reports instance ID, version, and OS/arch only. The plan's "feature
// flags in use" field (federation/moderation mode) was dropped: neither
// is a runtime config.Config value today (federation is install-scaffold
// only, moderation has no mode concept) — wiring one up would mean
// inventing config surface this plan doesn't otherwise need.
func Start(ctx context.Context, cfg config.Config, st IdentityStore) (stop func(), err error) {
	noop := func() {}

	enabled, err := st.TelemetryEnabled()
	if err != nil {
		return noop, err
	}
	if !enabled || cfg.TelemetryIngestURL == "" {
		return noop, nil
	}

	identity, ok, err := st.GetTelemetryIdentity()
	if err != nil {
		return noop, err
	}
	if !ok {
		// Enabled but never successfully registered — nothing to
		// authenticate exports with. Stay inert rather than sending
		// unauthenticated traffic.
		return noop, nil
	}

	archSuffix, err := updatecheck.ArchSuffix()
	if err != nil {
		archSuffix = "unknown"
	}

	exporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpointURL(cfg.TelemetryIngestURL),
		otlpmetrichttp.WithHeaders(map[string]string{
			"Authorization": "Bearer " + identity.Token,
		}),
	)
	if err != nil {
		return noop, err
	}

	reader := sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(heartbeatInterval))
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	meter := provider.Meter("nitpub/telemetry")
	attrs := attribute.NewSet(
		attribute.String("instance_id", identity.InstanceID),
		attribute.String("version", version.Version),
		attribute.String("os_arch", archSuffix),
	)
	_, err = meter.Int64ObservableGauge("nitpub.instance.info",
		metric.WithDescription("Reports 1 alongside instance version/arch/flags; the value itself carries no information."),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			o.Observe(1, metric.WithAttributeSet(attrs))
			return nil
		}),
	)
	if err != nil {
		return noop, err
	}

	// Startup ping: force an immediate export instead of waiting a full
	// heartbeatInterval for the first one.
	if err := reader.ForceFlush(ctx); err != nil {
		log.Printf("telemetry: startup export failed: %v", err)
	}

	stopCh := make(chan struct{})
	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stopCh:
				return
			case <-ticker.C:
				stillEnabled, err := st.TelemetryEnabled()
				if err != nil {
					log.Printf("telemetry: heartbeat check failed: %v", err)
					continue
				}
				if !stillEnabled {
					continue
				}
				if err := reader.ForceFlush(ctx); err != nil {
					log.Printf("telemetry: heartbeat export failed: %v", err)
				}
			}
		}
	}()

	return func() {
		close(stopCh)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := provider.Shutdown(shutdownCtx); err != nil {
			log.Printf("telemetry: shutdown: %v", err)
		}
	}, nil
}
