<script setup lang="ts">
import { ArrowDown, ArrowLeft, ArrowRight } from '@lucide/vue'
import { computed, onMounted, ref } from 'vue'
import PostCard from '../components/PostCard.vue'
import { fetchPostsPage, type Post } from '../lib/posts'
import { fetchSiteConfig, type SiteConfig } from '../lib/site'

const site = ref<SiteConfig | null>(null)
const posts = ref<Post[]>([])
const total = ref(0)
const offset = ref(0)
const loading = ref(true)
const loadingMore = ref(false)
const error = ref('')

const pageSize = computed(() => site.value?.archive.page_size ?? 20)
const mode = computed(() => site.value?.archive.mode ?? 'pagination')
const hasMore = computed(() => posts.value.length < total.value)
const page = computed(() => Math.floor(offset.value / pageSize.value) + 1)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

async function loadPage(newOffset: number) {
  const pageData = await fetchPostsPage(pageSize.value, newOffset)
  if (newOffset === 0) {
    posts.value = pageData.posts
  } else if (mode.value === 'infinite') {
    posts.value = [...posts.value, ...pageData.posts]
  } else {
    posts.value = pageData.posts
  }
  total.value = pageData.total
  offset.value = newOffset
}

async function init() {
  loading.value = true
  error.value = ''
  try {
    site.value = await fetchSiteConfig()
    await loadPage(0)
  } catch {
    error.value = 'Could not load archive.'
  } finally {
    loading.value = false
  }
}

async function goToPage(n: number) {
  const newOffset = (n - 1) * pageSize.value
  loading.value = true
  try {
    await loadPage(newOffset)
    window.scrollTo({ top: 0, behavior: 'smooth' })
  } catch {
    error.value = 'Could not load posts.'
  } finally {
    loading.value = false
  }
}

async function loadMore() {
  if (!hasMore.value || loadingMore.value) return
  loadingMore.value = true
  try {
    await loadPage(offset.value + pageSize.value)
  } catch {
    error.value = 'Could not load more posts.'
  } finally {
    loadingMore.value = false
  }
}

onMounted(init)
</script>

<template>
  <header class="page-header">
    <h1>All posts</h1>
    <p>Full archive of notes and articles.</p>
  </header>

  <p v-if="loading" class="status">Loading…</p>
  <p v-else-if="error" class="status error">{{ error }}</p>
  <p v-else-if="posts.length === 0" class="status">No posts yet.</p>

  <template v-else>
    <section class="post-list">
      <PostCard v-for="post in posts" :key="post.id" :post="post" />
    </section>

    <nav v-if="mode === 'pagination' && totalPages > 1" class="pager" aria-label="Pagination">
      <button
        type="button"
        class="pager-link"
        :disabled="page <= 1 || loading"
        @click="goToPage(page - 1)"
      >
        <ArrowLeft :size="16" :stroke-width="1.75" aria-hidden="true" />
        <span>Newer</span>
      </button>
      <span class="pager-meta">Page {{ page }} of {{ totalPages }}</span>
      <button
        type="button"
        class="pager-link"
        :disabled="page >= totalPages || loading"
        @click="goToPage(page + 1)"
      >
        <span>Older</span>
        <ArrowRight :size="16" :stroke-width="1.75" aria-hidden="true" />
      </button>
    </nav>

    <p v-else-if="mode === 'infinite' && hasMore" class="load-more">
      <button type="button" class="pager-link" :disabled="loadingMore" @click="loadMore">
        <span>{{ loadingMore ? 'Loading…' : 'Load more' }}</span>
        <ArrowDown v-if="!loadingMore" :size="16" :stroke-width="1.75" aria-hidden="true" />
      </button>
    </p>
  </template>
</template>

<style scoped>
.page-header h1 {
  font-family: var(--font-serif);
  font-size: 2rem;
  margin: 0 0 0.35rem;
}
.page-header p {
  margin: 0 0 1.5rem;
  color: var(--muted);
}
.pager {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  margin-top: var(--space-8);
  padding-top: var(--space-4);
  border-top: 1px solid var(--border);
}
.pager-link {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0;
  border: none;
  background: none;
  color: var(--muted);
  font: inherit;
  font-size: var(--text-sm);
  cursor: pointer;
}
.pager-link:hover:not(:disabled) {
  color: var(--accent);
}
.pager-link:disabled {
  opacity: 0.35;
  cursor: default;
}
.pager-meta {
  color: var(--muted);
  font-size: var(--text-sm);
  text-align: center;
}
.load-more {
  display: flex;
  justify-content: center;
  margin-top: var(--space-8);
  padding-top: var(--space-4);
  border-top: 1px solid var(--border);
}
.status {
  color: var(--muted);
}
.status.error {
  color: var(--danger);
}
</style>
