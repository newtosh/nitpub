<script setup lang="ts">
import { Search } from '@lucide/vue'
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import SearchField from './SearchField.vue'
import { searchContent, type SearchResult } from '../lib/search'

const props = defineProps<{ enabled?: boolean }>()

const router = useRouter()
const root = ref<HTMLElement | null>(null)
const searchFieldRef = ref<InstanceType<typeof SearchField> | null>(null)
const query = ref('')
const results = ref<SearchResult[]>([])
const panelOpen = ref(false)
const loading = ref(false)
let debounce: ReturnType<typeof setTimeout> | undefined

function close() {
  panelOpen.value = false
  query.value = ''
  results.value = []
}

async function openPanel() {
  panelOpen.value = true
  await nextTick()
  searchFieldRef.value?.focus()
}

function togglePanel() {
  if (panelOpen.value) close()
  else openPanel()
}

async function runSearch(q: string) {
  const trimmed = q.trim()
  if (!trimmed) {
    results.value = []
    return
  }
  loading.value = true
  try {
    results.value = await searchContent(trimmed)
  } catch {
    results.value = []
  } finally {
    loading.value = false
  }
}

watch(query, (q) => {
  clearTimeout(debounce)
  if (!q.trim()) {
    results.value = []
    return
  }
  debounce = setTimeout(() => runSearch(q), 250)
})

function goToResults() {
  const q = query.value.trim()
  if (!q) return
  close()
  router.push({ path: '/search', query: { q } })
}

function pick(hit: SearchResult) {
  close()
  router.push(hit.url)
}

function onDocumentClick(ev: MouseEvent) {
  if (!panelOpen.value || !root.value) return
  if (!root.value.contains(ev.target as Node)) close()
}

function onDocumentKeydown(ev: KeyboardEvent) {
  if (ev.key === 'Escape' && panelOpen.value) {
    ev.preventDefault()
    close()
  }
}

watch(panelOpen, (open) => {
  if (open) {
    document.addEventListener('click', onDocumentClick)
    document.addEventListener('keydown', onDocumentKeydown)
  } else {
    document.removeEventListener('click', onDocumentClick)
    document.removeEventListener('keydown', onDocumentKeydown)
  }
})

onBeforeUnmount(() => {
  document.removeEventListener('click', onDocumentClick)
  document.removeEventListener('keydown', onDocumentKeydown)
})
</script>

<template>
  <div v-if="enabled !== false" ref="root" class="search-box">
    <button
      type="button"
      class="nav-icon search-trigger"
      :class="{ active: panelOpen }"
      aria-label="Search posts and pages"
      title="Search"
      :aria-expanded="panelOpen"
      @click.stop="togglePanel"
    >
      <Search :size="20" :stroke-width="1.75" aria-hidden="true" />
    </button>

    <div v-if="panelOpen" class="search-panel" role="dialog" aria-label="Search" @click.stop>
      <SearchField
        ref="searchFieldRef"
        v-model="query"
        class="panel-field"
        :bordered="false"
        placeholder="Search posts and pages…"
        aria-label="Search query"
        @keydown.enter.prevent="goToResults"
      >
        <template #suffix>
          <span v-if="loading" class="loading" aria-hidden="true">…</span>
        </template>
      </SearchField>

      <ul v-if="results.length" class="dropdown">
        <li v-for="(hit, i) in results" :key="i">
          <button type="button" @click="pick(hit)">
            <span class="kind">{{ hit.type }}</span>
            {{ hit.title || hit.url }}
          </button>
        </li>
        <li class="more">
          <button type="button" @click="goToResults">View all results</button>
        </li>
      </ul>
      <p v-else-if="query.trim() && !loading" class="empty">No results yet.</p>
    </div>
  </div>
</template>

<style scoped>
.search-box {
  position: relative;
  display: inline-flex;
  align-items: center;
}

.nav-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 0;
  padding: 0;
  border: none;
  background: none;
  color: var(--muted);
  cursor: pointer;
  font: inherit;
}

.nav-icon:hover,
.search-trigger.active {
  color: var(--accent);
}

.search-panel {
  position: absolute;
  top: calc(100% + 0.5rem);
  right: 0;
  z-index: 300;
  width: min(20rem, calc(100vw - 2rem));
  padding: var(--space-2);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: 0 12px 32px rgb(0 0 0 / 0.14);
}

.panel-field {
  padding: 0.15rem 0;
}

.loading {
  flex-shrink: 0;
  color: var(--muted);
}

.dropdown {
  list-style: none;
  margin: var(--space-2) 0 0;
  padding: 0.25rem 0 0;
  border-top: 1px solid var(--border);
  max-height: 14rem;
  overflow: auto;
}

.dropdown button {
  display: block;
  width: 100%;
  text-align: left;
  padding: 0.45rem 0.35rem;
  border: none;
  background: none;
  color: inherit;
  font: inherit;
  cursor: pointer;
  border-radius: calc(var(--radius) - 2px);
}

.dropdown button:hover {
  background: var(--surface-2, var(--border));
}

.kind {
  font-size: var(--text-xs);
  text-transform: uppercase;
  color: var(--muted);
  margin-right: 0.35rem;
}

.more button {
  color: var(--accent);
  font-size: var(--text-sm);
}

.empty {
  margin: var(--space-2) 0 0;
  padding-top: var(--space-2);
  border-top: 1px solid var(--border);
  color: var(--muted);
  font-size: var(--text-sm);
}
</style>
