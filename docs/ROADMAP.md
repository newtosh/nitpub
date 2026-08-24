# nitpub V1 roadmap

Last updated: 2026-08-19

This document sets **execution order** after Site Content & Discovery shipped. Federation is the product bet; admin polish and content features are secondary until interop is proven and launch scope is explicit.

**Status: all four phases closed.** Phase 3 (moderation/trust) and Phase 4 (threading/comments) shipped together in #10 (`feat(moderation): add reply moderation and public thread display`). The Phase 2 P1 backlog item (federation admin delivery log + resend/backfill/redeliver UI) shipped in #12. Everything in the V1 launch definition below is live on `nitpub.com`. Work since then (#13–#17, plus this session's mobile UX pass) is admin/compose polish beyond the original roadmap, not gating launch. A 2026-08-19 competitive eval against pika.page found two gaps (newsletter, guestbook) that were deliberately deferred past v1 given the end-of-month ship target.

## Completed (pre–V1 launch)

| Milestone | Notes |
|-----------|--------|
| Federation Core U1–U8 | Code + deploy on nitpub.com |
| Content & Theming v1 | Notes/articles, markdown, themes |
| Author/Admin UI foundation | `/author` publish, `/admin` shell |
| Supported themes | Palette picker in admin |
| Site Content & Discovery | Site editor, `/posts`, search, import |
| Federation Core U9 | Real Mastodon interop verified on production, 2026-08-01 — see `docs/tasks/2026-07-29-u9-manual-verification.md` |

---

## Phase 1 — Federation integration & testing (closed 2026-08-01)

**Goal:** Prove the core bet works end-to-end on production before building more surface area.

**Status:** All deliverables verified against production (`nitpub.com`, actor `nit`, `@fleetfind@mastodon.social`). One gap found and resolved with residual (2 test posts silently dropped by Mastodon on first attempt, not reproducible on retest — see U9 doc Gap G1). Phase 2 is now unblocked.

**Authority:** `docs/plans/2026-07-15-001-feat-federation-core-milestone-plan.md` → U9

### Deliverables

1. **U9 interop checklist** (manual, real Mastodon account)
   - WebFinger / actor discovery
   - Follow from Mastodon → nitpub actor
   - Note + Article appear in Mastodon timeline with **full content** (not title-only)
   - Reply from Mastodon → verified, stored (inspect logs/DB)
   - RSS on VPS within budget (~200 MB idle tripwire)
2. **Regression after site/import work**
   - Publish note + article after markdown import
   - Confirm outbox delivery still succeeds
3. **Interop runbook** — short doc: what was tested, account used, known gaps (one Mastodon instance only per KTD10)
4. **Gap list** — anything U9 fails becomes Phase 1 engineering, not Phase 2

**Runbook:** [`docs/federation-interop.md`](federation-interop.md) · **Preflight:** `bash scripts/federation-interop-preflight.sh`

### Out of scope for Phase 1

- Federation admin UI (Phase 2 audit may prioritize it)
- Moderation UI
- Public thread view

---

## Phase 2 — Admin V1 enablement audit (closed 2026-08-01)

**Goal:** Inventory every operator-facing control, decide what ships in V1 vs stays CLI/post-launch, and produce a **prioritized backlog** so launch scope is intentional—not “whatever placeholders exist.”

This is a **planning gate**, not a feature milestone. Output is a signed-off priority list before large admin builds resume.

**Authority:** `docs/plans/2026-08-01-001-feat-admin-v1-enablement-audit-plan.md`

### Inventory template

For each capability, classify:

| Field | Values |
|-------|--------|
| **Surface** | Admin web / Author web / CLI only / Public |
| **Status** | Shipped / Partial / Placeholder / Missing |
| **V1 tier** | **P0 launch** / P1 soon after / P2 post-launch / **CLI-only OK** |
| **Notes** | Dependencies, risk, federation tie-in |

### Completed inventory

