---
artifact_contract: ce-unified-plan/v1
artifact_readiness: implementation-ready
product_contract_source: ce-brainstorm
execution: code
title: Telemetry Admin Toggle - Plan
date: 2026-08-27
---

# Telemetry Admin Toggle - Plan

## Goal Capsule

**Objective:** Give an already-installed operator a way to opt in/out of version telemetry from the admin panel, not just at install time — closing the gap left by `docs/plans/2026-08-27-001-feat-version-telemetry-plan.md`, which shipped the backend (`GET`/`POST /api/admin/telemetry`) and the CLI (`nitpub telemetry enable/disable/status`) but deferred the frontend.

**Product authority:** Carries forward the parent telemetry plan's transparency stance — opt-in only, explicit at both onboarding (`nitpub install --with-telemetry`, shipped) and now the admin panel (this plan). Accepted as-is per session decision: retention/deletion policy on the receiver side stays out of scope, given the project is open source and the operator gets explicit control at both surfaces.

**Product Contract preservation:** Product Contract unchanged from the requirements-only version; this enrichment adds the Planning Contract, Implementation Units, and Verification Contract below.

## Problem Frame

`nitpub install --with-telemetry` and `nitpub telemetry enable/disable/status` (CLI) are the only ways to change telemetry opt-in today. An operator who skipped it at install, or wants to reconsider later, has no in-app way to do it — they'd need SSH + CLI access, which most self-hosters won't reach for. The backend already supports this (`internal/api/admin_telemetry.go`, `GET`/`POST /api/admin/telemetry` returning/accepting `{"enabled": bool}`); only the UI is missing.

## Requirements

**R1.** The admin panel's "System" section shows the current telemetry enabled/disabled state and a control to change it, alongside the existing version-check UI.

**R2.** A sentence of inline explanatory text (what data, where it goes) sits above the control — same substance as the install-wizard's opt-in prompt copy.

**R3.** Toggling calls `POST /api/admin/telemetry`; a failure (e.g. registration failure on first enable) shows inline error text and the control reverts to its prior state rather than silently appearing to have succeeded.

**R4.** No backend change — reads/writes the already-shipped `GET`/`POST /api/admin/telemetry` (`{"enabled": bool}`) exactly as it exists today.

## Scope Boundaries

**Out of scope (deliberately, this plan):**
- Confirm dialog / modal before enabling — plain switch, per session decision
- Surfacing the instance ID or any extra status beyond enabled/disabled — `GET` response stays `{enabled: bool}`; showing the instance ID later is a small separate follow-up, not folded in here
- Retention/deletion policy on the telemetry receiver — accepted as-is (private infra, out of this repo)
- Any change to the install-wizard prompt or CLI — both already ship and are unaffected

## Key Technical Decisions

**KTD1 — Mirror `VersionCheck.vue`'s fetch/status shape, not a new pattern.** (session-settled: user-directed — chosen over a confirm-dialog flow: plain switch + inline text matches the existing minimal admin-panel style and the install-wizard's own copy.) `web/src/components/VersionCheck.vue` is the direct sibling component in the same "System" section: a `ref` holding fetched state, a `text-muted` explanatory paragraph, `.status`/`.status.error` classes for state/error display. New `TelemetryToggle.vue` follows the same shape.

**KTD2 — Checkbox control, not a custom switch component.** `web/src/components/FederationSettings.vue`'s `.field.checkbox` (`<label class="field checkbox"><input type="checkbox" /><span>…</span></label>`) is the existing toggle-control convention in the admin panel — reuse the class, not a new "switch" widget. Unlike `FederationSettings.vue`'s batched save-button flow, this control fires immediately on change (KTD3), since enabling has a real side effect (registration) the operator should see resolve right away.

**KTD3 — Immediate `@change` handler, not a debounced/batched save.** On toggle: disable the input, `POST /api/admin/telemetry`, then either update to the new confirmed state or revert the checkbox and show the error (R3). No separate "Save" button — matches a live setting, not a form.

