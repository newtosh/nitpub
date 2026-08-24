<script setup lang="ts">
import { ChevronDown, ChevronRight, CornerDownRight, ExternalLink } from '@lucide/vue'
import { ref } from 'vue'
import MastodonIcon from './icons/MastodonIcon.vue'
import { actorHandle } from '../lib/actorHandle'
import { formatDate, relativeTime } from '../lib/posts'
import { countReplyTree, type ReplyNode } from '../lib/replies'

const props = withDefaults(
  defineProps<{
    node: ReplyNode
    depth?: number
    showAvatars?: boolean
  }>(),
  { depth: 0, showAvatars: true },
)

// Collapsed by default beyond the root level, so a long nested thread
// doesn't dominate the page — expandable per-branch, not all-or-nothing.
const expanded = ref(props.depth === 0)
const childCount = countReplyTree(props.node.children)
</script>

<template>
  <li class="reply-item" :class="{ nested: depth > 0 }">
    <span v-if="depth > 0" class="nested-icon" aria-hidden="true">
      <CornerDownRight :size="16" :stroke-width="2" />
    </span>

    <p class="reply-author">
      <img
        v-if="showAvatars && node.reply.avatar_url"
        :src="node.reply.avatar_url"
        alt=""
        class="reply-avatar"
        loading="lazy"
      />
      <span class="reply-name">{{ node.reply.author_name || node.reply.actor }}</span>
      <span
        class="reply-handle"
        tabindex="0"
        role="img"
        :title="actorHandle(node.reply.actor)"
        :aria-label="actorHandle(node.reply.actor)"
      >
        <MastodonIcon :size="13" />
      </span>

      <span class="reply-author-right">
        <time
          v-if="node.reply.received_at"
          class="reply-meta"
          :datetime="node.reply.received_at"
          :title="formatDate(node.reply.received_at)"
        >
          {{ relativeTime(node.reply.received_at) }}
        </time>
        <a
          v-if="node.reply.url"
          :href="node.reply.url"
          target="_blank"
          rel="noopener noreferrer nofollow"
          class="reply-open-link"
          title="View on Mastodon"
          aria-label="View on Mastodon"
        >
          <ExternalLink :size="14" :stroke-width="1.75" aria-hidden="true" />
        </a>
      </span>
    </p>
    <!-- eslint-disable-next-line vue/no-v-html -->
    <div class="reply-content" v-html="node.reply.content" />
    <p class="reply-footer">
      <button
        v-if="node.children.length > 0"
        type="button"
        class="thread-toggle"
        :aria-expanded="expanded"
        @click="expanded = !expanded"
      >
        <ChevronDown v-if="expanded" :size="13" :stroke-width="2" aria-hidden="true" />
        <ChevronRight v-else :size="13" :stroke-width="2" aria-hidden="true" />
        {{ expanded ? 'Hide' : 'Show' }} {{ childCount }} {{ childCount === 1 ? 'reply' : 'replies' }}
      </button>
    </p>

    <ul v-if="node.children.length > 0 && expanded" class="reply-children">
      <ReplyThread
        v-for="child in node.children"
        :key="child.reply.object_id || child.reply.actor + child.reply.received_at"
        :node="child"
        :depth="depth + 1"
        :show-avatars="showAvatars"
      />
    </ul>
  </li>
</template>

<style scoped>
.reply-item {
  position: relative;
  padding: 0.85rem 1rem;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--surface) 85%, var(--border));
}
.reply-item.nested {
  border-radius: 0;
  border-left: 2rem solid color-mix(in srgb, var(--accent) 45%, var(--border));
  background: color-mix(in srgb, var(--surface) 92%, var(--border));
}
.nested-icon {
  position: absolute;
  top: 50%;
  /* Absolutely positioned children are placed relative to the padding
     edge, not the border edge — offset left by the border-left width
     (2rem) so this sits centered within the colored strip itself. */
  left: -2rem;
  width: 2rem;
  display: flex;
  align-items: center;
  justify-content: center;
  transform: translateY(-50%);
  color: #fff;
  line-height: 0;
}
.reply-author {
  display: flex;
  align-items: center;
  gap: var(--space-2, 0.5rem);
  margin: 0 0 0.35rem;
  font-weight: 600;
  font-size: 0.85rem;
}
.reply-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.reply-author-right {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  margin-left: auto;
  padding-left: 0.5rem;
  flex-shrink: 0;
}
.reply-open-link {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--muted);
}
.reply-open-link:hover {
  color: var(--accent);
}
.reply-avatar {
  width: 1.5rem;
  height: 1.5rem;
  border-radius: 50%;
  object-fit: cover;
  flex-shrink: 0;
}
.reply-handle {
  display: inline-flex;
  align-items: center;
  color: var(--muted);
  cursor: help;
}
.reply-handle:hover,
.reply-handle:focus-visible {
  color: var(--accent);
  outline: none;
}
.reply-content {
  margin: 0 0 0.35rem;
  font-size: 0.95rem;
  line-height: 1.55;
}
.reply-content :deep(p) {
  margin: 0 0 0.5em;
}
.reply-content :deep(p:last-child) {
  margin-bottom: 0;
}
.reply-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin: 0;
}
.reply-meta {
  color: var(--muted);
  font-size: 0.78rem;
  font-weight: 400;
  white-space: nowrap;
}
.thread-toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0;
  border: none;
  background: none;
  color: var(--muted);
  font: inherit;
  font-size: 0.8rem;
  font-weight: 500;
  text-decoration: none;
  cursor: pointer;
}
.thread-toggle:hover {
  color: var(--accent);
  text-decoration: underline;
}
.reply-children {
  list-style: none;
  margin: 0.75rem 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
</style>
