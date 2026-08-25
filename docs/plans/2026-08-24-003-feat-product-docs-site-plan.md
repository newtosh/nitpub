---
title: "Product Docs Site - Plan"
date: 2026-08-24
type: feat
topic: product-docs-site
artifact_contract: ce-unified-plan/v1
artifact_readiness: implementation-ready
product_contract_source: ce-brainstorm
execution: code
---

# Product Docs Site - Plan

## Goal Capsule

- **Objective:** Ship a public product marketing site on `www.nitpub.com` and canonical install documentation on `docs.nitpub.com` (in the public `nitpub` repo), after polishing the apex federated demo and capturing screenshots — without moving `@nit@nitpub.com` off `nitpub.com`.
- **Product authority:** This plan owns area B (product + docs site) plus in-package apex demo polish and screenshots. Area A (OSS docs package) and install/Releases are already shipped context. Area C (demo → `blog.*` / apex product cutover) is not active scope.
- **Product Contract preservation:** R/A/F/AE IDs preserved; R13 narrowed to docs-only `llms.txt` to match settled KTD5; Key Decision changelog wording aligned to build-time bake (KTD4). Planning settled VitePress (`docsite/`), Cloudflare Pages dual-project, build-time changelog (scoping-confirmed).
- **Open blockers:** None.
- **Stop when:** R1–R15 and AE1–AE6 are met by live www/docs, polished apex demo, README hub, and unchanged federation identity.

## Product Contract

### Summary

Publish `www.nitpub.com` (sell landing + `/changelog` fed by GitHub Releases) and `docs.nitpub.com` (canonical installer docs + `llms.txt`) from the public `nitpub` repository, using a split stack (custom marketing + OSS docs frontend). First bring the apex demo into good working order, capture screenshots for the landing, then ship. The federated demo identity stays on `nitpub.com` forever for this instance.

### Problem Frame

nitpub is now a public OSS product with a root README golden path and GitHub Releases, but there is no public marketing or docs hostname — only the federated demo blog on `nitpub.com` (full of probe content) and GitHub. Installers and curious visitors lack a product face and a durable docs home; the README cannot stay the long-term human install surface once a docs site exists. Moving the demo off apex to free the product homepage would risk breaking `@nit@nitpub.com` / `https://nitpub.com/actor`, which remotes already know.

### Key Decisions

- **Subdomain pair, apex stays federated demo** `(session-settled: user-directed — chosen over apex-as-product or demo→blog.* cutover: protects WebFinger/actor/post IDs on nitpub.com)`. Governs R1, R2, R11, R12.
- **Site lives in public `nitpub` repo** `(session-settled: user-directed — chosen over a separate nitpub-docs repo: one OSS home, less install/docs drift)`. Governs R3.
- **Split stack: custom marketing on www, OSS docs framework on docs** `(session-settled: user-directed — chosen over one docs framework for both or fully custom docs: sell page free of docs chrome; docs get search/nav from a known tool)`. Governs R4, R5.
- **Docs are canonical for install; README becomes a short hub** `(session-settled: user-directed — chosen over README-canonical or dual-write: one human install story)`. Governs R6, R7, R8.
- **www v1 = landing only + `/changelog`** `(session-settled: user-directed — chosen over vs/About pages and over naming the route /releases: minimal sell surface; changelog naming)`. Governs R4, R9.
- **`/changelog` lists GitHub Releases (build-time baked into www)** `(session-settled: user-directed — chosen over curated in-repo changelog: Release bodies become the product notes going forward; bake semantics in KTD4)`. Governs R9, R10.
- **Docs v1 = installer pack** `(session-settled: user-directed — chosen over golden-path-only or installer+Advanced: port current README install story without dragging deploy ops into v1)`. Governs R6, R8.
- **Include `llms.txt` in v1** `(session-settled: user-directed — chosen over deferring agent surfaces)`. Governs R13.
- **Demo polish + smoke (+ bounded UI/copy fixes) then screenshots, in this package** `(session-settled: user-directed — chosen over screenshots-as-is, separate prior polish, or site-without-shots: Live demo and landing must not ship embarrassing probe content)`. Governs R11, R12, R14.
- **Never move `@nit@nitpub.com` off apex for this instance** `(session-settled: user-directed — chosen over UI≠AP host split, classic domain move, or new demo identity: federation protection)`. Governs R1, R15.

