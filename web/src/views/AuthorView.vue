<script setup lang="ts">
import { ArrowLeft, ArrowRight, Inbox, Pencil, Plus, Trash2 } from '@lucide/vue'
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import ConfirmModal from '../components/ConfirmModal.vue'
import PostDeliveryBadges from '../components/PostDeliveryBadges.vue'
import SearchField from '../components/SearchField.vue'
import {
  deletePost,
  fetchPostsPage,
  formatDate,
  postDisplayTitle,
  postSlug,
  type Post,
} from '../lib/posts'
import { fetchPendingReplies } from '../lib/moderationAdmin'
import { searchContent, type SearchResult } from '../lib/search'
import { useSession } from '../composables/useSession'

const PAGE_SIZE = 10

const { refresh } = useSession()

const pendingByPost = ref<Map<string, number>>(new Map())

async function loadPendingCounts() {
  try {
    const pending = await fetchPendingReplies()
    const counts = new Map<string, number>()
    for (const reply of pending) {
      counts.set(reply.post_slug, (counts.get(reply.post_slug) ?? 0) + 1)
    }
    pendingByPost.value = counts
  } catch {
    pendingByPost.value = new Map()
  }
}

const posts = ref<Post[]>([])
const searchResults = ref<SearchResult[]>([])
const query = ref('')
const total = ref(0)
const offset = ref(0)
const loading = ref(false)
const searchLoading = ref(false)
const error = ref('')
const deleteOpen = ref(false)
const deleteBusy = ref(false)
const deleteTarget = ref<{ slug: string; title: string } | null>(null)

const deleteMessage = computed(() => {
  if (!deleteTarget.value) return ''
  return `“${deleteTarget.value.title}” will be removed from your site. This cannot be undone.`
})

let searchDebounce: ReturnType<typeof setTimeout> | undefined

const isSearching = computed(() => query.value.trim().length > 0)
const page = computed(() => Math.floor(offset.value / PAGE_SIZE) + 1)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)))

function slugFromPostUrl(url: string): string {
  const match = url.match(/^\/p\/(.+)$/)
  return match?.[1] ?? ''
}

async function loadPage(newOffset = offset.value) {
  loading.value = true
  error.value = ''
  try {
    const data = await fetchPostsPage(PAGE_SIZE, newOffset)
    posts.value = data.posts
    total.value = data.total
    offset.value = newOffset
  } catch {
    error.value = 'Failed to load posts'
  } finally {
    loading.value = false
  }
}

async function runSearch(q: string) {
  const trimmed = q.trim()
  if (!trimmed) {
    searchResults.value = []
    if (posts.value.length === 0) await loadPage(0)
    return
  }
  searchLoading.value = true
  error.value = ''
  try {
    const hits = await searchContent(trimmed)
    searchResults.value = hits.filter((hit) => hit.type === 'post')
  } catch {
    error.value = 'Search failed'
    searchResults.value = []
  } finally {
    searchLoading.value = false
  }
}

watch(query, (q) => {
  clearTimeout(searchDebounce)
  searchDebounce = setTimeout(() => runSearch(q), 250)
})

