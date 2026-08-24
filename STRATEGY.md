---
name: nitpub
last_updated: 2026-07-27
---

# nitpub Strategy

## Target problem

Solo blogger paying for micro.blog Premium — cost isn't justified by a closed-source SaaS you can't inspect, extend, or run yourself. If money's changing hands, ownership should too.

## Our approach

Win by treating notes and long-form articles as equally first-class in one ActivityPub-federated tool, on your own domain — not bolted onto a microblog (Mastodon can't do custom-domain long-form at all) and not split across separate single-purpose tools (WriteFreely = articles only, no visible replies; microblog.pub = notes only, no articles).

## Who it's for

**Primary:** Self-hosting indie blogger - Technically-capable enough to run a single Go binary on a $5-10/mo VPS, currently paying for (or avoiding) micro.blog, wants ownership without Mastodon's ops weight.

## Key metrics

- **RSS footprint** - measured RSS on the 1GB VPS tier stays flat or shrinks release-over-release; regression tripwire at ~200MB idle.
- **30/90-day active-instance retention** - % of opt-in-telemetry instances still reporting 30 and 90 days after install.
- **Posting cadence among active instances** - median posts/week per opted-in instance; cross-referenced against retention to separate "kept running" from "actually used."
- **Opt-in install count** - leading reach signal from opt-in telemetry.

## V1 execution order

Detailed backlog: **`docs/ROADMAP.md`**.

1. **Federation integration & testing** — Close Federation Core U9 (real Mastodon interop on nitpub.com); fix gaps before new features.
2. **Admin V1 enablement audit** — Review every admin/author/CLI capability; prioritize P0 launch vs post-launch vs CLI-only.
3. **Moderation & trust** — Approval queue, allow-list, block/mute before public replies.
4. **Threading & comments** — Show moderated replies on permalinks (replies already stored).

Site content, themes, and the site editor are **done for now** unless the audit elevates a gap to P0.

## Tracks

### Federation Core

AP protocol implementation, thread/reply visibility, go-ap integration.

_Why it serves the approach:_ Visible replies is the gap WriteFreely and Mastodon both leave open.

### Content & Theming

Notes and long-form articles, custom domain, custom themes, PWA editor.

_Why it serves the approach:_ Both content types first-class, on your own domain — the core bet.

### Moderation & Trust

Approval queue for inbound replies, trusted/allow-list, block/mute.

_Why it serves the approach:_ Visible replies only work if spam/abuse from the open fediverse is contained.

### Lightweight Ops

RSS footprint discipline, single-binary deploy, budget-VPS target.

_Why it serves the approach:_ Ownership without Mastodon's ops weight is the differentiator vs. every heavier self-hosted option.

## Not working on

- Multi-user / multi-tenant.
- Native mobile app (PWA only).
- AT Protocol / Bluesky bridging - deferred, not committed to v1; revisit once federation core + moderation are solid.