| Capability | Surface | Status | Proposed V1 tier | Notes |
|------------|---------|--------|------------------|-------|
| Theme palette | Admin → Appearance | Shipped | P0 | |
| Site manifest, pages, nav | Admin → Site | Shipped | P0 | Includes search enabled/disabled toggle |
| Post import (upload) | Admin → Site | Shipped | P0 | |
| Compose / edit / publish | Author | Shipped | P0 | Includes media upload (`/api/media`) and unfurl link-preview support |
| Login, session, passkey enroll | Author | Shipped | P0 | |
| WebFinger / actor / delivery | Backend | Shipped, verified (U9, 2026-08-01) | P0 | |
| Federation status (followers, delivery log, resend-accepts/backfill/redeliver-shared) | Admin → Federation | Shipped (#12) | P1 | Delivery log + UI buttons for resend-accepts/backfill/redeliver-shared shipped 2026-08-11 |
| Security (password, TOTP, backup codes, passkey management) | Admin → Security / CLI | CLI today | CLI-only OK (V1) | Admin UI gets 4 disabled controls signaling a future web UI; CLI mutations require `--offline` (stop service, holds exclusive DB lock) |
| `admin reset-2fa` | CLI only | Shipped | CLI-only OK | Recovery-path command, same family as Security; requires `--offline`; no disabled UI control (destructive action, not a "coming soon" candidate) |
| `config.toml` editing | Admin | Missing | CLI-only, post-launch | One-time-setup values (domain, port, actor, secret); requires `--offline` |
| `admin status` | CLI only | Shipped | CLI-only OK | Operator health-check (TOTP/WebAuthn/backup-code state, config/data paths) |
| feed.xml (RSS/Atom) | Public | Shipped | P0 | ~19 MB verified during U9 RSS budget check; no admin configurability needed |
| Moderation queue | Admin | Shipped (#10) | P0 | |
| Thread / reply display | Public | Shipped (#10) | P0 | |

### V1 launch definition

On day one, without SSH access, an operator can: sign in with a password (and passkey, once enrolled) at `/author`; compose, edit, and publish notes and articles, including image uploads and link previews; import existing markdown posts; configure the site's theme, pages, navigation, and search visibility from `/admin`; and have posts and follows federate correctly over ActivityPub. Security credential changes, `config.toml` edits, and the federation delivery log/resend tools require CLI + SSH access — the Security card in `/admin` signals this explicitly rather than looking broken or unfinished.

### Prioritized backlog

1. **P0 — shipped, no action needed:** theme palette, site manifest/pages/nav/search, post import, compose/edit/publish (incl. media upload, unfurl), login/session/passkey, federation core delivery, moderation queue, thread/reply display.
2. **P1 — shipped:** federation admin delivery log + UI buttons for resend-accepts, backfill, and redeliver-shared (#12).
3. **P2 / CLI-only, intentionally deferred:** Security web UI (password/TOTP/backup-codes/passkey management), `config.toml` editor.

---

## Phase 3 — Moderation & trust (closed)

**Goal:** Contain open-fediverse abuse before showing replies publicly.

**Authority:** PRD §7; `STRATEGY.md` Moderation & Trust track

**Shipped:** #10 `feat(moderation): add reply moderation and public thread display` — inbound reply approval queue, trusted/blocked actor lists (`ModerationQueue.vue`), moderation-gated thread rendering.

---

## Phase 4 — Threading & comments (closed)

**Goal:** Make stored replies **visible** on permalinks and feeds—the differentiation vs WriteFreely.

**Shipped:** bundled into #10 alongside Phase 3 — thread model on post permalinks, public display of approved replies. Inbound replies were already stored (Federation Core R7); this phase was presentation + moderation integration, not new AP protocol work.

---

## Explicitly later (post–V1)

- Link collections in search
- In-app `config.toml` editor (Phase 2 audit kept this CLI-only, post-launch)
- Multi-instance deploy (`docs/tasks/2026-07-17-multi-instance-deployment.md`)
- AT Protocol bridging
- Vitest / frontend test harness
- Newsletter / email delivery, guestbook (evaluated 2026-08-19 against pika.page, deferred for EOM v1 ship)

---

## Order summary

```
Done:     Content, themes, site editor (U1–U8 site plan)
    ↓
Phase 1:  Federation interop + testing (U9 + regression)         — closed 2026-08-01
    ↓
Phase 2:  Admin V1 enablement audit → priority list               — closed 2026-08-01
    ↓
Phase 3:  Moderation & trust                                      — closed (#10)
    ↓
Phase 4:  Threading & comments                                    — closed (#10)
    ↓
Launch:   V1 launch definition met — live on nitpub.com
    ↓
Since:    Compose/admin UX polish (#13–#17, mobile pass 2026-08-19)
```
