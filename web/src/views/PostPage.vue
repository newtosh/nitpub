<script setup lang="ts">
import { ArrowLeft, ChevronRight, Link2, Pencil } from '@lucide/vue'
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import CommentBox from '../components/CommentBox.vue'
import { useSession } from '../composables/useSession'
import MarkdownBody from '../components/MarkdownBody.vue'
import ReplyThread from '../components/ReplyThread.vue'
import {
  avatarsEnabledFromConfig,
  federationShared,
  repliesCollapsedByDefaultFromConfig,
} from '../lib/federationDelivery'
import {
  articleBody,
  articleTitle,
  fetchPost,
  formatDate,
  noteBody,
  noteTitle,
  type Post,
} from '../lib/posts'
import { buildReplyTree, fetchReplies, type PublicReply } from '../lib/replies'
import { fetchSiteConfig } from '../lib/site'

const props = defineProps<{ slug: string }>()
const route = useRoute()
const { authed } = useSession()

const post = ref<Post | null>(null)
const error = ref('')
const loading = ref(true)

const replies = ref<PublicReply[]>([])
const repliesError = ref('')
const repliesLoading = ref(true)
const showAvatars = ref(true)
const repliesExpanded = ref(false)
// Only auto-expand once, from the site's admin-configured default (or a
// direct #replies link) — a later explicit collapsed-fetch (0 replies)
// must not silently re-collapse a state the visitor already toggled.
let repliesExpandedInitialized = false

const replyTree = computed(() => buildReplyTree(replies.value))

async function load(slug: string) {
  loading.value = true
  error.value = ''
  post.value = null
  try {
    post.value = await fetchPost(slug)
  } catch (e) {
    error.value = e instanceof Error && e.message === 'not-found'
      ? 'Post not found.'
      : 'Could not load post.'
  } finally {
    loading.value = false
  }
}

async function loadReplies(slug: string) {
  repliesLoading.value = true
  repliesError.value = ''
  replies.value = []
  try {
    replies.value = await fetchReplies(slug)
  } catch {
    repliesError.value = "Couldn't load replies."
  } finally {
    repliesLoading.value = false
  }
}

async function loadAll(slug: string) {
  await Promise.all([load(slug), loadReplies(slug)])
  // The #replies anchor only exists once the post has rendered, so jump to
  // it here rather than relying on the router's (pre-render) scrollBehavior.
  if (route.hash === '#replies' && post.value) {
    await nextTick()
    document.getElementById('replies')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }
}

function toggleReplies() {
  repliesExpanded.value = !repliesExpanded.value
}

onMounted(async () => {
  loadAll(props.slug)
  if (route.hash === '#replies') {
    repliesExpanded.value = true
    repliesExpandedInitialized = true
  }
  try {
    const config = await fetchSiteConfig()
    showAvatars.value = avatarsEnabledFromConfig(config.federation)
    if (!repliesExpandedInitialized) {
      repliesExpanded.value = !repliesCollapsedByDefaultFromConfig(config.federation)
      repliesExpandedInitialized = true
    }
  } catch {
    showAvatars.value = true
    if (!repliesExpandedInitialized) {
      repliesExpanded.value = false
      repliesExpandedInitialized = true
    }
  }
})
watch(() => props.slug, loadAll)
</script>