<!-- ce-section: work-relationships -->
### How This Work Fits Together

This plan owns **B — product + docs site** (plus in-package apex demo polish/screenshots). The broader breakdown below is the current understanding, not a committed roadmap.

- **A — OSS docs package** (shipped): root README, LICENSE, SECURITY, SUPPORT; README currently holds the golden path this plan will demote to a hub.
  - Enables B (public install story already exists to move onto docs).
- **Install CLI + Releases** (shipped): one-liner, `nitpub install`/`doctor`, GitHub Releases — content sources for docs and `/changelog`.
  - Enables B.
- **C — Demo → `blog.*` / apex as product home**
  - Can proceed independently of B only as a future product choice; **not planned for this instance** while federation stays on `nitpub.com` (Key Decision above).
  - Still to decide for other instances or a later identity strategy — out of active Requirements here.

### Actors

- A1. Prospective installer — self-hosting indie blogger evaluating or installing nitpub.
- A2. Curious visitor / agent — lands on www or fetches `llms.txt` / docs without installing yet.
- A3. Demo instance — live federated blog at `nitpub.com` (`@nit@nitpub.com`), polished as the Live demo target.
- A4. GitHub Releases — source of truth for version artifacts and changelog bodies shown on www.

### Requirements

**Hosts and packaging**

- R1. `www.nitpub.com` serves the product marketing site; `docs.nitpub.com` serves documentation; `nitpub.com` remains the federated demo blog and ActivityPub origin for `@nit@nitpub.com`.
- R2. Human visitors to www and docs are not required to use the apex demo to install or learn about the product.
- R3. Marketing and docs site sources live in the public `nitpub` repository (not a separate docs-only repo).

**www marketing**

- R4. www v1 is a single landing: brand/pitch, feature list, and CTAs to docs (install), Live demo (apex), and GitHub — no comparison or About pages in v1.
- R5. Landing includes example screenshots captured after the apex demo meets R11–R12.
- R9. www exposes `/changelog` listing published GitHub Releases for `newtosh/nitpub` with each entry’s Release title, tag, date, and body.
- R10. Going forward, GitHub Release bodies are the human-readable product changelog; thin historical bodies are acceptable in v1 unless optionally backfilled outside this contract’s minimum.

**docs**

- R6. `docs.nitpub.com` is the canonical human install documentation for the golden path, Federation expandable, Analytics optional, Updates, and Manual download (the current root README installer pack).
- R7. After docs ship, the root README becomes a short hub (positioning + pointers to docs, demo, Releases, community files) and does not remain a second full golden path.
- R8. Advanced/ops content that today lives in `deploy/README.md` stays out of docs v1 (link or leave in-repo); not a docs rewrite of the full ops manual.
- R13. Docs publish `llms.txt` at `https://docs.nitpub.com/llms.txt` summarizing product bounds, install entrypoints, and links into canonical docs for agents (not on www).

**Demo polish and screenshots**

- R11. Before screenshots or treating Live demo as shippable for this package, replace probe/test posts on the apex demo with a small intentional content set (at least one note, one article, and a sensible About/site chrome).
- R12. Run a short smoke checklist on the polished demo (home, permalink, comments affordance where federation-shared, federated identity visible) and apply bounded UI/copy fixes discovered during that pass — not an open-ended QA bug bash.
- R14. Capture screenshots from the polished demo for use on the www landing (R5).

**Federation boundary**

- R15. This work does not change the ActivityPub actor IRI, WebFinger subject, or post object ID host for the nitpub.com demo instance.

### Key Flows

- F1. Installer path
  - **Trigger:** A1 arrives via www or GitHub.
  - **Actors:** A1
  - **Steps:** Reads pitch on www → follows Install CTA to docs → completes golden path from docs → optionally Federation/Analytics expandables.
  - **Outcome:** First published note without needing the README as the full install guide.
  - **Covers:** R2, R4, R6, R7