async function goToPage(n: number) {
  await loadPage((n - 1) * PAGE_SIZE)
  document.querySelector('.recent')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function openDelete(slug: string, title: string) {
  deleteTarget.value = { slug, title }
  deleteOpen.value = true
}

function openDeletePost(post: Post) {
  openDelete(postSlug(post.id), postDisplayTitle(post))
}

function openDeleteSearchHit(hit: SearchResult) {
  const slug = slugFromPostUrl(hit.url)
  if (!slug) return
  openDelete(slug, hit.title || hit.url)
}

async function confirmDelete() {
  if (!deleteTarget.value) return
  const slug = deleteTarget.value.slug
  deleteBusy.value = true
  error.value = ''
  try {
    await deletePost(slug)
    deleteOpen.value = false
    deleteTarget.value = null
    if (isSearching.value) {
      searchResults.value = searchResults.value.filter((hit) => slugFromPostUrl(hit.url) !== slug)
      await runSearch(query.value)
    } else {
      await loadPage(offset.value)
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Delete failed'
  } finally {
    deleteBusy.value = false
  }
}

onMounted(async () => {
  await refresh()
  await Promise.all([loadPage(0), loadPendingCounts()])
})
</script>

<template>
  <div class="author-top">
    <header class="page-header">
      <div class="page-header-text">
        <h1>Author</h1>
        <p class="text-muted">Manage notes and articles for this instance.</p>
      </div>
    </header>

    <div class="recent-header">
      <h2>Recent posts</h2>
      <RouterLink to="/author/compose" class="btn btn-primary">
        <Plus :size="16" :stroke-width="1.75" aria-hidden="true" />
        <span>New post</span>
      </RouterLink>
    </div>

    <div class="search-row">
      <SearchField
        v-model="query"
        class="recent-search"
        placeholder="Search posts…"
        aria-label="Search recent posts"
      />
    </div>
  </div>

  <section class="recent section">
    <p v-if="isSearching" class="text-muted search-scope-note">
      Search covers published posts — drafts aren't indexed yet.
    </p>

    <p v-if="loading || searchLoading" class="text-muted">Loading…</p>
    <p v-else-if="isSearching && searchResults.length === 0" class="text-muted">No matching posts.</p>
    <p v-else-if="!isSearching && posts.length === 0" class="text-muted">No posts yet.</p>

    <ul v-else-if="isSearching">
      <li v-for="(hit, i) in searchResults" :key="hit.url + i">
        <div class="row">
          <RouterLink :to="hit.url" class="row-title">
            <span class="title-text">{{ hit.title || hit.url }}</span>
          </RouterLink>
          <div v-if="slugFromPostUrl(hit.url)" class="post-actions cluster">
            <RouterLink
              class="btn btn-ghost btn-icon"
              :to="`/author/edit/${slugFromPostUrl(hit.url)}`"
              title="Edit post"
              aria-label="Edit post"
            >
              <Pencil :size="16" :stroke-width="1.75" aria-hidden="true" />
            </RouterLink>
            <button
              type="button"
              class="btn btn-ghost btn-icon btn-danger"
              title="Delete post"
              aria-label="Delete post"
              @click="openDeleteSearchHit(hit)"
            >
              <Trash2 :size="16" :stroke-width="1.75" aria-hidden="true" />
            </button>
          </div>
        </div>
        <p v-if="hit.snippet" class="snippet">{{ hit.snippet }}</p>
      </li>
    </ul>

    <ul v-else>
      <li v-for="post in posts" :key="post.id">
        <div class="row">
          <RouterLink
            class="row-title"
            :to="post.status === 'draft' ? `/author/edit/${postSlug(post.id)}` : `/p/${postSlug(post.id)}`"
          >
            <span class="kind">{{ post.kind }}</span>
            <span v-if="post.status === 'draft'" class="draft-badge">Draft</span>
            <span class="title-text">{{ postDisplayTitle(post) }}</span>
          </RouterLink>
          <div class="post-actions cluster">
            <RouterLink
              class="btn btn-ghost btn-icon"
              :to="`/author/edit/${postSlug(post.id)}`"
              :title="`Edit ${post.kind}`"
              :aria-label="`Edit ${post.kind}`"
            >
              <Pencil :size="16" :stroke-width="1.75" aria-hidden="true" />
            </RouterLink>
            <button
              type="button"
              class="btn btn-ghost btn-icon btn-danger"
              :title="`Delete ${post.kind}`"
              :aria-label="`Delete ${post.kind}`"
              @click="openDeletePost(post)"
            >
              <Trash2 :size="16" :stroke-width="1.75" aria-hidden="true" />
            </button>
          </div>
        </div>
        <div class="meta-row">
          <PostDeliveryBadges :post="post" />
          <span class="meta-row-right">
            <RouterLink
              v-if="pendingByPost.get(postSlug(post.id))"
              class="pending-badge"
              to="/admin/moderation"
              :title="`${pendingByPost.get(postSlug(post.id))} reply/replies awaiting moderation`"
            >
              <Inbox :size="13" :stroke-width="2" aria-hidden="true" />
              {{ pendingByPost.get(postSlug(post.id)) }} pending
            </RouterLink>
            <time>{{ formatDate(post.created_at) }}</time>
          </span>
        </div>
      </li>
    </ul>

    <nav
      v-if="!isSearching && totalPages > 1 && !loading"
      class="pager"
      aria-label="Recent posts pagination"
    >
      <button type="button" class="pager-link" :disabled="page <= 1" @click="goToPage(page - 1)">
        <ArrowLeft :size="16" :stroke-width="1.75" aria-hidden="true" />
        <span>Newer</span>
      </button>
      <span class="pager-meta">Page {{ page }} of {{ totalPages }}</span>
      <button
        type="button"
        class="pager-link"
        :disabled="page >= totalPages"
        @click="goToPage(page + 1)"
      >
        <span>Older</span>
        <ArrowRight :size="16" :stroke-width="1.75" aria-hidden="true" />
      </button>
    </nav>
  </section>

  <ConfirmModal
    v-model:open="deleteOpen"
    title="Delete post?"
    :message="deleteMessage"
    confirm-label="Delete"
    :danger="true"
    :busy="deleteBusy"
    @confirm="confirmDelete"
  />

  <p v-if="error" class="alert alert-error">{{ error }}</p>
</template>

<style scoped>
.page-header h1 {
  margin: 0;
}
@media (max-width: 47.99rem) {
  /* page-header now only wraps the title/description text (New post
     moved next to the Recent posts heading) — hide the whole element,
     not just its child, so its margin-bottom doesn't leave a stray gap. */
  .page-header {
    display: none;
  }
}
.author-top {
  display: flex;
  flex-direction: column;
}
.recent-header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  margin-bottom: var(--space-3);
}
.recent-header h2 {
  font-size: var(--text-lg);
  margin: 0;
}
.search-row {
  display: flex;
  justify-content: flex-end;
  margin-bottom: var(--space-3);
}
.recent-search {
  flex: 1;
  width: 100%;
}
.recent-search :deep(.search-field) {
  flex: none;
  width: 100%;
}
.recent ul {
  list-style: none;
  padding: 0;
  margin: 0;
}
.recent li {
  padding: var(--space-3) 0;
  border-bottom: 1px solid var(--border);
}
.recent .row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-3);
  margin-bottom: var(--space-1);
}
.post-actions {
  flex-shrink: 0;
}
.recent li:last-child {
  border-bottom: none;
}
.recent .row > a {
  color: var(--text);
  text-decoration: none;
}
.recent .row > a:hover {
  color: var(--accent);
}
.row-title {
  display: flex;
  align-items: baseline;
  min-width: 0;
}
/* .kind and .draft-badge keep their own size (no shrink); only the title
   text truncates. An unbroken run of characters with no natural wrap
   point (a pasted URL, a stress-test string) used to force this flex row
   to grow past its container, shoving the edit/delete buttons off to the
   side instead of staying pinned at the row's right edge. */
.title-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}
.recent .kind {
  text-transform: capitalize;
  color: var(--accent);
  font-weight: 600;
  margin-right: var(--space-2);
}
.draft-badge {
  display: inline-block;
  padding: 0.05rem 0.4rem;
  margin-right: var(--space-2);
  border-radius: 999px;
  background: color-mix(in srgb, var(--warn) 20%, transparent);
  color: var(--warn);
  font-size: var(--text-xs);
  font-weight: 600;
  text-transform: uppercase;
}
.search-scope-note {
  margin: 0 0 var(--space-2);
  font-size: var(--text-xs);
}
.recent time,
.recent .snippet {
  font-size: var(--text-xs);
  color: var(--muted);
}
.meta-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
}
.meta-row-right {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}
.pending-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.05rem 0.4rem;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--danger) 15%, transparent);
  color: var(--danger);
  font-size: var(--text-xs);
  font-weight: 500;
  text-decoration: none;
  white-space: nowrap;
}
.pending-badge:hover {
  text-decoration: underline;
}
.recent .snippet {
  margin: 0;
  line-height: 1.45;
}
.pager {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  margin-top: var(--space-4);
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
</style>