<template>
  <p class="back">
    <RouterLink to="/" class="btn btn-ghost btn-back">
      <ArrowLeft :size="16" :stroke-width="1.75" aria-hidden="true" />
      <span>All posts</span>
    </RouterLink>
    <RouterLink v-if="authed" :to="`/author/edit/${props.slug}`" class="text-link edit-link">
      <Pencil :size="16" :stroke-width="1.75" aria-hidden="true" />
      <span>Edit</span>
    </RouterLink>
  </p>

  <p v-if="loading" class="status">Loading…</p>
  <p v-else-if="error" class="status error">{{ error }}</p>

  <article v-else-if="post" class="post-full">
    <p class="meta">
      <span class="kind">{{ post.kind }}</span>
      <time :datetime="post.created_at">{{ formatDate(post.created_at) }}</time>
      <span v-if="post.updated_at" class="edited">· edited {{ formatDate(post.updated_at) }}</span>
      <a
        v-if="post.federation?.remote_url"
        :href="post.federation.remote_url"
        target="_blank"
        rel="noopener noreferrer nofollow"
        class="permalink-btn"
        title="View on Mastodon"
        aria-label="View on Mastodon"
      >
        <Link2 :size="14" :stroke-width="1.75" aria-hidden="true" />
        <span>Mastodon</span>
      </a>
    </p>

    <template v-if="post.kind === 'article'">
      <h1>{{ articleTitle(post.content) }}</h1>
      <MarkdownBody :content="articleBody(post.content)" />
    </template>
    <template v-else>
      <h1 v-if="noteTitle(post.content)">{{ noteTitle(post.content) }}</h1>
      <MarkdownBody class="note-body" :content="noteBody(post.content)" :inline-link-cards="true" />
    </template>

    <section id="replies" class="thread">
      <button type="button" class="thread-heading" :aria-expanded="repliesExpanded" @click="toggleReplies">
        <ChevronRight :size="18" :stroke-width="1.75" class="thread-chevron" :class="{ expanded: repliesExpanded }" aria-hidden="true" />
        Replies
        <span v-if="!repliesLoading && replies.length > 0" class="thread-count">({{ replies.length }})</span>
      </button>

      <div v-show="repliesExpanded" class="thread-body">
        <p v-if="repliesLoading" class="status">Loading replies…</p>
        <p v-else-if="repliesError" class="status error">{{ repliesError }}</p>
        <ul v-else-if="replies.length > 0" class="reply-list">
          <ReplyThread
            v-for="node in replyTree"
            :key="node.reply.object_id || node.reply.actor + node.reply.received_at"
            :node="node"
            :depth="0"
            :show-avatars="showAvatars"
          />
        </ul>

        <CommentBox v-if="post && federationShared(post)" :post-slug="props.slug" />
      </div>
    </section>
  </article>
</template>

<style scoped>
.back {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 0 0 1.5rem;
}
.edit-link {
  color: var(--muted);
  font-size: 0.9rem;
  text-decoration: none;
}
.edit-link:hover {
  color: var(--accent);
}
.btn-back {
  padding: 0.35rem 0.85rem 0.35rem 0.65rem;
  font-size: 0.85rem;
}
.text-link {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
}
.status {
  color: var(--muted);
}
.status.error {
  color: var(--danger);
}
.meta {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin: 0 0 0.75rem;
  font-size: 0.85rem;
  color: var(--muted);
}
.kind {
  text-transform: capitalize;
  font-weight: 600;
  color: var(--accent);
}
.edited {
  color: var(--muted);
}
.permalink-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  margin-left: auto;
  color: var(--muted);
  text-decoration: none;
}
.permalink-btn:hover {
  color: var(--accent);
}
h1 {
  font-family: var(--font-serif);
  font-size: 2rem;
  line-height: 1.25;
  margin: 0 0 1.25rem;
}
.note-body :deep(.markdown-body) {
  font-size: 1.15rem;
}
.thread {
  margin-top: 2.5rem;
  padding-top: 1.5rem;
  border-top: 1px solid var(--border);
  scroll-margin-top: calc(var(--site-chrome-height, 3.5rem) + 0.75rem);
}
.thread-heading {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  width: 100%;
  margin: 0;
  padding: 0;
  border: none;
  background: none;
  font-family: var(--font-serif);
  font-size: 1.1rem;
  color: var(--text);
  text-align: left;
  cursor: pointer;
}
.thread-heading:hover {
  color: var(--accent);
}
.thread-chevron {
  color: var(--muted);
  transition: transform 0.15s ease;
}
.thread-chevron.expanded {
  transform: rotate(90deg);
}
.thread-count {
  font-weight: 400;
  color: var(--muted);
}
.thread-body {
  margin-top: 1rem;
}
.reply-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
</style>