- F2. Changelog browse
  - **Trigger:** A1 or A2 opens `/changelog` on www.
  - **Actors:** A1, A2, A4
  - **Steps:** Page lists Releases from GitHub; each shows notes from the Release body; links to assets/tag as appropriate.
  - **Outcome:** Version history is readable in one place without hunting tags.
  - **Covers:** R9, R10

- F3. Demo credibility
  - **Trigger:** Maintainer prepares to ship www.
  - **Actors:** A3
  - **Steps:** Content reset → smoke (+ bounded UI/copy fixes) → screenshots → Live demo CTA points at polished apex.
  - **Outcome:** Landing and Live demo show intentional product, not U9 probes.
  - **Covers:** R5, R11, R12, R14

- F4. Agent orientation
  - **Trigger:** A2 fetches `llms.txt`.
  - **Actors:** A2
  - **Steps:** Reads product bounds and install pointers; follows links into docs rather than inventing APIs.
  - **Outcome:** Agent has a single fetchable index for correct install guidance.
  - **Covers:** R13

### Acceptance Examples

- AE1. Given docs are live, when an installer follows only docs.nitpub.com (not the old full README golden path), then they can complete install → first published note. Covers R6, R7.
- AE2. Given www is live, when a visitor opens the landing, then they see pitch, features, screenshots from the polished demo, and working CTAs to docs, Live demo (apex), and GitHub. Covers R4, R5, R14.
- AE3. Given at least one GitHub Release exists, when a visitor opens `/changelog`, then they see that Release’s tag/date/body from GitHub. Covers R9.
- AE4. Given the apex demo was probe-heavy, when this package is marked done, then the Live demo no longer leads with U9 probe posts and smoke checklist items pass. Covers R11, R12.
- AE5. Given federation remotes already know `@nit@nitpub.com`, when this package ships, then WebFinger and `https://nitpub.com/actor` remain on nitpub.com unchanged. Covers R1, R15.
- AE6. Given an agent fetches `llms.txt`, when it follows the documented install pointers, then those URLs resolve to the live docs install content. Covers R13.

### Scope Boundaries

**Deferred for later**

- Comparison (“vs”) and About pages on www
- Advanced/ops docs migration from `deploy/README.md` onto docs.*
- `sitemap.xml` and richer agent surfaces beyond `llms.txt`
- Backfilling historical Release bodies (optional; not required for v1 minimum)
- UI host ≠ AP host split; any demo move to `blog.*`; apex-as-product homepage

**Outside this product’s identity**

- Changing `@nit@nitpub.com` / actor IRI to free the apex
- Separate `nitpub-docs` repository
- DigitalOcean Marketplace 1-Click (north star only)

### Success Criteria

- www and docs are publicly reachable on their hostnames with the v1 content above.
- README is a hub; docs own the installer pack.
- Apex demo is intentional + smoke-passed; landing uses real screenshots.
- `/changelog` reflects GitHub Releases; `llms.txt` is fetchable.
- Demo federation identity on nitpub.com is unchanged.

### Assumptions

- Cloudflare account can attach custom domains `www.nitpub.com` and `docs.nitpub.com` to two Pages projects; apex `nitpub.com` DNS/Caddy for the demo/AP stays on the VPS.
- Going forward, Release authors will write usable Release bodies so `/changelog` stays valuable.
- Thin historical Release bodies (e.g. v0.1.5) are acceptable on `/changelog` for v1.
- Site packages land in the public `newtosh/nitpub` tree (sync from `nitpub-dev` as today); Cloudflare Pages connects to the **public** repo.

### Outstanding Questions

**Deferred to Planning** — resolved in Planning Contract KTDs below (VitePress, Cloudflare Pages, build-time changelog, docs-only `llms.txt`, screenshot paths).

**Deferred to implementation**

- Exact visual design tokens for www (match product feel without inventing a second brand system).
- Whether CF Pages uses native Git integration vs GitHub Actions + Wrangler upload (either satisfies R1/R3).

---

## Planning Contract

### Key Technical Decisions

