<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { MessageCircle } from '@lucide/vue'
import MarkdownBody from './MarkdownBody.vue'
import type { Post } from '../lib/posts'
import {
  articlePreviewMarkdown,
  articleTitle,
  formatDate,
  noteIsTruncatedInPreview,
  notePreviewMarkdown,
  noteTitle,
  postSlug,
  quoteIsTruncatedInPreview,
  quotePreviewMarkdown,
} from '../lib/posts'
import { getCachedSiteConfig } from '../lib/site'

const props = defineProps<{
  post: Post
}>()

// Site-wide admin toggle — when on, notes render in full instead of
// truncating at the usual 280-char preview length.
const noteMaxLen = computed(() =>
  getCachedSiteConfig()?.content?.expand_notes_in_feed ? Infinity : 280,
)

const isNote = computed(() => props.post.kind !== 'article')
const titledNote = computed(() => isNote.value && !!noteTitle(props.post.content))
const noteMarkdown = computed(() =>
  props.post.kind === 'quote'
    ? quotePreviewMarkdown(props.post, noteMaxLen.value)
    : notePreviewMarkdown(props.post, noteMaxLen.value),
)
const articleMarkdown = computed(() => articlePreviewMarkdown(props.post))
const truncated = computed(() => {
  if (!isNote.value) return false
  return props.post.kind === 'quote'
    ? quoteIsTruncatedInPreview(props.post, noteMaxLen.value)
    : noteIsTruncatedInPreview(props.post, noteMaxLen.value)
})
</script>

<template>
  <article class="post-card" :class="{ note: isNote, article: !isNote }">
    <div class="card-header">
      <span class="kind">{{ post.kind }}</span>
      <div class="card-header-side">
        <time :datetime="post.created_at">{{ formatDate(post.created_at) }}</time>
        <RouterLink v-if="post.reply_count" class="reply-count" :to="`/p/${postSlug(post.id)}#replies`">
          <MessageCircle :size="13" :stroke-width="2" aria-hidden="true" />
          {{ post.reply_count }} {{ post.reply_count === 1 ? 'reply' : 'replies' }}
        </RouterLink>
      </div>
    </div>

    <template v-if="isNote">
      <h2 v-if="titledNote">
        <RouterLink :to="`/p/${postSlug(post.id)}`">
          {{ noteTitle(post.content) }}
        </RouterLink>
      </h2>
      <div class="note-content">
        <MarkdownBody :content="noteMarkdown" :inline-link-cards="true" />
      </div>
      <RouterLink class="read-more" :to="`/p/${postSlug(post.id)}`">
        {{ truncated ? 'Read more' : 'Permalink' }}
      </RouterLink>
    </template>

    <template v-else>
      <h2>
        <RouterLink :to="`/p/${postSlug(post.id)}`">
          {{ articleTitle(post.content) }}
        </RouterLink>
      </h2>
      <MarkdownBody v-if="articleMarkdown" class="excerpt" :content="articleMarkdown" />
      <RouterLink class="read-more" :to="`/p/${postSlug(post.id)}`">
        Read more
      </RouterLink>
    </template>
  </article>
</template>

<style scoped>
.post-card {
  padding: 1.25rem 0;
  border-bottom: 1px solid var(--border);
}
.post-card:last-child {
  border-bottom: none;
}
.post-card.note {
  border-left: 3px solid color-mix(in srgb, var(--accent) 45%, var(--border));
  padding-left: 1rem;
  margin-left: -0.15rem;
}
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin: 0 0 0.5rem;
  font-size: 0.85rem;
}
.card-header-side {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-shrink: 0;
}
.card-header-side time {
  font-size: 0.8rem;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.reply-count {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  font-size: 0.75rem;
  color: var(--muted);
  text-decoration: none;
  white-space: nowrap;
}
.reply-count:hover {
  color: var(--accent);
  text-decoration: underline;
}
.kind {
  text-transform: capitalize;
  font-weight: 600;
  color: var(--accent);
}
.post-card.note h2 {
  font-family: var(--font-serif);
  font-size: 1.35rem;
  margin: 0 0 0.5rem;
  line-height: 1.3;
}
.post-card.note h2 a {
  color: var(--text);
  text-decoration: none;
}
.post-card.note h2 a:hover {
  color: var(--accent);
}
.note-content {
  font-size: 1.08rem;
  line-height: 1.65;
  color: var(--text);
}
.note-content :deep(.markdown-body > :last-child) {
  margin-bottom: 0;
}
h2 {
  font-family: var(--font-serif);
  font-size: 1.35rem;
  margin: 0 0 0.5rem;
  line-height: 1.3;
}
h2 a {
  color: var(--text);
  text-decoration: none;
}
h2 a:hover {
  color: var(--accent);
}
.excerpt {
  margin: 0 0 0.75rem;
  color: var(--muted);
}
.excerpt :deep(.markdown-body > :last-child) {
  margin-bottom: 0;
}
.excerpt :deep(.figure) {
  margin: 0.75rem 0 0;
}
.excerpt :deep(.figure img) {
  max-height: 220px;
  object-fit: cover;
}
.excerpt :deep(.figure-caption) {
  font-size: 0.78rem;
}
.post-card :deep(.embed) {
  margin: 0.75rem 0;
}
.post-card :deep(.embed-video:not(.yt-facade) iframe) {
  max-height: 220px;
}
.read-more {
  display: inline-block;
  margin-top: 0.5rem;
  font-size: 0.9rem;
  color: var(--accent);
  text-decoration: none;
}
.read-more:hover {
  text-decoration: underline;
}
</style>
