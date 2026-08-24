<script setup lang="ts">
import { ChevronUp, X } from '@lucide/vue'
import { onBeforeUnmount, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import PostDeliveryBadges from './PostDeliveryBadges.vue'
import {
  fetchPostsPage,
  formatDate,
  postDisplayTitle,
  postSlug,
  type Post,
} from '../lib/posts'

const RECENT_LIMIT = 5

const open = ref(false)
const loading = ref(false)
const error = ref('')
const posts = ref<Post[]>([])
const loaded = ref(false)
const dragOffset = ref(0)

const DISMISS_THRESHOLD = 80
let dragStartY: number | null = null

function onDragStart(ev: PointerEvent) {
  dragStartY = ev.clientY
  ;(ev.target as HTMLElement).setPointerCapture?.(ev.pointerId)
}

function onDragMove(ev: PointerEvent) {
  if (dragStartY === null) return
  if (ev.buttons === 0) {
    onDragEnd(ev)
    return
  }
  const delta = ev.clientY - dragStartY
  dragOffset.value = Math.max(0, delta)
}

function onDragEnd(ev?: PointerEvent) {
  if (dragStartY === null) return
  if (dragOffset.value > DISMISS_THRESHOLD) close()
  dragStartY = null
  dragOffset.value = 0
  if (ev) (ev.target as HTMLElement).releasePointerCapture?.(ev.pointerId)
}

function toggle() {
  open.value = !open.value
}

function close() {
  open.value = false
}

function onBackdropClick(ev: MouseEvent) {
  if (ev.target === ev.currentTarget) close()
}

async function loadRecent() {
  loading.value = true
  error.value = ''
  try {
    const data = await fetchPostsPage(RECENT_LIMIT, 0)
    posts.value = data.posts
    loaded.value = true
  } catch {
    error.value = 'Failed to load posts'
  } finally {
    loading.value = false
  }
}

watch(open, (isOpen) => {
  document.body.style.overflow = isOpen ? 'hidden' : ''
  if (isOpen && !loaded.value) loadRecent()
})

onBeforeUnmount(() => {
  document.body.style.overflow = ''
})
</script>

<template>
  <div class="recent-sheet-root">
    <button
      type="button"
      class="recent-sheet-handle"
      :aria-expanded="open"
      aria-controls="recent-sheet-content"
      @click="toggle"
    >
      <ChevronUp :size="16" :stroke-width="1.75" aria-hidden="true" />
      <span>Recent posts</span>
    </button>

    <Teleport to="body">
      <div v-if="open" class="recent-sheet-backdrop" @click="onBackdropClick">
        <div
          id="recent-sheet-content"
          class="recent-sheet card"
          role="dialog"
          aria-modal="true"
          aria-label="Recent posts"
          :style="{ transform: dragOffset ? `translateY(${dragOffset}px)` : undefined }"
        >
          <div
            class="recent-sheet-header"
            @pointerdown="onDragStart"
            @pointermove="onDragMove"
            @pointerup="onDragEnd"
            @pointercancel="onDragEnd"
          >
            <span>Recent posts</span>
            <button type="button" class="recent-sheet-close" aria-label="Close" @click="close">
              <X :size="18" :stroke-width="1.75" aria-hidden="true" />
            </button>
          </div>

          <p v-if="loading" class="recent-sheet-status text-muted">Loading…</p>
          <p v-else-if="error" class="recent-sheet-status alert alert-error">{{ error }}</p>
          <p v-else-if="posts.length === 0" class="recent-sheet-status text-muted">No posts yet.</p>

          <ul v-else class="recent-sheet-list">
            <li v-for="post in posts" :key="post.id">
              <RouterLink :to="`/author/edit/${postSlug(post.id)}`" class="recent-sheet-row" @click="close">
                <div class="recent-sheet-row-main">
                  <span class="recent-sheet-kind">{{ post.kind }}</span>
                  <span v-if="post.status === 'draft'" class="recent-sheet-draft-badge">Draft</span>
                  {{ postDisplayTitle(post) }}
                </div>
                <div class="recent-sheet-row-meta">
                  <PostDeliveryBadges :post="post" />
                  <time>{{ formatDate(post.created_at) }}</time>
                </div>
              </RouterLink>
            </li>
          </ul>

          <RouterLink to="/author" class="recent-sheet-more" @click="close">See more</RouterLink>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.recent-sheet-handle {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.35rem;
  width: 100%;
  padding: 0.5rem;
  border: 1px solid var(--border);
  border-bottom: none;
  border-radius: var(--radius-md) var(--radius-md) 0 0;
  background: var(--surface);
  color: var(--muted);
  font-size: var(--text-sm);
  cursor: pointer;
}
.recent-sheet-handle:hover {
  color: var(--accent);
}
@media (min-width: 48rem) {
  /* Sticky must go on .recent-sheet-root, not the handle button itself —
     a sticky element's containing block is its immediate parent, and the
     button's own parent (this root div) is naturally sized to exactly
     the button's height, leaving zero room to travel/pin. .recent-sheet-
     root's own parent (.compose-page) is genuinely tall, so promoting
     sticky one level up gives it real room. .site-scroll (BlogLayout.vue)
     is the app's one real scrolling ancestor — no intermediate overflow
     container between here and there — so unlike mobile, sticky
     positions cleanly without drift. Pins the handle to the viewport
     bottom while scrolling the compose page, then gives way once the
     real site footer scrolls into view. */
  .recent-sheet-root {
    position: sticky;
    bottom: 0;
    z-index: 5;
    /* .site-main's own bottom padding (--space-12, BlogLayout.vue) used
       to be invisible whitespace after ordinary page content. Now that
       this bar is the last thing in the page, that padding reads as a
       gap floating it above the footer instead. Cancel it so the bar's
       bottom edge lands flush on the footer's top border, both while
       stuck mid-scroll and once released at the true page end. */
    margin-bottom: calc(-1 * var(--space-12));
  }
}
@media (max-width: 47.99rem) {
  /* Anchor to the actual device viewport edge, not wherever normal
     document flow happens to land it — position:sticky needs a
     scrolling ancestor to key off and compose-page doesn't reliably
     provide one, which is why the last attempt drifted. Fixed to the
     visual viewport bottom is unambiguous. */
  .recent-sheet-handle {
    position: fixed;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 10;
    border-radius: 0;
    padding-bottom: calc(0.5rem + env(safe-area-inset-bottom));
  }
}
.recent-sheet-backdrop {
  position: fixed;
  inset: 0;
  z-index: 100;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  background: rgba(0, 0, 0, 0.35);
}
.recent-sheet {
  width: 100%;
  max-width: 32rem;
  max-height: 70vh;
  display: flex;
  flex-direction: column;
  border-radius: var(--radius-md) var(--radius-md) 0 0;
  padding: var(--space-4);
  overflow: hidden;
  touch-action: pan-y;
}
.recent-sheet-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-weight: 600;
  margin-bottom: var(--space-3);
}
.recent-sheet-close {
  border: none;
  background: none;
  color: var(--muted);
  cursor: pointer;
}
.recent-sheet-status {
  margin: 0;
}
.recent-sheet-list {
  list-style: none;
  padding: 0;
  margin: 0;
  overflow-y: auto;
}
.recent-sheet-row {
  display: block;
  padding: var(--space-2) 0;
  border-bottom: 1px solid var(--border);
  color: var(--text);
  text-decoration: none;
}
.recent-sheet-row:hover {
  color: var(--accent);
}
.recent-sheet-row-main {
  font-size: var(--text-sm);
}
.recent-sheet-kind {
  text-transform: capitalize;
  color: var(--accent);
  font-weight: 600;
  margin-right: var(--space-2);
}
.recent-sheet-draft-badge {
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
.recent-sheet-row-meta {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin-top: 0.15rem;
  font-size: var(--text-xs);
  color: var(--muted);
}
.recent-sheet-more {
  display: block;
  text-align: center;
  margin-top: var(--space-3);
  padding-top: var(--space-3);
  border-top: 1px solid var(--border);
  color: var(--accent);
  text-decoration: none;
  font-size: var(--text-sm);
}
.recent-sheet-more:hover {
  text-decoration: underline;
}
</style>
