<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import SearchField from '../components/SearchField.vue'
import { searchContent, type SearchResult } from '../lib/search'

const route = useRoute()
const router = useRouter()

const query = ref(typeof route.query.q === 'string' ? route.query.q : '')
const results = ref<SearchResult[]>([])
const loading = ref(false)
const error = ref('')

const hasQuery = computed(() => query.value.trim().length > 0)

async function runSearch(q: string) {
  const trimmed = q.trim()
  if (!trimmed) {
    results.value = []
    return
  }
  loading.value = true
  error.value = ''
  try {
    results.value = await searchContent(trimmed)
  } catch {
    error.value = 'Search failed.'
    results.value = []
  } finally {
    loading.value = false
  }
}

function submit() {
  const q = query.value.trim()
  router.replace({ path: '/search', query: q ? { q } : {} })
  runSearch(q)
}

watch(query, (q) => {
  if (!q.trim()) {
    results.value = []
    if (route.path === '/search') {
      router.replace({ path: '/search', query: {} })
    }
  }
})

onMounted(() => {
  if (hasQuery.value) runSearch(query.value)
})
</script>

<template>
  <header class="page-header">
    <h1>Search</h1>
    <form class="search-form" @submit.prevent="submit">
      <SearchField
        v-model="query"
        placeholder="Search posts and pages…"
        aria-label="Search posts and pages"
        @keydown.enter.prevent="submit"
      />
      <button type="submit" class="btn btn-primary">Search</button>
    </form>
  </header>

  <p v-if="loading" class="status">Searching…</p>
  <p v-else-if="error" class="status error">{{ error }}</p>
  <p v-else-if="hasQuery && results.length === 0" class="status">No results.</p>

  <ul v-else-if="results.length" class="results">
    <li v-for="(hit, i) in results" :key="i">
      <RouterLink :to="hit.url">
        <span class="kind">{{ hit.type }}</span>
        <strong>{{ hit.title || hit.url }}</strong>
        <p v-if="hit.snippet" class="snippet">{{ hit.snippet }}</p>
      </RouterLink>
    </li>
  </ul>
</template>

<style scoped>
.search-form {
  display: flex;
  gap: var(--space-2);
  margin-top: var(--space-3);
  align-items: stretch;
}
.results {
  list-style: none;
  margin: 0;
  padding: 0;
}
.results li {
  border-bottom: 1px solid var(--border);
}
.results a {
  display: block;
  padding: var(--space-4) 0;
  text-decoration: none;
  color: inherit;
}
.kind {
  display: inline-block;
  font-size: var(--text-xs);
  text-transform: uppercase;
  color: var(--muted);
  margin-right: var(--space-2);
}
.snippet {
  margin: var(--space-1) 0 0;
  color: var(--muted);
  font-size: var(--text-sm);
}
.status {
  color: var(--muted);
}
.status.error {
  color: var(--danger);
}
</style>
