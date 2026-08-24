package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/client"
	"github.com/spf13/cobra"

	"github.com/newtosh/nitpub/internal/actor"
	"github.com/newtosh/nitpub/internal/apstore"
	"github.com/newtosh/nitpub/internal/config"
	"github.com/newtosh/nitpub/internal/federation"
	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/store"
)

func newFederationCmd() *cobra.Command {
	var offline bool

	fed := &cobra.Command{
		Use:   "federation",
		Short: "Federation maintenance",
		Long: `Deliver or re-deliver ActivityPub activities without the admin HTTP API.

These commands open the database directly. While nitpub is running, pass --offline
(as root) to stop the service, run the command, then start it again.`,
	}

	fed.PersistentFlags().BoolVar(&offline, "offline", false,
		"stop nitpub.service before running, then start it again (requires root)")

	fed.AddCommand(&cobra.Command{
		Use:   "redeliver-shared",
		Short: "Re-send already-federated posts to followers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFederationRedeliverShared(offline)
		},
	})

	fed.AddCommand(&cobra.Command{
		Use:   "backfill",
		Short: "Deliver never-shared posts to followers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFederationBackfill(offline)
		},
	})

	return fed
}

type federationEnv struct {
	cleanup func()
	ob      *outbox.Service
	deliver func(activity any) error
}

func openFederationEnv(offline bool) (*federationEnv, error) {
	env := &federationEnv{cleanup: func() {}}
	if offline {
		stopped, err := stopNitpubService()
		if err != nil {
			return nil, err
		}
		if stopped {
			env.cleanup = func() {
				_ = startNitpubService()
			}
		}
	}

	cfg, err := config.Load()
	if err != nil {
		env.cleanup()
		return nil, err
	}
	if err := config.EnsureDataDirWritable(cfg); err != nil {
		env.cleanup()
		return nil, err
	}

	timeout := 3 * time.Second
	if offline {
		timeout = 0
	}
	st, err := store.OpenWithTimeout(cfg.DataDir, timeout)
	if err != nil {
		env.cleanup()
		if errors.Is(err, store.ErrDatabaseLocked) {
			return nil, fmt.Errorf("%w\n\nhint: nitpub federation … --offline   (as root, stops the service)\n      systemctl stop nitpub && sudo -u nitpub nitpub federation …", err)
		}
		return nil, err
	}

	prev := env.cleanup
	env.cleanup = func() {
		_ = st.Close()
		prev()
	}

	actorIRI := vocab.IRI(cfg.BaseURL + "/actor")
	ap := apstore.New(st, actorIRI)
	actSvc, err := actor.LoadOrCreate(cfg, ap, nil)
	if err != nil {
		env.cleanup()
		return nil, err
	}

	ob := outbox.New(st, cfg.BaseURL, string(actorIRI))
	cl := client.New()
	signer := federation.NewSigner(actSvc.Actor(), actSvc.PrivateKey())
	env.ob = ob
	env.deliver = func(activity any) error {
		followers, err := ap.ListFollowers()
		if err != nil {
			return err
		}
		return federation.DeliverToInboxes(cl, signer, apstore.UniqueDeliveryInboxes(followers), activity)
	}
	return env, nil
}

func runFederationRedeliverShared(offline bool) error {
	env, err := openFederationEnv(offline)
	if err != nil {
		return err
	}
	defer env.cleanup()

	result, err := env.ob.RedeliverSharedPosts(env.deliver)
	if err != nil {
		return err
	}
	return encodeFederationResult(result)
}

func runFederationBackfill(offline bool) error {
	env, err := openFederationEnv(offline)
	if err != nil {
		return err
	}
	defer env.cleanup()

	result, err := env.ob.BackfillFederation(env.deliver)
	if err != nil {
		return err
	}
	return encodeFederationResult(result)
}

func encodeFederationResult(result outbox.BackfillResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
