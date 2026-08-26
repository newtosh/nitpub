<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

type Breakdown = { name: string; count: number; code?: string }
type DailyPoint = { day: string; count: number }
type Stats = {
  total_pageviews: number
  daily_totals: DailyPoint[]
  top_pages: Breakdown[]
  top_referrers: Breakdown[]
  top_locations: Breakdown[]
  goatcounter_url: string
}
type Window = '24h' | '7d' | '30d'

const SPARK_W = 180
const SPARK_H = 44

const WINDOWS: { id: Window; label: string }[] = [
  { id: '24h', label: 'Last 24 hours' },
  { id: '7d', label: 'Last 7 days' },
  { id: '30d', label: 'Last 30 days' },
]

const window = ref<Window>('7d')
const loading = ref(true)
const error = ref('')
const stats = ref<Stats | null>(null)

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await fetch(`/api/admin/analytics?window=${window.value}`, { credentials: 'include' })
    if (res.status === 404) {
      error.value = 'Analytics is disabled for this instance.'
      return
    }
    if (!res.ok) {
      error.value = 'Could not load analytics — GoatCounter may be unreachable.'
      return
    }
    stats.value = await res.json()
  } catch {
    error.value = 'Could not load analytics — GoatCounter may be unreachable.'
  } finally {
    loading.value = false
  }
}

function selectWindow(next: Window) {
  window.value = next
  load()
}

function barWidth(count: number, rows: Breakdown[]): number {
  const max = rows.reduce((m, r) => Math.max(m, r.count), 0)
  return max === 0 ? 0 : Math.round((count / max) * 100)
}

// Internal/self-check paths that show up as noise in a low-traffic blog's
// analytics: the admin's own dashboard visits, auth flow, and unmatched
// (404-ish) requests GoatCounter still counted as a pageview. Labeled, not
// filtered — the count stays honest, just legible.
const INTERNAL_PATH_PREFIXES = ['/admin', '/login', '/logout', '/verify-']

function pageLabel(row: Breakdown): { text: string; badge: string | null } {
  if (INTERNAL_PATH_PREFIXES.some((p) => row.name.startsWith(p))) {
    return { text: row.name, badge: 'self' }
  }
  return { text: row.name, badge: null }
}

// ISO 3166-1 alpha-2 -> flag emoji via regional indicator symbols (each
// letter A-Z maps 1:1 onto U+1F1E6-U+1F1FF). No lookup table, no dep.
function flagEmoji(code?: string): string {
  if (!code || code.length !== 2) return ''
  const upper = code.toUpperCase()
  const points = [...upper].map((c) => 0x1f1e6 + c.charCodeAt(0) - 65)
  if (points.some((p) => p < 0x1f1e6 || p > 0x1f1ff)) return ''
  return String.fromCodePoint(...points)
}

function referrerLabel(row: Breakdown): { text: string; badge: string | null } {
  if (row.name === '') {
    return { text: 'Direct / no referrer', badge: 'unknown' }
  }
  return { text: row.name, badge: null }
}

// Sparkline geometry for the total-pageviews trend, built from the same
// window's daily_totals — no extra request. Needs at least 2 points to
// draw a line; short windows (e.g. 24h) may only return 1, which hides
// the sparkline rather than drawing a degenerate flat/dot line.
const sparkline = computed(() => {
  const points = stats.value?.daily_totals ?? []
  if (points.length < 2) return null
  const max = Math.max(...points.map((p) => p.count), 1)
  const stepX = SPARK_W / (points.length - 1)
  const coords = points.map((p, i) => {
    const x = i * stepX
    const y = SPARK_H - (p.count / max) * SPARK_H
    return { x, y }
  })
  const line = coords.map((c, i) => `${i === 0 ? 'M' : 'L'}${c.x.toFixed(1)},${c.y.toFixed(1)}`).join(' ')
  const fill = `${line} L${SPARK_W},${SPARK_H} L0,${SPARK_H} Z`
  const last = coords[coords.length - 1]
  return { line, fill, lastX: last.x, lastY: last.y }
})

// Trend delta: percent change between the first and second half of the
// same window's daily totals — no separate "prior period" fetch.
const trendDeltaPct = computed(() => {
  const counts = stats.value?.daily_totals.map((p) => p.count) ?? []
  if (counts.length < 2) return null
  const mid = Math.floor(counts.length / 2)
  const sum = (arr: number[]) => arr.reduce((a, b) => a + b, 0)
  const first = sum(counts.slice(0, mid))
  const second = sum(counts.slice(mid))
  if (first === 0) return null
  return Math.round(((second - first) / first) * 100)
})