**KTD4 — Fetch/error conventions from `web/src/lib/adminSite.ts`.** `credentials: 'include'`, `Content-Type: application/json`, and on a non-ok response read `res.text()` for the body (the backend's `http.Error` calls write plain text, e.g. `"telemetry is not configured on this instance"`, `"registration failed: …"`) and surface it directly as the inline error — no generic "Something went wrong."

## Implementation Units

### U1. `web/src/lib/telemetry.ts` — fetch/set functions

**Goal:** Typed client functions for `GET`/`POST /api/admin/telemetry`, mirroring `lib/version.ts` and `lib/adminSite.ts`'s conventions.

**Requirements:** R3, R4, KTD4

**Dependencies:** none

**Files:**
- `web/src/lib/telemetry.ts` (new)

**Approach:**
1. `export type TelemetryStatus = { enabled: boolean }`.
2. `fetchTelemetryStatus(): Promise<TelemetryStatus>` — `GET /api/admin/telemetry`, `credentials: 'include'`; on non-ok, throw with a generic message (status fetch failures aren't expected to carry a useful body, per the handler's plain 401/404/500 responses).
3. `setTelemetryEnabled(enabled: boolean): Promise<TelemetryStatus>` — `POST /api/admin/telemetry`, `credentials: 'include'`, `Content-Type: application/json`, body `JSON.stringify({ enabled })`; on non-ok, throw `new Error(await res.text() || 'Failed to update telemetry setting')` (KTD4); on ok, return the parsed `TelemetryStatus`.

**Patterns to follow:** `web/src/lib/version.ts` (fetch shape), `web/src/lib/adminSite.ts:24-40` (`saveManifest`'s error-body handling).

**Test scenarios:**
- No frontend test infra exists for `web/src/lib/*.ts` today (no test runner wired for this directory — confirmed by scanning for existing `*.test.ts` files alongside `lib/version.ts`, `lib/adminSite.ts`, none found). `Test expectation: none -- matches existing lib/ convention, no test harness to add one to without a separate, unrelated setup task.`

**Verification:** `npm run build` (or `tsc --noEmit`) in `web/` succeeds with no type errors.

---

### U2. `web/src/components/TelemetryToggle.vue`

**Goal:** The toggle component itself — status display, checkbox, inline explanatory text, inline error, immediate on-change POST.

**Requirements:** R1, R2, R3, KTD1, KTD2, KTD3

**Dependencies:** U1

**Files:**
- `web/src/components/TelemetryToggle.vue` (new)

**Approach:**
1. On mount, call `fetchTelemetryStatus()` into a `status` ref (mirrors `VersionCheck.vue`'s `result` ref); render nothing conclusive until it resolves (a brief "Loading…" or blank, matching `VersionCheck`'s `v-if="!result"` pattern is fine — this is a low-stakes loading state, not worth over-designing).
2. Explanatory paragraph (`text-muted`, R2) above the checkbox, always visible regardless of state — one sentence: what's reported (version, instance ID, OS/arch) and that it's fully inert until enabled. Reuse the install-wizard's confirm copy for consistency rather than inventing new wording (`cmd/nitpub/install_cmd.go`'s telemetry `huh.NewConfirm().Title(...)` string is the source phrasing to adapt).
3. `<label class="field checkbox">` (KTD2) bound to a local `enabled` ref seeded from `status.enabled`; `@change` handler (not `v-model` alone, since a plain `v-model` wouldn't let the change be vetoed on failure):
   - set `saving = true`
   - call `setTelemetryEnabled(newValue)`
   - on success: update `status`/`enabled` from the response, clear any prior error
   - on failure: revert `enabled` to `status.enabled` (the last confirmed state), set `error` to the caught message (R3)
   - `saving = false`; disable the checkbox input while `saving`
4. `.status.error` paragraph for `error`, matching `VersionCheck.vue`'s error style exactly (same CSS class names, so no new error-styling rules needed).

**Technical design:**
```
onMounted: status = await fetchTelemetryStatus(); enabled = status.enabled

onChange(newValue):
  saving = true; error = ''
  try:
    status = await setTelemetryEnabled(newValue)
    enabled = status.enabled
  catch e:
    enabled = status.enabled  // revert
    error = e.message
  finally:
    saving = false
```

**Patterns to follow:** `web/src/components/VersionCheck.vue` (overall shape, `.status`/`.status.error` styling — reuse the scoped-style classes, don't redefine them locally if a shared class already exists at that specificity); `web/src/components/FederationSettings.vue:226-228` (`.field.checkbox` markup).

**Test scenarios:**
- No component test harness exists in `web/` today (confirmed alongside U1 — no `*.spec.ts`/`*.test.ts` for existing components like `VersionCheck.vue` or `FederationSettings.vue`). `Test expectation: none -- matches existing web/src/components/ convention.` Verification is manual (see Verification Contract) — this is a UI unit with real branching (enable/disable/error/revert) that would benefit from tests if a harness existed, but adding one is out of scope for a single toggle component.

**Verification:** Manual smoke in a running dev instance (`make dev-web` against a local `httptest.Server` telemetry backend or a real one) — confirmed via the plan's Verification Contract below.

---

### U3. Wire into `AdminShellView.vue`

**Goal:** Render `TelemetryToggle` in the existing "System" section, next to `VersionCheck`.

**Requirements:** R1

**Dependencies:** U2

**Files:**
- `web/src/views/AdminShellView.vue`

**Approach:**
1. Import `TelemetryToggle` alongside the existing `VersionCheck` import (`web/src/views/AdminShellView.vue:11`).
2. Add it to the `v-else-if="activeSection === 'system'"` render branch (`web/src/views/AdminShellView.vue:115`), directly after `<VersionCheck />` — no new section id, no nav change, matches the confirmed scope (KTD1).

**Patterns to follow:** The existing `<VersionCheck v-else-if="activeSection === 'system'" />` line — same conditional, same section.

**Test scenarios:** `Test expectation: none -- a one-line template addition wiring an already-tested-by-U2 component into an existing conditional branch; no new logic.`

**Verification:** `npm run build` succeeds; manual check that the System section renders both components.

## Verification Contract

- `cd web && npm run build` succeeds (TypeScript + Vite build, no errors).
- `go build ./...` and `go test ./...` at repo root still pass (no backend files touched — regression guard only).
- Manual smoke against a real or `httptest.Server`-backed telemetry endpoint:
  - Fresh instance with telemetry off: toggle shows off, explanatory text visible.
  - Enable: toggle flips, a `POST` fires, endpoint returns `{"enabled": true}`, state updates, no error shown.
  - Enable against a failing registration endpoint: toggle reverts to off, inline error text shows the server's message.
  - Disable: toggle flips off, `POST {"enabled": false}` fires, state updates.
  - Reload the page after enabling: `GET` reflects the persisted `enabled: true` (confirms no client-only state).

## Definition of Done

- All three implementation units merged.
- `npm run build` and `go build`/`go test` pass.
- Manual smoke scenarios above all verified in a running instance.

## Sources & Research

- Repo: `web/src/components/VersionCheck.vue`, `web/src/components/FederationSettings.vue`, `web/src/lib/version.ts`, `web/src/lib/adminSite.ts`, `web/src/views/AdminShellView.vue`, `internal/api/admin_telemetry.go`, `cmd/nitpub/install_cmd.go` (opt-in prompt copy), `docs/plans/2026-08-27-001-feat-version-telemetry-plan.md` (parent plan).
