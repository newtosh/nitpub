<script setup lang="ts">
import { CircleUserRound, Rss, Search } from '@lucide/vue'
import { computed } from 'vue'
import InfoTip from './InfoTip.vue'
import { navIcon } from '../lib/icons'

export type NavPreviewItem = {
  label: string
  path: string
  icon?: string
}

export type PagePreviewRef = {
  path: string
}

const props = defineProps<{
  nav: NavPreviewItem[]
  pages?: PagePreviewRef[]
  searchEnabled?: boolean
}>()

const BUILTIN_NAV_PATHS = new Set(['/', '/posts', '/search', '/author', '/login'])

function normalizePath(p: string): string {
  const s = (p || '').trim()
  if (!s) return ''
  const withSlash = s.startsWith('/') ? s : `/${s}`
  if (withSlash === '/') return '/'
  return withSlash.replace(/\/+$/, '')
}

const pagePaths = computed(() => new Set((props.pages ?? []).map((p) => normalizePath(p.path))))

function needsRegisteredPage(path: string): boolean {
  const n = normalizePath(path)
  if (!n || BUILTIN_NAV_PATHS.has(n) || n.startsWith('/p/')) return false
  return true
}

function navItemWarning(path: string): string | null {
  const n = normalizePath(path)
  if (!n) return 'Missing path'
  if (needsRegisteredPage(n) && !pagePaths.value.has(n)) {
    return `No page registered for ${n}`
  }
  return null
}

const unlinkedPages = computed(() => {
  const navPaths = new Set(props.nav.map((i) => normalizePath(i.path)).filter(Boolean))
  return (props.pages ?? [])
    .map((p) => normalizePath(p.path))
    .filter((p) => p && !navPaths.has(p))
})

const previewHelp =
  'Live preview of the site header. Red items need a matching page. Save to apply changes.'

const unlinkedTip = computed(() => {
  if (!unlinkedPages.value.length) return ''
  return `Not in navigation: ${unlinkedPages.value.join(', ')}. These URLs work but won't appear in the header.`
})
</script>

<template>
  <div class="nav-preview" aria-label="Navigation preview">
    <div class="preview-label">
      <span>Header preview</span>
      <span class="preview-tips cluster">
        <InfoTip :label="previewHelp" />
        <InfoTip v-if="unlinkedTip" :label="unlinkedTip" warn />
      </span>
    </div>
    <div class="preview-header">
      <span class="brand">nitpub</span>
      <nav class="preview-routes cluster" aria-hidden="true">
        <span
          v-for="(item, i) in nav"
          :key="`${item.path}-${i}`"
          class="preview-item"
          :class="{ warn: navItemWarning(item.path) }"
          :title="navItemWarning(item.path) ?? item.path"
        >
          <component
            :is="navIcon(item.icon)"
            v-if="navIcon(item.icon)"
            :size="18"
            :stroke-width="1.75"
            aria-hidden="true"
          />
          <span class="preview-text">{{ item.label || item.path || 'Untitled' }}</span>
        </span>
        <span v-if="nav.length === 0" class="preview-empty">No nav items</span>
      </nav>
      <nav class="preview-utils cluster" aria-hidden="true">
        <span v-if="searchEnabled !== false" class="preview-icon" title="Search">
          <Search :size="20" :stroke-width="1.75" />
        </span>
        <span class="preview-icon" title="RSS">
          <Rss :size="20" :stroke-width="1.75" />
        </span>
        <span class="preview-icon" title="Sign in">
          <CircleUserRound :size="20" :stroke-width="1.75" />
        </span>
      </nav>
    </div>
  </div>
</template>

<style scoped>
.nav-preview {
  margin: 0 0 var(--space-4);
  padding: var(--space-3);
  border: 1px dashed var(--border);
  border-radius: var(--radius);
  background: color-mix(in srgb, var(--surface) 85%, var(--border));
}
.preview-label {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 0 0 var(--space-3);
  font-size: var(--text-xs);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--muted);
}
.preview-tips {
  gap: 0.35rem;
}
.preview-header {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--surface);
}
.brand {
  justify-self: start;
  font-family: var(--font-serif);
  font-size: var(--text-lg);
  font-weight: 600;
  color: var(--text);
}
.preview-routes {
  justify-self: center;
  gap: var(--space-3);
  flex-wrap: wrap;
  justify-content: center;
}
.preview-item {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  color: var(--muted);
  font-size: var(--text-sm);
}
.preview-item.warn {
  color: var(--danger);
  outline: 1px dashed color-mix(in srgb, var(--danger) 55%, transparent);
  outline-offset: 2px;
  border-radius: var(--radius);
}
.preview-text {
  white-space: nowrap;
}
.preview-empty {
  font-size: var(--text-sm);
  color: var(--muted);
  font-style: italic;
}
.preview-utils {
  justify-self: end;
  gap: var(--space-2);
  color: var(--muted);
}
.preview-icon {
  display: inline-flex;
  line-height: 0;
}
@media (max-width: 40rem) {
  .preview-header {
    grid-template-columns: 1fr auto;
  }
  .preview-routes {
    grid-column: 1 / -1;
    justify-self: center;
  }
  .preview-utils {
    justify-self: end;
  }
}
</style>
