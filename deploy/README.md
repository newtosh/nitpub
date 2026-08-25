# nitpub deployment (Advanced / ops)

Deep ops (multi-instance, GoatCounter, git-pull rebuilds). Installers: [docs.nitpub.com](https://docs.nitpub.com/) (hub: root [README](../README.md)). Product www/docs hosting: [pages.md](pages.md).

**GitHub repo:** [`newtosh/nitpub`](https://github.com/newtosh/nitpub)

## Private → public repo sync

`newtosh/nitpub-dev` (private) is the source of truth — internal plans, ops docs, and the VPS auto-deploy workflow live there only. `newtosh/nitpub` (public) is a filtered mirror: install script, `www`/`docs` Cloudflare Pages source, and the only place tags get built into GitHub Releases.

After merging to private `main`:

```bash
scripts/sync-public.sh
```

Diffs private `main`'s tracked tree against public `main` (excluding `.cursor/`, `docs/plans/`, `docs/tasks/`, `docs/ideation/`, and the private-only deploy workflow), and opens a PR on the public repo if anything changed. Review and merge that PR before tagging a release — `release.yml` only exists on the public repo now, so a tag only needs to reach public to ship.

## Configuration (all environments)

nitpub loads the **first config file that exists** (see `internal/config/config.go`). When the systemd service runs as user `nitpub` with `WorkingDirectory=/var/lib/nitpub`, the usual production path is:

**`/var/lib/nitpub/.config/nitpub/config.toml`**

not `/etc/nitpub/config.toml` unless that is the only file present.

Search order:

1. `$NITPUB_CONFIG` — explicit path (overrides everything)
2. `./nitpub.toml` — relative to the process working directory
3. `~/.config/nitpub/config.toml` — for the service user, `~` is `/var/lib/nitpub`
4. `/etc/nitpub/config.toml`
5. `/var/lib/nitpub/.config/nitpub/config.toml` — same as (3) when `HOME=/var/lib/nitpub`

Environment variables **override** values from the loaded file. See `deploy/config.toml.example`.

| Key | Description |
|-----|-------------|
| `domain` | Public hostname |
| `title` | Site name shown in the header and browser tab. Defaults to `"nitpub"` if unset — set this before first launch so a fresh instance isn't branded as the software itself. Changing it later requires editing this file and restarting `nitpub` (not yet exposed in `/admin`). |
| `port` | Listen port (default `8080`) |
| `data_dir` | bbolt data directory |
| `actor` | ActivityPub local username → WebFinger `acct:{actor}@{domain}` (e.g. `alice` → `@alice@blog.example.com`). **Set per instance** in server config, not in git. |
| `secret` | Production secret (not the admin password) |
| `http` | `true` for local HTTP dev |
| `system_user` | User that owns `data_dir` (default hint: `nitpub`) |

After changing `actor` or `domain`, restart `nitpub`. The actor document is rebuilt on startup; existing remote follows tied to the old handle are not migrated.

**Instance identity is not in the repo.** The committed `deploy/config.toml.example` is a generic template (`actor = "user"`). Your production handle (e.g. `@you@blog.example.com`) lives only in `/etc/nitpub/config.toml` (or `~/.config/nitpub/config.toml`) on that server. Bootstrap scripts accept `NITPUB_ACTOR` when first creating config.

**CLI and server share the same config.** Admin commands need exclusive access to the database; use `--offline` on the VPS (stops the service, runs the command, restarts):

```bash
# on the VPS as root (recommended)
nitpub admin init --username you@example.com --offline

# or from your laptop
ssh -t nitpub 'nitpub admin init --username you@example.com --offline'
```

## Standard update: `nitpub update --apply`

The normal way to update any instance, including the demo, is the Release binary — no git checkout required:

```bash
ssh nitpub 'nitpub update --apply'
```

Downloads the matching Release, verifies `SHA256SUMS`, replaces `/usr/local/bin/nitpub`, restarts the service. Run `nitpub update` (no `--apply`) first to just check what's available. This is manual by design — it runs whenever you choose, not on every merge, since not every merge cuts a new release.

## Maintainer/dev VPS: git pull deploy workflow (`--from-source`)

For rebuilding from an unreleased commit instead of the latest Release, `nitpub update --apply --from-source` (or `deploy/update.sh` directly) rebuilds from a git checkout at **`/var/lib/nitpub/src`**.

### VPS layout

| Path | Purpose |
|------|---------|
| `/var/lib/nitpub/src` | git checkout |
| `/usr/local/bin/nitpub` | installed binary (after `update.sh`) |
| `/etc/nitpub/config.toml` | system config (root setup) |
| `/var/lib/nitpub` | database + uploads |
| `/var/lib/nitpub/.local/` | user-installed gh, Go, Node |

### One-time: user-space prep

As the **`nitpub`** user (no root):

```bash
sudo -u nitpub -i
NITPUB_REPO_DIR=/var/lib/nitpub/src bash /var/lib/nitpub/src/deploy/prepare-user.sh
```

Installs **gh**, **Go**, **Node** under `~/.local/`.

### One-time: GitHub auth (private forks / maintainer workflows)

Public clones use plain `git clone https://github.com/newtosh/nitpub.git` — no `gh` required.

If your checkout is a **private** fork or mirror, authenticate `gh` for that remote. Run as the user that owns the checkout (`nitpub`):

```bash
sudo -u nitpub -i
export PATH="$HOME/.local/bin:$HOME/.local/go/bin:$PATH"
gh auth login
```

Use **HTTPS** or **SSH**; confirm with `gh auth status`.

> If you authenticated as `root` instead, either re-run `gh auth login` as `nitpub`, or copy `/root/.config/gh/` to `/var/lib/nitpub/.config/gh/` and `chown -R nitpub:nitpub`.

### One-time: link checkout to GitHub

After `gh auth login` (or after a public clone with `origin` already set):

```bash
sudo -u nitpub -i
export PATH="$HOME/.local/bin:$HOME/.local/go/bin:$PATH"
bash /var/lib/nitpub/src/deploy/link-github-remote.sh
```

This adds `origin`, fetches, and resets to `origin/main`.

### Rebuild from source (maintainer, not the standard path)

From your laptop:

```bash
make deploy
```

Or explicitly:

```bash
ssh nitpub 'bash /var/lib/nitpub/src/deploy/update.sh'
```

On the VPS directly:

```bash
make deploy-local
```

The script pulls as the `nitpub` user, builds in the checkout, and installs/restarts as root.

`update.sh` builds the PWA, installs `/usr/local/bin/nitpub`, and restarts `nitpub` (restart needs root/systemd).

### Local development

```bash
make help          # list targets
make build         # PWA + .build/nitpub
make test          # go test ./...
make run           # local HTTP server
make dev-web       # Vite dev server
```

### One-time root setup (systemd + system config)

Still requires root once:

```bash
install -m 644 /var/lib/nitpub/src/deploy/nitpub.service /etc/systemd/system/
# create /etc/nitpub/config.toml from deploy/config.toml.example
systemctl daemon-reload && systemctl enable --now nitpub
```

Or `deploy/cutover-domain.sh` from your laptop for TLS + config.

## Manual deploy (scp, no git on server)

```bash
cd web && npm install && npm run build
cd .. && GOOS=linux GOARCH=amd64 go build -o nitpub ./cmd/nitpub
scp nitpub nitpub:/usr/local/bin/nitpub
ssh nitpub 'systemctl restart nitpub'
```

## Web UI routes

| Path | Purpose |
|------|---------|
| `/` | Public blog index |
| `/p/:slug` | Post permalink |
| `/login` | Sign in (redirects to `/author` when already authenticated) |
| `/author` | Compose and recent posts (**requires sign-in**) |
| `/author/edit/:slug` | Edit post |
| `/author/enroll?token=…` | WebAuthn enrollment (from `nitpub admin webauthn register`) |
| `/admin` | Instance settings — **Appearance** palette + light/dark mode |

### Site themes

Themes are **instance-wide** palette presets. Each palette ships **light and dark** variants. The active palette is stored in **bbolt** (not `config.toml`). **Color mode** (auto/light/dark) is per-user in browser `localStorage`.

| Palette | Notes |
|---------|-------|
| `github` | Default — Primer-inspired neutral |
| `nord` | Arctic blues |
| `ayu` | Warm editor tones |
| `tokyo-night` | Day / night indigo |
| `catppuccin` | Latte / Mocha |
| `dracula` | Purple accents |
| `monokai` | Classic editor green |

- **Public:** `GET /api/appearance` returns `{ "theme_id" }`; production `index.html` injects `data-theme` on `<html>`. Color mode is set client-side (`data-scheme`, default `auto`).
- **Everyone:** header icons toggle Auto / Light / Dark (persisted locally).
- **Admin:** sign in at `/admin`, pick a palette, preview, then **Save palette**.

Legacy v1 ids (`warm`, `paper`, `ocean`, `midnight`) map to the closest palette above.

Legacy URLs `/admin/edit/*` and `/admin/enroll` redirect to the `/author` equivalents.

## Admin account

While `nitpub.service` is running it holds an exclusive database lock. Admin CLI commands fail fast after a few seconds, or use `--offline` to stop the service automatically:

```bash
# first-time setup (on VPS as root)
nitpub admin init --username you@example.com --offline
nitpub admin status --offline
nitpub admin backup-codes regenerate --offline
nitpub admin totp enable --offline          # optional
nitpub admin webauthn register --offline    # optional
```

Non-interactive password (scripts):

```bash
nitpub admin init --username you --password-stdin --offline <<< 'your-password'
```

Recovery: `nitpub admin password --force --offline`, `nitpub admin reset-2fa --force --offline`.

Run `nitpub admin --help` for all subcommands.

## DNS

| Type | Name | Value |
|------|------|-------|
| A | `@` | VPS IPv4 |
| A | `www` | same IPv4 |

## Domain cutover (TLS)

```bash
export DROPLET_HOST=myhost
deploy/cutover-domain.sh blog.example.com
```

## Updating

Binary-only hosts (quick install):

```bash
nitpub update              # check current vs. latest GitHub release — read-only
nitpub update --apply      # download Release binary, verify SHA256SUMS, restart
```

Maintainer VPS with a git checkout that rebuilds from source:

```bash
nitpub update --apply --from-source   # runs deploy/update.sh (git pull, rebuild, restart)
```

`--from-source` needs `NITPUB_REPO_DIR` set if the checkout isn't at the default `/var/lib/nitpub/src`, and the same permissions `deploy/update.sh` itself needs (repo write access; root or passwordless sudo to restart). The admin UI (`/admin` → System) shows the same check-only comparison — it never triggers an update itself.

## Multi-instance (same VPS, second domain)

The same binary and VPS can run more than one instance — e.g. a project demo site and a personal blog — each with its own domain, port, data directory, and admin account. See `docs/tasks/2026-07-17-multi-instance-deployment.md` for the full checklist; summary:

1. **Config**: copy `deploy/config.instance.toml.example` → `/etc/nitpub/<name>/config.toml`, edit `domain`, `port` (pick an unused one, e.g. `8082`), `data_dir` (e.g. `/var/lib/nitpub-<name>`), `actor`, and a **freshly generated** `secret` (`openssl rand -hex 32` — never reuse another instance's).
2. **Service**: copy `deploy/nitpub-instance.service.example` → `/etc/systemd/system/nitpub-<name>.service`. The unit name **must** match `nitpub*.service` (that's what `deploy/update.sh` restarts on every deploy) — set `WorkingDirectory` to the new `data_dir` and `Environment=NITPUB_CONFIG` to the new config path. Then `systemctl daemon-reload && systemctl enable --now nitpub-<name>`.
3. **Reverse proxy**: append a second block to the Caddyfile (see `deploy/Caddyfile`) pointing the new domain at the new port. Append, don't overwrite — the existing site's block must stay untouched.
4. **DNS**: point the new domain at the same VPS IP (A/AAAA), same as the [DNS](#dns) section above.
5. **Admin account**: `nitpub admin init --offline` needs the right service stopped — pass `NITPUB_SERVICE=nitpub-<name>` (matches the unit name from step 2) so `--offline` stops/starts the correct instance, not the default one:
   ```bash
   NITPUB_CONFIG=/etc/nitpub/<name>/config.toml NITPUB_SERVICE=nitpub-<name> \
     nitpub admin init --username you@example.com --offline
   ```
6. **2FA/WebAuthn**: credentials aren't portable across domains — enroll separately per instance.

## Site content

Custom pages, navigation, home teaser count, archive mode, and search are configured under **`{data_dir}/site/`** (see [`docs/site-content.md`](../docs/site-content.md)).

| Path | Purpose |
|------|---------|
| `{data_dir}/site/site.toml` | Nav, home/archive/search settings, page registry |
| `{data_dir}/site/pages/*.md` | Markdown custom pages |
| `{data_dir}/site/pages/*.links.toml` | Link collection pages |

Edit files directly or use **Admin → Site** in the PWA. Posts remain in bbolt; bulk import:

```bash
nitpub import posts /path/to/markdown --offline
```

Local example seed: `./scripts/seed-site-example.sh`

## Federation interop (U9)

Before trusting federation in production, run the U9 checklist in [`docs/federation-interop.md`](../docs/federation-interop.md).

```bash
# Public discovery preflight
DOMAIN=blog.example.com ACTOR=alice bash scripts/federation-interop-preflight.sh

# On VPS after a Mastodon follow/reply (stop service first — bbolt lock)
sudo systemctl stop nitpub
cd /var/lib/nitpub/src
sudo -u nitpub env PATH=/var/lib/nitpub/.local/go/bin:/usr/bin:/bin \
  NITPUB_CONFIG=/etc/nitpub/config.toml go run scripts/federation-inspect.go
sudo systemctl start nitpub

# From laptop (preflight + inspect + RSS):
bash scripts/u9-status.sh
```

## GoatCounter analytics (optional)

nitpub can proxy pageview stats from a self-hosted [GoatCounter](https://www.goatcounter.com/) instance into **Admin → Analytics** — GoatCounter itself is never exposed publicly; nitpub's backend is the only client, over `localhost`. This is a one-time manual setup, not part of the automated `nitpub*.service` deploy (`deploy/update.sh`'s restart loop only matches that glob — see `deploy/goatcounter.service`'s comment).

1. **Download the binary** on the VPS (see [goatcounter.com/code](https://www.goatcounter.com/code) for the latest release):
   ```bash
   curl -L -o goatcounter.gz https://github.com/arp242/goatcounter/releases/latest/download/goatcounter-linux-amd64.gz
   gunzip goatcounter.gz && chmod +x goatcounter
   sudo mv goatcounter /usr/local/bin/goatcounter
   ```

2. **Create the `goatcounter` service user and data dir:**
   ```bash
   sudo useradd --system --home /var/lib/goatcounter --create-home goatcounter
   sudo -u goatcounter mkdir -p /var/lib/goatcounter/db
   ```

3. **Create the site.** `-vhost` must match the Host header browsers will send to GoatCounter. If you use the public `stats.` subdomain (recommended — enables the auto-injected tracking beacon and admin dashboard link-out), use that hostname:
   ```bash
   sudo -u goatcounter /usr/local/bin/goatcounter db create site \
     -db=sqlite:///var/lib/goatcounter/db/goatcounter.sqlite3 \
     -createdb \
     -vhost=stats.example.com -user.email=you@example.com
   ```
   Follow the prompt to set a login password for the GoatCounter admin UI (or pass `-user.password=...`).

   **API-only / no public subdomain:** you can use an internal identifier like `-vhost=nitpub-internal` instead — but then pageviews are never collected (nothing public serves `/count`), and Admin → Analytics stays empty unless something else feeds GoatCounter.

   **This same GoatCounter instance can host other apps' sites too** — run this command again with a different `-vhost` against the same database, no new systemd unit or port needed (GoatCounter is natively multi-tenant). Once a *second* site exists, set `analytics_vhost` in nitpub's config to this site's `-vhost` (step 6) — otherwise nitpub's API calls silently relied on GoatCounter's single-site fallback, which stops applying.

4. **Install and start the service:**
   ```bash
   sudo install -m 644 /var/lib/nitpub/src/deploy/goatcounter.service /etc/systemd/system/
   sudo systemctl daemon-reload && sudo systemctl enable --now goatcounter
   ```

5. **Generate an API token:** with the service running, tunnel to it (`ssh -L 8181:127.0.0.1:8181 nitpub`) and open `http://127.0.0.1:8181` in a browser, sign in, then **Settings → API** to create a token with read access to the stats endpoints.

6. **Enable in nitpub's config** (`/etc/nitpub/config.toml` or `/etc/nitpub/<instance>/config.toml` — see `deploy/config.toml.example`):
   ```toml
   analytics_enabled = true
   analytics_api_token = "<token from step 5>"
   analytics_base_url = "http://127.0.0.1:8181"
   analytics_vhost = "stats.example.com"   # match -vhost from step 3; required once >1 site exists
   ```
   Restart the matching `nitpub*.service` to pick up the change. The **Analytics** tab then appears under `/admin`.

### Public stats subdomain (beacon + dashboard link-out)

With only the steps above, Admin → Analytics can *read* stats but nothing *records* pageviews — nitpub auto-injects GoatCounter's tracking beacon only when `analytics_public_url` is set. The same URL powers the "Open GoatCounter" link on the Analytics tab.

Caddy's `forward_auth` gates the dashboard UI behind nitpub's admin session, while `/count` and `/count.js` stay public so anonymous visitors can be counted. Caddy injects GoatCounter's `access-token` secret itself, so that secret never reaches the browser.

1. **DNS:** point `stats.example.com` (A/AAAA) at the same VPS IP as the site.

2. **Put GoatCounter behind Caddy** — uncomment and adapt the `stats.{$DOMAIN}` block in `deploy/Caddyfile`. Point `forward_auth` at this instance's own port (default `8080`; a second instance like nwtn.sh uses `8081`). Keep the `@goatPublic` path matcher so `/count` and `/count.js` bypass auth.

3. **Generate GoatCounter's dashboard secret token.** Tunnel in (`ssh -L 8181:127.0.0.1:8181 nitpub`, open `http://127.0.0.1:8181`), go to **Settings → Site settings → Dashboard viewable by**, choose **"Logged in users or with secret token,"** and generate a random secret. Different from the API token in step 5 — don't reuse it.

4. **Give Caddy that secret via an env var**, not a committed file:
   ```bash
   # in Caddy's EnvironmentFile (systemd) or however this VPS injects env vars into Caddy:
   GOATCOUNTER_ACCESS_TOKEN=<secret from step 3>
   ```
   Reload Caddy after setting it.

5. **Widen nitpub's session cookie** and set the public URL. In nitpub's `config.toml`:
   ```toml
   session_cookie_domain = ".example.com"    # covers both blog.example.com and stats.example.com
   analytics_public_url = "https://stats.example.com"
   ```
   `session_cookie_domain` must be a common parent of both nitpub's own domain and the stats subdomain — a bare domain works too (`example.com`), the leading dot is cosmetic (RFC 6265 treats them the same). Restart nitpub. Public pages then include the GoatCounter beacon automatically; no manual `<script>` paste.

6. **Verify:**
   - Anonymous: `curl -sI https://stats.example.com/count.js` → 200; page HTML on the site contains `data-goatcounter=...`.
   - Logged into nitpub admin: `https://stats.example.com` opens GoatCounter's dashboard.
   - Logged out / private window: dashboard paths 401 before reaching GoatCounter.

**Tradeoff to know:** this widens the admin session cookie beyond nitpub's own domain — a compromise of the stats subdomain (or anything else sharing that parent domain and able to set cookies) could then read or spoof nitpub's session cookie too. Skip this section and leave `analytics_public_url` unset if that's not a tradeoff you want; the Admin Analytics tab still works for reading whatever data exists, but no beacon is injected.

## Verify

1. `curl https://$DOMAIN/healthz`
2. `curl 'https://$DOMAIN/.well-known/webfinger?resource=acct:$ACTOR@$DOMAIN'` (set `ACTOR` to your config `actor` value)
