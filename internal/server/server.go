package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/client"

	"github.com/newtosh/nitpub/internal/actor"
	"github.com/newtosh/nitpub/internal/analytics"
	"github.com/newtosh/nitpub/internal/api"
	"github.com/newtosh/nitpub/internal/apstore"
	"github.com/newtosh/nitpub/internal/auth"
	"github.com/newtosh/nitpub/internal/commentauth"
	"github.com/newtosh/nitpub/internal/config"
	"github.com/newtosh/nitpub/internal/federation"
	"github.com/newtosh/nitpub/internal/icons"
	"github.com/newtosh/nitpub/internal/inbox"
	"github.com/newtosh/nitpub/internal/mastodon"
	"github.com/newtosh/nitpub/internal/media"
	"github.com/newtosh/nitpub/internal/moderation"
	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/search"
	"github.com/newtosh/nitpub/internal/sitecontent"
	"github.com/newtosh/nitpub/internal/store"
	"github.com/newtosh/nitpub/internal/telemetry"
)

// Server wires nitpub HTTP routes.
type Server struct {
	cfg           config.Config
	mux           *http.ServeMux
	actor         *actor.Service
	outbox        *outbox.Service
	cancel        context.CancelFunc
	telemetryStop func()
}

// New constructs the application server.
func New(ctx context.Context, cfg config.Config, st *store.Store, static fs.FS) (*Server, error) {
	workerCtx, cancel := context.WithCancel(ctx)
	actorIRI := vocab.IRI(cfg.BaseURL + "/actor")
	ap := apstore.New(st, actorIRI)

	siteSvc, err := sitecontent.New(cfg.DataDir)
	if err != nil {
		cancel()
		return nil, err
	}

	actSvc, err := actor.LoadOrCreate(cfg, ap, siteSvc)
	if err != nil {
		cancel()
		return nil, err
	}
	ob := outbox.New(st, cfg.BaseURL, string(actorIRI))
	if n, err := ob.RewritePostBaseURLs(); err != nil {
		cancel()
		return nil, err
	} else if n > 0 {
		log.Printf("migrated %d post(s) to base URL %s", n, cfg.BaseURL)
	}
	cl := client.New()
	verify := federation.NewVerifier(ap, cl)
	signer := federation.NewSigner(actSvc.Actor(), actSvc.PrivateKey())

	deliverOne := func(inboxURL string, activity any) error {
		return federation.Deliver(cl, signer, inboxURL, activity)
	}

	deliverFollowers := func(activity any) error {
		followers, err := ap.ListFollowers()
		if err != nil {
			return err
		}
		return federation.DeliverToInboxes(cl, signer, apstore.UniqueDeliveryInboxes(followers), activity)
	}

	federation.StartKeyFetchWorker(workerCtx, ap, cl)

	telemetryStop, err := telemetry.Start(workerCtx, cfg, st)
	if err != nil {
		log.Printf("telemetry: startup failed, continuing without it: %v", err)
		telemetryStop = func() {}
	}

	mediaSvc, err := media.New(cfg.DataDir)
	if err != nil {
		cancel()
		return nil, err
	}

	searchIdx := search.NewIndex()
	rebuildSearch := func() {
		posts, err := ob.ListPosts()
		if err != nil {
			log.Printf("search rebuild: list posts: %v", err)
			return
		}
		pages, err := siteSvc.MarkdownPagesForSearch()
		if err != nil {
			log.Printf("search rebuild: load pages: %v", err)
			return
		}
		searchIdx.Rebuild(posts, pages)
	}
	rebuildSearch()

	authSvc, err := auth.NewService(st, cfg.Domain, "nitpub")
	if err != nil {
		cancel()
		return nil, err
	}
	authSvc.SetRPOrigin(cfg.BaseURL)
	if cfg.SessionCookieDomain != "" {
		authSvc.SetCookieDomain(cfg.SessionCookieDomain)
	}

	exists, err := authSvc.Store().AdminExists()
	if err != nil {
		cancel()
		return nil, err
	}
	if !exists {
		log.Printf("WARNING: no admin account configured — run `nitpub admin init` before using the admin UI")
	}

	authAPI := api.NewAuth(authSvc)
	resendAccepts := func() (int, error) {
		return federation.ResendAccepts(ap, string(actorIRI), cfg.BaseURL, deliverOne)
	}

	backfillFederation := func() (outbox.BackfillResult, error) {
		return ob.BackfillFederation(deliverFollowers)
	}

	redeliverShared := func() (outbox.BackfillResult, error) {
		return ob.RedeliverSharedPosts(deliverFollowers)
	}

	modSvc := moderation.New(st)
	postExists := func(slug string) bool {
		_, err := ob.GetPost(slug)
		return err == nil
	}
	if err := modSvc.RunBackfillOnce(postExists); err != nil {
		log.Printf("moderation: backfill: %v", err)
	}

	apiHandler := api.NewHandler(ob, authAPI, mediaSvc, siteSvc, searchIdx, rebuildSearch, deliverFollowers, resendAccepts, backfillFederation, redeliverShared, cfg.Domain, cfg.BaseURL, cfg.Actor, func() int {
		followers, err := ap.ListFollowers()
		if err != nil {
			return 0
		}
		return len(followers)
	}, modSvc, cfg.Title, cfg.AnalyticsEnabled)

	iconsSvc, err := icons.New(cfg.DataDir)
	if err != nil {
		cancel()
		return nil, err
	}
	apiHandler.SetIcons(iconsSvc)

	// Constructed only when enabled — analyticsEnabled (above) still flows
	// to the frontend either way via ServeSite, but h.analytics itself
	// stays nil when off, so AdminGetAnalytics's nil-guard 404s without
	// this service ever dialing out.
	if cfg.AnalyticsEnabled {
		apiHandler.SetAnalytics(analytics.New(cfg.AnalyticsBaseURL, cfg.AnalyticsAPIToken, cfg.AnalyticsVhost))
		if cfg.AnalyticsPublicURL != "" {
			apiHandler.SetAnalyticsPublicURL(cfg.AnalyticsPublicURL)
		}
	}

	commentSessions := commentauth.NewStore(st)
	mastodonClient := mastodon.NewClient()
	commentApps := mastodon.NewAppRegistrar(mastodonClient, mastodon.NewAppStore(st))
	commentHandler := api.NewCommentHandler(ob, commentSessions, commentApps, mastodonClient, cfg.BaseURL)

	// Local dev only (HTTPDev, e.g. NITPUB_HTTP=1): skip the loopback/
	// private-IP check so a local mock Mastodon instance can be used to
	// manually exercise the reference-instance connect flow. Never true
	// for a real deployment — see SetReference's doc comment.
	referenceClient := mastodonClient
	if cfg.HTTPDev {
		// InsecureSkipVerify too: a local mock instance for manual testing
		// serves a self-signed cert (RegisterApp/ExchangeToken/search all
		// hardcode https://), which the default transport would otherwise
		// reject before ever reaching the loopback check this is already
		// bypassing.
		referenceClient = mastodon.NewClientWithHTTP(&http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		})
	}
	referenceApps := mastodon.NewAppRegistrar(referenceClient, mastodon.NewAppStoreIn(st, store.BucketReferenceApps))
	apiHandler.SetReference(referenceClient, referenceApps, mastodon.NewReferenceAuthStore(st), cfg.HTTPDev)

	inboxHandler := inbox.NewHandler(verify, ap, ob, deliverOne, string(actorIRI), cfg.BaseURL, modSvc, func() bool {
		m, err := siteSvc.Load()
		if err != nil {
			return true // fail safe: keep the moderation queue on
		}
		return m.Federation.ModerationOn()
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /.well-known/webfinger", actSvc.ServeWebFinger)
	mux.HandleFunc("GET /.well-known/host-meta", actSvc.ServeHostMeta)
	mux.HandleFunc("GET /actor", actSvc.ServeActor)
	mux.HandleFunc("GET /outbox", func(w http.ResponseWriter, r *http.Request) {
		col, err := ob.OutboxCollection()
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/activity+json")
		_ = json.NewEncoder(w).Encode(col)
	})
	mux.Handle("POST /inbox", inboxHandler)
	mux.HandleFunc("GET /posts/{id}", apiHandler.ServePostObject)
	mux.HandleFunc("/api/posts", apiHandler.ServePosts)
	mux.HandleFunc("GET /api/posts/{id}", apiHandler.GetPost)
	mux.HandleFunc("GET /api/posts/{id}/replies", apiHandler.GetPostReplies)
	mux.HandleFunc("PUT /api/posts/{id}", apiHandler.GetPost)
	mux.HandleFunc("DELETE /api/posts/{id}", apiHandler.GetPost)
	mux.HandleFunc("POST /api/posts/drafts", apiHandler.SaveDraft)
	mux.HandleFunc("POST /api/posts/{id}/publish", apiHandler.PublishDraft)
	mux.HandleFunc("GET /api/site", apiHandler.ServeSite)
	mux.HandleFunc("GET /api/site/pages/{path...}", apiHandler.ServeSitePage)
	mux.HandleFunc("GET /api/search", apiHandler.ServeSearch)
	mux.HandleFunc("GET /api/admin/version", apiHandler.AdminCheckVersion)
	mux.HandleFunc("GET /api/admin/site", apiHandler.AdminGetSite)
	mux.HandleFunc("PUT /api/admin/site/manifest", apiHandler.AdminPutManifest)
	mux.HandleFunc("PUT /api/admin/site/files/{relPath...}", apiHandler.AdminPutSiteFile)
	mux.HandleFunc("POST /api/admin/import/posts", apiHandler.AdminImportPosts)
	mux.HandleFunc("GET /api/admin/federation", apiHandler.AdminGetFederation)
	mux.HandleFunc("POST /api/admin/federation/resend-accepts", apiHandler.AdminResendAccepts)
	mux.HandleFunc("POST /api/admin/federation/backfill", apiHandler.AdminBackfillFederation)
	mux.HandleFunc("POST /api/admin/federation/redeliver-shared", apiHandler.AdminRedeliverShared)
	mux.HandleFunc("GET /api/admin/federation/deliveries", apiHandler.AdminFederationDeliveries)
	mux.HandleFunc("GET /api/admin/federation/reference/status", apiHandler.AdminGetReferenceStatus)
	mux.HandleFunc("POST /api/admin/federation/reference/connect", apiHandler.AdminStartReferenceConnect)
	mux.HandleFunc("GET /api/admin/federation/reference/callback", apiHandler.AdminReferenceCallback)
	mux.HandleFunc("POST /api/admin/federation/reference/disconnect", apiHandler.AdminDisconnectReference)
	mux.HandleFunc("POST /api/admin/federation/reference/resolve", apiHandler.AdminResolveReferencePermalinks)
	mux.HandleFunc("GET /api/admin/replies", apiHandler.AdminListPendingReplies)
	mux.HandleFunc("GET /api/admin/replies/reviewed", apiHandler.AdminListReviewedReplies)
	// {id} (single path segment) is sufficient here — the composite reply key
	// (postSlug:orderingValue:activityIDHash) contains ':' but never '/', so
	// it fits one segment. Only actor URIs below need the {name...} wildcard
	// since they contain '/'.
	mux.HandleFunc("POST /api/admin/replies/{id}/approve", apiHandler.AdminApproveReply)
	mux.HandleFunc("POST /api/admin/replies/{id}/reject", apiHandler.AdminRejectReply)
	mux.HandleFunc("POST /api/admin/replies/{id}/skip", apiHandler.AdminSkipReply)
	mux.HandleFunc("POST /api/admin/replies/{id}/revert", apiHandler.AdminRevertReply)
	mux.HandleFunc("GET /api/admin/moderation/trusted", apiHandler.AdminListTrustedActors)
	mux.HandleFunc("POST /api/admin/moderation/trusted", apiHandler.AdminAddTrustedActor)
	mux.HandleFunc("DELETE /api/admin/moderation/trusted/{actor...}", apiHandler.AdminRemoveTrustedActor)
	mux.HandleFunc("GET /api/admin/moderation/blocked", apiHandler.AdminListBlockedActors)
	mux.HandleFunc("POST /api/admin/moderation/blocked", apiHandler.AdminAddBlockedActor)
	mux.HandleFunc("DELETE /api/admin/moderation/blocked/{actor...}", apiHandler.AdminRemoveBlockedActor)
	mux.HandleFunc("POST /api/auth/login", authAPI.Login)
	mux.HandleFunc("POST /api/auth/verify", authAPI.Verify)
	mux.HandleFunc("POST /api/auth/logout", authAPI.Logout)
	mux.HandleFunc("GET /api/auth/session", authAPI.Session)
	mux.HandleFunc("GET /api/admin/authcheck", authAPI.AuthCheck)
	mux.HandleFunc("GET /api/admin/settings", authAPI.Settings)
	mux.HandleFunc("PUT /api/admin/settings", authAPI.UpdateSettings)
	mux.HandleFunc("POST /api/admin/security/password", authAPI.ChangePassword)
	mux.HandleFunc("POST /api/admin/security/totp/enable", authAPI.EnableTOTP)
	mux.HandleFunc("POST /api/admin/security/totp/confirm", authAPI.ConfirmTOTP)
	mux.HandleFunc("POST /api/admin/security/totp/disable", authAPI.DisableTOTP)
	mux.HandleFunc("POST /api/admin/security/totp/cleanup", authAPI.CleanupTOTP)
	mux.HandleFunc("POST /api/admin/security/backup-codes/regenerate", authAPI.RegenerateBackupCodes)
	mux.HandleFunc("POST /api/admin/security/passkey/enroll-link", authAPI.PasskeyEnrollLink)
	mux.HandleFunc("POST /api/admin/security/passkey/disable", authAPI.DisablePasskey)
	mux.HandleFunc("GET /api/appearance", authAPI.Appearance)
	mux.HandleFunc("POST /api/auth/webauthn/login/begin", authAPI.WebAuthnLoginBegin)
	mux.HandleFunc("POST /api/auth/webauthn/login/finish", authAPI.WebAuthnLoginFinish)
	mux.HandleFunc("POST /api/auth/webauthn/register/begin", authAPI.WebAuthnRegisterBegin)
	mux.HandleFunc("POST /api/auth/webauthn/register/finish", authAPI.WebAuthnRegisterFinish)
	mux.HandleFunc("POST /api/comments/oauth/start", commentHandler.StartCommentAuth)
	mux.HandleFunc("GET /comment/callback", commentHandler.CommentAuthCallback)
	mux.HandleFunc("GET /api/comments/session", commentHandler.CommentSessionStatus)
	mux.HandleFunc("POST /api/comments/logout", commentHandler.CommentLogout)
	mux.HandleFunc("POST /api/media", apiHandler.UploadMedia)
	mux.HandleFunc("GET /media/{file}", apiHandler.ServeMedia)
	mux.HandleFunc("GET /icons/{name}", apiHandler.ServeIcon)
	mux.HandleFunc("GET /api/admin/analytics", apiHandler.AdminGetAnalytics)
	mux.HandleFunc("GET /api/icons/catalog", apiHandler.ServeIconCatalog)
	mux.HandleFunc("GET /feed.xml", apiHandler.ServeFeed)
	mux.HandleFunc("GET /api/unfurl", apiHandler.Unfurl)

	registerLegacyRedirects(mux)

	if static != nil {
		// Beacon is injected only when analytics is on *and* a public
		// GoatCounter URL is configured — that URL is both the admin
		// dashboard link-out and the count.js /count collector base.
		analyticsPublicURL := ""
		if cfg.AnalyticsEnabled {
			analyticsPublicURL = cfg.AnalyticsPublicURL
		}
		mux.Handle("/", spaHandler(static, authSvc.ThemeID, cfg.BaseURL, string(actorIRI), cfg.Title, analyticsPublicURL))
	}

	return &Server{cfg: cfg, mux: mux, actor: actSvc, outbox: ob, cancel: cancel, telemetryStop: telemetryStop}, nil
}

func (s *Server) Handler() http.Handler { return withSecurityHeaders(s.mux) }

// Close stops background workers.
func (s *Server) Close() {
	if s.telemetryStop != nil {
		s.telemetryStop()
	}
	if s.cancel != nil {
		s.cancel()
	}
}
