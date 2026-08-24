# nitpub

**ActivityPub blog for notes and long-form articles on your own domain** — one Go binary on a small VPS, without running Mastodon.

Live demo: [nitpub.com](https://nitpub.com) (`@nit@nitpub.com`)

## Who it’s for

Self-hosting indie bloggers who want ownership of notes + articles with federated replies, on a budget VPS — not a closed SaaS, and not Mastodon’s ops weight.

## Prerequisites

- A Linux VPS (Debian/Ubuntu for the installer helpers; ≈1 GB RAM is enough for idle use)
- A domain pointed at the VPS (A/AAAA)
- Root (or sudo) on the VPS

You do **not** need Go or Node on the VPS for the golden path.

## Golden path (first published note)

One recommended path: **one-liner → `nitpub install` → `/login` → publish a note.**

Optional gates during install: **Caddy**, **Federation** (cross-post default), and **Analytics** (config scaffold). Deep ops live in [`deploy/README.md`](deploy/README.md).

### 1. DNS

Point your apex (and optional `www`) at the VPS:

| Type | Name | Value |
|------|------|-------|
| A / AAAA | `@` | VPS address |
| A / AAAA | `www` | same (optional) |

### 2. Install

On the VPS as root:

```bash
curl -fsSL https://raw.githubusercontent.com/newtosh/nitpub/main/scripts/install.sh | bash
```

That downloads the latest GitHub Release binary (linux amd64/arm64), installs it to `/usr/local/bin/nitpub`, and runs `nitpub install`.

The wizard asks for domain / actor / secret (or generates a secret), then y/n for Caddy, Federation, and Analytics. It writes config if missing, sets up systemd, creates an admin account, runs `nitpub doctor`, and prints a `/login` next step.

Noninteractive example (fail closed if required flags are missing):

```bash
nitpub install --yes \
  --domain blog.example.com \
  --actor you \
  --secret "$(openssl rand -hex 32)" \
  --password '…' \
  --with-caddy --with-federation --no-analytics
```

Re-running `nitpub install` is lossless: existing config, Caddy site blocks, and `site.toml` are reported and skipped.

### 3. Publish your first note

1. Open `https://YOUR_DOMAIN/login` and sign in.
2. Open `/author/compose` and publish a note.
3. Confirm it is public on `/` and `/p/<slug>`.

### 4. Verify

```bash
nitpub doctor
curl -fsS "https://YOUR_DOMAIN/healthz"
curl -fsS "https://YOUR_DOMAIN/.well-known/webfinger?resource=acct:ACTOR@YOUR_DOMAIN"
```

Replace `ACTOR` with the `actor` value from your config.

---

## Manual download (backup)

If you prefer not to use the one-liner:

1. Download `nitpub-<tag>-linux-amd64` (or `arm64`) and `SHA256SUMS` from [Releases](https://github.com/newtosh/nitpub/releases).
2. Verify the checksum, install to `/usr/local/bin/nitpub`, then run `nitpub install`.

## Updates

```bash
nitpub update              # check only
nitpub update --apply      # download Release binary, verify checksum, restart
```

Maintainer checkouts that rebuild from source: `nitpub update --apply --from-source` (runs `deploy/update.sh`).

---

## Federation (expandable) — visible federated reply

Required for the full “docs done” bar after the golden path. nitpub speaks ActivityPub; remote replies are moderated by default.

1. **Discoverability** — From another Fediverse client, resolve `@actor@YOUR_DOMAIN` (exact handle from config), not a fuzzy search.
2. **Optional preflight** — `DOMAIN=YOUR_DOMAIN ACTOR=youractor bash scripts/federation-interop-preflight.sh`
3. **Follow** — Follow the nitpub actor from a Mastodon (or compatible) account you control.
4. **Publish with federation** — Keep cross-post enabled (install gate default, or Admin → Federation). Publish a note from `/author/compose` so followers receive it.
5. **Reply from Mastodon** — Reply publicly to that post from your Mastodon account.
6. **Make the reply visible** — By default inbound replies are **pending**:
   - Open **Admin → Moderation** and **approve** the reply, **or**
   - Trust the remote actor, **or**
   - Turn moderation off in Admin → Federation (`moderation_enabled = false`)
7. **Confirm** — Open the permalink `/p/<slug>#replies` and check the reply is shown.

Full checklist and inspect scripts: [`docs/federation-interop.md`](docs/federation-interop.md). Ops notes: [`deploy/README.md`](deploy/README.md#federation-interop-u9).

---

## Analytics (optional)

Self-hosted [GoatCounter](https://www.goatcounter.com/) is optional. The install Analytics gate only scaffolds config keys; finish token / public URL setup in [`deploy/README.md`](deploy/README.md#goatcounter-analytics-optional).

---

## Advanced / ops

Clone/build on the VPS, multi-instance, detailed GoatCounter, and maintenance: [`deploy/README.md`](deploy/README.md).

Local development:

```bash
make help
make build    # PWA + .build/nitpub
make test     # go test ./...
make run      # local HTTP
```

---

## Community

| Doc | Purpose |
|-----|---------|
| [LICENSE](LICENSE) | MIT |
| [SECURITY.md](SECURITY.md) | Vulnerability reports via GitHub Security Advisories only |
| [SUPPORT.md](SUPPORT.md) | Help via GitHub Issues (not for vulns) |

North star (not required today): a one-click VPS / marketplace image so the golden path shrinks further.

## License

[MIT](LICENSE)
