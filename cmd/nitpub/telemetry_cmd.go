package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/newtosh/nitpub/internal/config"
	"github.com/newtosh/nitpub/internal/store"
	"github.com/newtosh/nitpub/internal/telemetry"
)

// telemetryEnv mirrors adminEnv's offline-store-access pattern (see
// openAdminEnv in admin_cmd.go, both built on openOfflineStore in
// service.go): the bbolt database is exclusively locked while
// nitpub.service runs, so mutating telemetry state needs the same
// --offline stop/restart dance admin commands use.
type telemetryEnv struct {
	st      *store.Store
	cfg     config.Config
	cleanup func()
}

func openTelemetryEnv(offline bool) (*telemetryEnv, error) {
	st, cfg, cleanup, err := openOfflineStore(offline, "telemetry")
	if err != nil {
		return nil, err
	}
	return &telemetryEnv{st: st, cfg: cfg, cleanup: cleanup}, nil
}

func withTelemetryEnv(offline *bool, fn func(*telemetryEnv) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		env, err := openTelemetryEnv(*offline)
		if err != nil {
			return err
		}
		defer env.cleanup()
		return fn(env)
	}
}

func newTelemetryCmd() *cobra.Command {
	var offline bool

	root := &cobra.Command{
		Use:   "telemetry",
		Short: "Manage opt-in version telemetry",
		Long: `Enable, disable, or check the status of opt-in version telemetry.

Telemetry commands open the bbolt database directly. While nitpub.service is
running it holds an exclusive lock, so commands fail fast unless you pass
--offline (which stops the service, runs the command, then starts it again).`,
	}
	root.PersistentFlags().BoolVar(&offline, "offline", false,
		"stop nitpub.service before running, then start it again (requires root)")

	root.AddCommand(newTelemetryEnableCmd(&offline))
	root.AddCommand(newTelemetryDisableCmd(&offline))
	root.AddCommand(newTelemetryStatusCmd(&offline))
	return root
}

func newTelemetryEnableCmd(offline *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "enable",
		Short: "Opt this instance into version telemetry",
		Long: `Registers this instance (if not already registered) and turns
telemetry on. No-ops with a clear error if telemetry_register_url is not
configured — there is nowhere to register against.`,
		RunE: withTelemetryEnv(offline, func(env *telemetryEnv) error {
			if env.cfg.TelemetryRegisterURL == "" {
				return fmt.Errorf("telemetry_register_url is not configured — nothing to register against")
			}

			if _, ok, err := env.st.GetTelemetryIdentity(); err != nil {
				return err
			} else if !ok {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				id, token, err := telemetry.Register(ctx, env.cfg.TelemetryRegisterURL)
				if err != nil {
					return fmt.Errorf("registration failed, telemetry left disabled: %w", err)
				}
				if err := env.st.SetTelemetryIdentity(id, token); err != nil {
					return err
				}
			}

			if err := env.st.SetTelemetryEnabled(true); err != nil {
				return err
			}
			fmt.Println("telemetry enabled")
			return nil
		}),
	}
}

func newTelemetryDisableCmd(offline *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Opt this instance out of version telemetry",
		RunE: withTelemetryEnv(offline, func(env *telemetryEnv) error {
			if err := env.st.SetTelemetryEnabled(false); err != nil {
				return err
			}
			fmt.Println("telemetry disabled")
			return nil
		}),
	}
}

func newTelemetryStatusCmd(offline *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show telemetry opt-in status",
		RunE: withTelemetryEnv(offline, func(env *telemetryEnv) error {
			enabled, err := env.st.TelemetryEnabled()
			if err != nil {
				return err
			}
			_, registered, err := env.st.GetTelemetryIdentity()
			if err != nil {
				return err
			}
			available := env.cfg.TelemetryRegisterURL != "" && env.cfg.TelemetryIngestURL != ""
			fmt.Printf("enabled: %v\nregistered: %v\navailable (endpoints configured): %v\n", enabled, registered, available)
			return nil
		}),
	}
}