onMounted(load)
</script>

<template>
  <div class="analytics-dashboard stack">
    <div class="analytics-window-switch">
      <button
        v-for="w in WINDOWS"
        :key="w.id"
        type="button"
        :class="{ active: window === w.id }"
        :disabled="loading"
        @click="selectWindow(w.id)"
      >
        {{ w.label }}
      </button>
    </div>
    <p v-if="loading" class="status">Loading…</p>
    <p v-else-if="error" class="status error">{{ error }}</p>
    <template v-else-if="stats">
      <div class="analytics-total">
        <div>
          <div class="analytics-total-figures">
            <span class="analytics-total-value">{{ stats.total_pageviews }}</span>
            <span class="analytics-total-label">pageviews</span>
          </div>
          <div v-if="trendDeltaPct !== null" class="analytics-trend-delta" :class="{ down: trendDeltaPct < 0 }">
            {{ trendDeltaPct >= 0 ? '▲' : '▼' }} {{ Math.abs(trendDeltaPct) }}% within this window
          </div>
        </div>
        <svg v-if="sparkline" class="analytics-sparkline" :width="SPARK_W" :height="SPARK_H" :viewBox="`0 0 ${SPARK_W} ${SPARK_H}`">
          <line x1="0" :y1="SPARK_H / 2" :x2="SPARK_W" :y2="SPARK_H / 2" class="analytics-sparkline-grid" />
          <path :d="sparkline.fill" class="analytics-sparkline-fill" />
          <path :d="sparkline.line" class="analytics-sparkline-line" />
          <circle :cx="sparkline.lastX" :cy="sparkline.lastY" r="3" class="analytics-sparkline-dot" />
        </svg>
      </div>

      <div class="analytics-breakdown">
        <h3>Top pages</h3>
        <p v-if="stats.top_pages.length === 0" class="status">No data yet.</p>
        <table v-else class="analytics-table">
          <tbody>
            <!-- name is visitor-controlled (a requested path) — plain text
                 interpolation only, never v-html or a rendered link. -->
            <tr v-for="row in stats.top_pages" :key="row.name">
              <td>
                <div class="analytics-bar-row">
                  <span class="analytics-bar" :style="{ width: barWidth(row.count, stats.top_pages) + '%' }" />
                  <span class="analytics-bar-label">
                    {{ pageLabel(row).text }}
                    <span v-if="pageLabel(row).badge" class="analytics-badge">{{ pageLabel(row).badge }}</span>
                  </span>
                </div>
              </td>
              <td class="analytics-count">{{ row.count }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="analytics-breakdown">
        <h3>Top referrers</h3>
        <p v-if="stats.top_referrers.length === 0" class="status">No data yet.</p>
        <table v-else class="analytics-table">
          <tbody>
            <!-- name is visitor-controlled (a Referer header) — plain text
                 interpolation only, never v-html or a rendered link. -->
            <tr v-for="row in stats.top_referrers" :key="row.name">
              <td>
                <div class="analytics-bar-row">
                  <span class="analytics-bar" :style="{ width: barWidth(row.count, stats.top_referrers) + '%' }" />
                  <span class="analytics-bar-label">
                    {{ referrerLabel(row).text }}
                    <span v-if="referrerLabel(row).badge" class="analytics-badge">{{ referrerLabel(row).badge }}</span>
                  </span>
                </div>
              </td>
              <td class="analytics-count">{{ row.count }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="analytics-breakdown">
        <h3>Top locations</h3>
        <p v-if="stats.top_locations.length === 0" class="status">No data yet.</p>
        <table v-else class="analytics-table">
          <tbody>
            <!-- name is GoatCounter's resolved country name, not raw
                 visitor input, but rendered as plain text regardless. -->
            <tr v-for="row in stats.top_locations" :key="row.name">
              <td>
                <div class="analytics-bar-row">
                  <span class="analytics-bar" :style="{ width: barWidth(row.count, stats.top_locations) + '%' }" />
                  <span class="analytics-bar-label">{{ flagEmoji(row.code) }} {{ row.name || 'Unknown' }}</span>
                </div>
              </td>
              <td class="analytics-count">{{ row.count }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- goatcounter_url is a deploy-time config value the admin set
           themselves (internal/config), not visitor-controlled data —
           safe to use directly as an href. It never carries a secret:
           access is gated by a Caddy forward_auth check against this
           same nitpub session, not a bearer token in the URL. -->
      <div v-if="stats.goatcounter_url" class="analytics-goatcounter-card">
        <div>
          <h3>Full dashboard</h3>
          <p>GoatCounter's own charts — visitor trends, browsers, devices — gated by this same admin login.</p>
        </div>
        <a :href="stats.goatcounter_url" target="_blank" rel="noopener noreferrer" class="analytics-goatcounter-link">
          Open GoatCounter
        </a>
      </div>
    </template>
  </div>
</template>

<style scoped>
.analytics-window-switch {
  display: flex;
  gap: var(--space-2);
}
.analytics-window-switch button {
  padding: 0.35rem 0.75rem;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: transparent;
  color: var(--muted);
  font-size: var(--text-sm);
  cursor: pointer;
}
.analytics-window-switch button:hover:not(:disabled) {
  color: var(--text);
}
.analytics-window-switch button.active {
  color: var(--text);
  background: color-mix(in srgb, var(--accent) 12%, transparent);
  border-color: var(--accent);
  font-weight: 600;
}
.analytics-window-switch button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
.analytics-total {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-4);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
}
.analytics-total-figures {
  display: flex;
  align-items: baseline;
  gap: var(--space-2);
}
.analytics-total-value {
  font-size: var(--text-2xl);
  font-weight: 600;
  font-family: var(--font-serif);
  font-variant-numeric: tabular-nums;
}
.analytics-total-label {
  color: var(--muted);
  font-size: var(--text-sm);
}
.analytics-trend-delta {
  font-size: var(--text-sm);
  font-weight: 600;
  color: var(--accent);
  font-variant-numeric: tabular-nums;
}
.analytics-trend-delta.down {
  color: var(--muted);
}
.analytics-sparkline {
  flex-shrink: 0;
}
.analytics-sparkline-grid {
  stroke: var(--border);
  stroke-width: 1;
  stroke-dasharray: 2 3;
}
.analytics-sparkline-fill {
  fill: color-mix(in srgb, var(--accent) 22%, transparent);
}
.analytics-sparkline-line {
  fill: none;
  stroke: var(--accent);
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.analytics-sparkline-dot {
  fill: var(--accent);
}
.analytics-breakdown h3 {
  margin: 0 0 var(--space-2);
  font-size: var(--text-sm);
  font-weight: 600;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.03em;
}
.analytics-table {
  width: 100%;
  border-collapse: collapse;
  font-size: var(--text-sm);
}
.analytics-table td {
  padding: var(--space-2) 0;
  border-bottom: 1px solid var(--border);
}
.analytics-count {
  text-align: right;
  color: var(--muted);
  white-space: nowrap;
}
.analytics-bar-row {
  position: relative;
  display: flex;
  align-items: center;
  min-height: 1.5em;
}
.analytics-bar {
  position: absolute;
  inset: 0 auto 0 0;
  background: color-mix(in srgb, var(--accent) 16%, transparent);
  border-radius: var(--radius-sm, 4px);
  transition: width 0.2s ease;
}
.analytics-bar-label {
  position: relative;
  padding-left: var(--space-2);
}
.analytics-badge {
  margin-left: var(--space-2);
  padding: 0.05rem 0.4rem;
  border-radius: 999px;
  background: var(--surface);
  border: 1px solid var(--border);
  color: var(--muted);
  font-size: 0.7em;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}
.analytics-goatcounter-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-4);
  background: var(--surface);
  border: 1px dashed var(--border);
  border-radius: var(--radius-md);
}
.analytics-goatcounter-card h3 {
  margin: 0 0 var(--space-1, 0.25rem);
  font-size: var(--text-sm);
  font-weight: 600;
}
.analytics-goatcounter-card p {
  margin: 0;
  color: var(--muted);
  font-size: var(--text-sm);
  max-width: 34rem;
}
.analytics-goatcounter-card code {
  font-size: 0.85em;
}
.analytics-goatcounter-link {
  flex-shrink: 0;
  padding: 0.4rem 0.9rem;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm, 4px);
  background: transparent;
  color: var(--accent);
  text-decoration: none;
  font-size: var(--text-sm);
  font-weight: 600;
  white-space: nowrap;
}
.analytics-goatcounter-link:hover {
  border-color: var(--accent);
}
</style>