- KTD1. **VitePress for `docsite/`** `(session-settled: user-directed — chosen over Starlight/Docusaurus/custom SvelteKit: Vue/Vite aligned with web/, enough for installer pack)`. Package dir is `docsite/` so it does not collide with `docs/plans/`. Hostname remains `docs.nitpub.com`. Governs R6, R8, R13.
- KTD2. **Lightweight Vite MPA for `www/`** (not a second docs framework) — landing + real static `/changelog/index.html` (do not rely on SPA fallback alone on Pages); Vue optional if it speeds the changelog page. Governs R4, R5, R9.
- KTD3. **Cloudflare Pages monorepo: two projects** `(session-settled: user-directed — chosen over GitHub Pages, Vercel, Netlify, VPS static: dual custom domains + free static hosting)`. Project A root `www/` → `www.nitpub.com` (build `npm ci && npm run build`, output `dist/`); project B root `docsite/` → `docs.nitpub.com` (build `npm ci && npm run build` / VitePress, output `docs/.vitepress/dist`). Apex remains VPS. Governs R1, R3.
- KTD4. **Changelog data at build time** — fetch `GET https://api.github.com/repos/newtosh/nitpub/releases` during `www` build with a descriptive `User-Agent` and `GITHUB_TOKEN` when present (set as a Pages/CI secret on `nitpub-www`); bake into static assets. No runtime GitHub dependency for visitors. On fetch failure in CI, fail the build (do not ship an empty silent changelog). Optionally commit a generated `www/data/releases.json` for local offline builds when required-fetch is off. Governs R9, R10.
- KTD5. **`llms.txt` on docs only** — served as a static file at `https://docs.nitpub.com/llms.txt` (VitePress `public/llms.txt` or equivalent). Governs R13.
- KTD6. **Screenshots in `www/public/screenshots/`** (committed PNGs/WebPs) after U1 polish; wire into landing. Governs R5, R14.
- KTD7. **Demo polish is content/ops on apex** — use Admin → Site / import or `scripts/seed-*` patterns; delete or unpublish probe posts; do not change `domain`/`base_url`/actor. Bounded UI/copy fixes only when found during smoke. Governs R11, R12, R15.
- KTD8. **DNS cutover** — point `www` and `docs` at Cloudflare Pages per CF custom-domain instructions; leave apex A/AAAA on the VPS. After Pages owns `www`/`docs`, remove `www` from the live nitpub.com VPS Caddy site block if present (e.g. `cutover-domain.sh` may have installed `$DOMAIN, www.$DOMAIN` — apex-only afterward). DNS-away alone is not enough if Caddy still claims `www`. Governs R1, R15.

### Sequencing

1. U1 Apex demo polish + smoke
2. U2 Screenshots into `www/public/screenshots/`
3. U3 `www/` marketing + changelog (can scaffold before U2; needs U2 assets before ship)
4. U4 `docsite/` VitePress + installer content + `llms.txt`
5. U5 Cloudflare Pages projects + DNS custom domains
6. U6 README hub rewrite (depends U4, U5 live URLs)

### High-Level Technical Design

```text
newtosh/nitpub (public)
  www/       → Vite static (landing, /changelog)
  docsite/   → VitePress (installer pack + public/llms.txt)
             (dir name avoids clashing with docs/plans/)

Cloudflare Pages
  nitpub-www  (root www/)     ←── www.nitpub.com
  nitpub-docs (root docsite/) ←── docs.nitpub.com

VPS (unchanged AP)
  nitpub.com → Caddy → :8080  (@nit@nitpub.com)

Build www:
  fetch GitHub Releases → bake JSON → static /changelog
```

### Sources / Research

- `deploy/Caddyfile` — example apex reverse_proxy only; live VPS Caddy (via `cutover-domain.sh`) may still list `www.$DOMAIN` — U5/KTD8 remove that for nitpub.com after Pages cutover.
- `README.md` L19–116 — installer pack to port into VitePress.
- `deploy/README.md` — Advanced/ops remains linked, not migrated (R8).
- Cloudflare Pages monorepos: multiple projects per repo with distinct root directories.
- Live demo currently probe-heavy (U9 import/article probe posts) — U1 target.

---

## Implementation Units

### U1. Apex demo polish and smoke

- **Goal:** Replace probe content with intentional demo content; smoke-check; bounded UI/copy fixes only as needed.
- **Requirements:** R11, R12, R15; AE4, AE5
- **Dependencies:** None
- **Files:**
  - Ops on VPS data (`{data_dir}/site/`, posts via Admin or import) — not necessarily repo commits
  - Possibly: `scripts/example-site/` or a new `scripts/demo-site/` if a reproducible seed helps (optional)
  - Modify only if smoke finds a product UI/copy bug worth a small code fix under R12
