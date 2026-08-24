# Federation interop (U9)

Manual verification that nitpub federates with a **real Mastodon account** on the public fediverse.

**Plan authority:** `docs/plans/2026-07-15-001-feat-federation-core-milestone-plan.md` → U9
**Roadmap:** Phase 1 in `docs/ROADMAP.md`

## Prerequisites

- Public HTTPS instance with `domain` and `actor` set in **server config** (`/etc/nitpub/config.toml` or equivalent — not in git)
- A Mastodon account you control on any public instance
- Operator access to nitpub author UI (publish) and VPS (logs / DB inspect)

Your federation handle is **`@<actor>@<domain>`** (from config). Example: `actor = "alice"` and `domain = "blog.example.com"` → `@alice@blog.example.com`.

## Automated preflight

From your laptop or CI (substitute your instance values):

```bash
DOMAIN=blog.example.com ACTOR=alice bash scripts/federation-interop-preflight.sh
```

**High-confidence resolve check** (simulates Mastodon's WebFinger → actor chain; run this before mobile):

```bash
DOMAIN=blog.example.com ACTOR=alice STALE_ACTORS=wrongname bash scripts/federation-resolve.sh
```

Optional: confirm from *your* Mastodon instance (needs a user access token from Settings → Development):

```bash
MASTODON_INSTANCE=https://mastodon.social MASTODON_TOKEN=... \
  DOMAIN=blog.example.com ACTOR=alice bash scripts/federation-resolve.sh
```

Checks: `healthz`, WebFinger, `host-meta`, actor document (`#main-key`, `url`, `preferredUsername`, `@context`), outbox reachable, inbox rejects unsigned POST with 401.

### Mastodon search vs resolve (read before mobile testing)

| What you do | What happens |
|-------------|--------------|
| Fuzzy search `nit` or `nitpub.com` | Searches **local index only** — remote actors won't appear |
| Search `@nit@nitpub.com` → **Go to** | Forces **WebFinger resolve** on your instance — correct path |
| Search wrong handle first (`@nitpub@…`) | Mastodon caches **not found ~1 hour** — blocks later correct lookups |
| Follow → paste `https://<domain>/actor` | Bypasses search cache; fetches actor URL directly |

There is **no fediverse-wide propagation delay** for new domains. If origin checks pass but mobile still fails, suspect **negative cache on your Mastodon instance** or using the wrong handle.

### Operator preflight log (reference)

Record your own runs below. Example from the nitpub.com project instance:

| Date | Instance | Result | Notes |
|------|----------|--------|-------|
| 2026-07-27 | nitpub.com (`actor=nitpub`) | Pass | WebFinger → actor; RSS ~19 MB idle on VPS |

## Manual U9 checklist

Record results in the table below. One Mastodon instance is sufficient (KTD10).

### 1. Discovery

In your Mastodon client, search for the **full handle** `@<actor>@<domain>` (leading `@`). Fuzzy name search without the full handle will not resolve remote actors.

- [ ] Search `@<actor>@<domain>` — profile resolves (not unrelated accounts on other instances)
- [ ] Profile opens; actor URL is `https://<domain>/actor`

### 2. Follow

- [ ] Follow from Mastodon
- [ ] On VPS, confirm follower stored:

```bash
ssh <host> 'sudo systemctl stop nitpub && cd /var/lib/nitpub/src && sudo -u nitpub go run scripts/federation-inspect.go && sudo systemctl start nitpub'
```

Or copy `nitpub.db` offline and run `federation-inspect.go` locally.

- [ ] No `accept delivery failed` errors in `journalctl -u nitpub` during follow

### 3. Outbound — Note

- [ ] Publish a **note** from `/author` with distinctive text (e.g. `U9 note probe 2026-07-27`)
- [ ] Note appears on Mastodon home timeline with **rendered body** (not empty)
- [ ] Markdown sanity: link, bold, or short list renders readably

### 4. Outbound — Article

- [ ] Publish an **article** with title line + body paragraph
- [ ] Mastodon shows **summary text + link** to permalink (KTD2 — not title-only dead card)
- [ ] Opening the link shows full article on nitpub

### 5. Inbound — Reply

- [ ] Reply to the nitpub post **from Mastodon** (public reply to the delivered activity)
- [ ] `federation-inspect.go` shows `inbox activities` count increased
- [ ] Reply appears in **Admin → Moderation** (or auto-approves if the actor is trusted / `moderation_enabled = false`)
- [ ] After approve (or auto-approve), reply is visible on the public permalink `#replies`

### 6. Regression (post site-content work)

- [ ] Import a `.md` post via Admin → Site → Import posts
- [ ] Publish a new note; delivery still succeeds with follower from step 2

### 7. Footprint

- [ ] RSS on VPS under ~200 MB idle: `ps -o rss= -C nitpub`

## Manual run log

| Step | Date | Tester | Mastodon instance | Pass? | Notes |
|------|------|--------|-------------------|-------|-------|
| Discovery | | | | | |
| Follow | | | | | |
| Note outbound | | | | | |
| Article outbound | | | | | |
| Reply inbound | | | | | |
| Import regression | | | | | |
| RSS | 2026-07-27 | agent | n/a | Pass | ~19 MB (nitpub.com) |

## Gap list

Record failures here; each row becomes Phase 1 engineering or an explicit deferral.

| ID | Symptom | Likely area | Status |
|----|---------|-------------|--------|
| | | | |

## Known limitations (not U9 failures)

- Single Mastodon instance tested (KTD10)
- No Unfollow / Update / Delete federation
- Inbound replies default to a moderation queue (`moderation_enabled` defaults on). Approve in **Admin → Moderation**, trust the actor, or set `moderation_enabled = false` in site settings before a reply is public on `/p/:slug#replies`
