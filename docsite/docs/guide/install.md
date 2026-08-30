# Quick install <Badge type="tip" text="Recommended" />

One path: **one-liner → `nitpub install` → `/login` → publish a note.**

Optional gates during install: **Caddy**, **Federation** (cross-post default), and **Analytics** (config scaffold). Deep ops: [deploy/README.md](https://github.com/newtosh/nitpub/blob/main/deploy/README.md).

## Prerequisites

- A Linux VPS (Debian/Ubuntu for the installer helpers; ≈1 GB RAM is enough for idle use)
- A domain pointed at the VPS (A/AAAA)
- Root (or sudo) on the VPS

You do **not** need Go or Node on the VPS for this path.

## 1. DNS

Point your apex (and optional `www`) at the VPS:

| Type | Name | Value |
|------|------|-------|
| A / AAAA | `@` | VPS address |
| A / AAAA | `www` | same (optional) |

For the public product/docs hosts (`www.nitpub.com`, `docs.nitpub.com`), see the product site cutover — those are separate from your blog apex.

## 2. Install

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

## 3. Publish your first note

1. Open `https://YOUR_DOMAIN/login` and sign in.
2. Open `/author/compose` and publish a note.
3. Confirm it is public on `/` and `/p/<slug>`.

## 4. Verify

```bash
nitpub doctor
curl -fsS "https://YOUR_DOMAIN/healthz"
curl -fsS "https://YOUR_DOMAIN/.well-known/webfinger?resource=acct:ACTOR@YOUR_DOMAIN"
```

Replace `ACTOR` with the `actor` value from your config.

## Next

- [One-click deploy](/guide/one-click-deploy) to skip the SSH install step via a provider startup script
- [Manual download](/guide/manual-download) if you prefer not to use the one-liner
- [Federation](/guide/federation) for a visible federated reply
- [Analytics](/guide/analytics) for optional GoatCounter