- **Approach:**
  1. Inventory and remove/unpublish U9 probe posts and junk pages.
  2. Publish a small set: ≥1 note, ≥1 article, About (and nav) that reads as a product demo.
  3. Smoke: home, permalink, comments affordance on a federation-shared post, `@nit@nitpub.com` / footer identity visible.
  4. Confirm WebFinger + `/actor` still on `nitpub.com` (R15).
- **Patterns to follow:** `docs/site-content.md`, `scripts/seed-site-example.sh`, Admin → Site.
- **Test scenarios:**
  - Covers AE4. Home no longer leads with “U9 … probe” titles.
  - Covers AE5. `acct:nit@nitpub.com` still resolves to `https://nitpub.com/actor`.
  - Smoke checklist items pass on production apex.
- **Verification:** Manual browser smoke on `https://nitpub.com`; WebFinger curl check.

### U2. Marketing screenshots

- **Goal:** Capture polished-demo screenshots and commit them for the landing.
- **Requirements:** R5, R14
- **Dependencies:** U1
- **Files:**
  - Create: `www/public/screenshots/` (home, note or article permalink, optional compose/comments — at least two shots)
- **Approach:**
  1. Capture desktop (and optionally one mobile) shots from the polished apex.
  2. Optimize to WebP/PNG reasonable size; commit under `www/public/screenshots/`.
- **Patterns to follow:** Prefer Playwright or OS tools; do not commit `.playwright-mcp/` scratch paths.
- **Test scenarios:**
  - At least two screenshot files exist and are referenced by the landing after U3.
- **Verification:** Files present in-repo; visually show intentional content (not probes).

### U3. www marketing site + changelog

- **Goal:** Vite static site: landing (R4/R5) and `/changelog` from build-time GitHub Releases (R9/R10).
- **Requirements:** R3, R4, R5, R9, R10; AE2, AE3; F2
- **Dependencies:** U2 for ship-ready visuals (scaffold may start earlier)
- **Files:**
  - Create: `www/` (package.json, vite.config, index/landing, changelog route/page, styles)
  - Create: `www/scripts/fetch-releases.mjs` (or similar) + optional `www/data/releases.json`
  - Create: `www/public/screenshots/` (from U2)
- **Approach:**
  1. Scaffold Vite app; landing with pitch, features, screenshot gallery, CTAs → `https://docs.nitpub.com`, `https://nitpub.com`, `https://github.com/newtosh/nitpub`.
  2. Prebuild: fetch Releases API (User-Agent + optional `GITHUB_TOKEN`); write data file; render static `/changelog/index.html` with title, tag, date, body (markdown→HTML sanitized).
  3. CI/local: fail build if fetch fails when `CI=true` or `--require-releases`.
- **Patterns to follow:** Keep visual language distinct from the blog PWA; avoid generic purple/AI-slop defaults per project frontend rules when designing.
- **Test scenarios:**
  - Covers AE3. With a fixture releases JSON, `/changelog` lists tag + body.
  - Landing includes screenshot `<img>` paths under `/screenshots/`.
  - Build fails when required fetch fails in CI mode.
- **Verification:** `npm run build` in `www/` succeeds; preview shows landing + changelog.

### U4. docs VitePress + installer pack + llms.txt

- **Goal:** Canonical install docs on VitePress; port README installer pack; ship `llms.txt`.
- **Requirements:** R6, R8, R13; AE1, AE6; F1, F4
- **Dependencies:** None (content can land before DNS)
- **Files:**
  - Create: `docsite/` VitePress app (package root; CF Pages root `docsite/` — avoids clashing with `docs/plans/`)
  - Create: markdown pages for golden path, Federation, Analytics, Updates, Manual download; link Advanced → `deploy/README.md` on GitHub
  - Create: `docsite/public/llms.txt`
- **Approach:**
  1. Scaffold VitePress; configure `site` title, nav, sidebar for installer pack.
  2. Port content from `README.md` L19–116; keep commands accurate to current install CLI.
  3. Write `llms.txt` with product bounds, install one-liner, links to docs pages, demo, GitHub; no invented APIs.
- **Patterns to follow:** Current README tone; VitePress default theme is fine for v1.
- **Test scenarios:**
  - Covers AE1. Docs pages include one-liner + first-note steps.
  - Covers AE6. `llms.txt` is in build output and references real doc paths.
  - `npm run build` in `docsite/` succeeds.
- **Verification:** Local VitePress preview; spot-check against live install commands.

### U5. Cloudflare Pages + DNS

- **Goal:** Two Pages projects with custom domains; apex federation untouched.
- **Requirements:** R1, R3, R15; AE5
- **Dependencies:** U3, U4
- **Files:**
  - Possibly: `docsite/wrangler.toml` / `www/wrangler.toml` or CF dashboard-only config
  - Modify: `deploy/README.md` — note that `www`/`docs` are Pages, not Caddy blog hosts
  - Optional: `.github/workflows/pages-*.yml` only if not using CF native Git builds
- **Approach:**
  1. Connect public `newtosh/nitpub` to Cloudflare; create `nitpub-www` (root `www/`, output `dist/`) and `nitpub-docs` (root `docsite/`, output `.vitepress/dist`) with build commands from KTD3.
  2. Attach custom domains `www.nitpub.com` and `docs.nitpub.com`; follow CF DNS (proxied CNAME/A as instructed).
  3. Ensure apex `nitpub.com` still points at VPS; verify WebFinger unchanged after cutover.
  4. On the VPS, remove `www` from the nitpub.com Caddy site block if present so only apex reverse-proxies the blog; leave customer install docs free to teach optional www→VPS for *other* domains.
- **Patterns to follow:** Cloudflare Pages monorepo docs; existing multi-host DNS notes in `deploy/README.md`.
- **Test scenarios:**
  - `https://www.nitpub.com/` returns marketing landing; `/changelog` is a real static path.
  - `https://docs.nitpub.com/` returns VitePress docs; `/llms.txt` 200.
  - Covers AE5. Apex WebFinger unchanged.
- **Verification:** Live HTTP checks + WebFinger curl.

### U6. README hub rewrite

- **Goal:** Demote README to short hub pointing at docs, demo, Releases, community.
- **Requirements:** R7; AE1
- **Dependencies:** U4, U5 (stable docs URLs)
- **Files:**
  - Modify: `README.md`
- **Approach:**
  1. Keep positioning + live demo + who it’s for.
  2. Replace full golden path with link to `https://docs.nitpub.com` (install).
  3. Keep LICENSE/SECURITY/SUPPORT pointers; link Advanced to `deploy/README.md`.
- **Patterns to follow:** Prior OSS docs package hub intent; do not leave a second full install checklist.
- **Test scenarios:**
  - README has no duplicate full one-liner→first-note procedure (or only a one-line pointer).
  - Docs URL is present and correct.
- **Verification:** README review + link check.

---

## Verification Contract

| Scope | Command / check | Applies to |
|---|---|---|
| www build | `npm ci && npm run build` in `www/` (output `dist/`) | U3 |
| docs build | `npm ci && npm run build` in `docsite/` (VitePress output `docs/.vitepress/dist`) | U4 |
| Changelog fixture | Unit/script test or build with fixture `releases.json` | U3 |
| Apex smoke | Browser: home, permalink, comments affordance, identity | U1 |
| WebFinger | `curl` WebFinger + `/actor` on `nitpub.com` | U1, U5 |
| Live hosts | `https://www.nitpub.com/`, `/changelog`, `https://docs.nitpub.com/`, `/llms.txt` | U5 |
| README hub | Manual: no second full golden path; docs link present | U6 |

No Go test suite is required for pure static packages; prefer small Node tests for the releases fetch script when practical.

## Definition of Done

- All Verification Contract checks pass.
- AE1–AE6 satisfied on production hostnames.
- Probe posts are gone from the Live demo; screenshots on www match the polished demo.
- `@nit@nitpub.com` / actor IRI unchanged.
- README is a hub; docs own the installer pack.
- No abandoned alternate hosts (Pages/Vercel experiments) left half-configured in-repo.
