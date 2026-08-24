<script setup lang="ts">
import { CornerDownRight, FileText } from '@lucide/vue'
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import MastodonIcon from './icons/MastodonIcon.vue'
import { actorHandle } from '../lib/actorHandle'
import { formatDate } from '../lib/posts'
import type { PendingReply } from '../lib/moderationAdmin'

const props = defineProps<{
  reply: PendingReply
  showAvatars: boolean
  postTag: { title: string; known: boolean }
  /** Shown as a colored badge when set — the Reviewed tab's decided status. */
  status?: 'approved' | 'rejected' | 'skipped'
}>()

// Older entries (seeded/migrated before ParentActor/ParentAuthorName
// existed) may have neither set even when nested — fall back to a generic
// label instead of interpolating "undefined" into the tooltip/text.
const parentLabel = computed(
  () => props.reply.parent_author_name || props.reply.parent_actor || 'another reply',
)
</script>

<template>
  <p class="reply-author">
    <img
      v-if="showAvatars && reply.avatar_url"
      :src="reply.avatar_url"
      alt=""
      class="reply-avatar"
      loading="lazy"
    />
    {{ reply.author_name || reply.actor }}
    <span
      class="reply-handle"
      tabindex="0"
      role="img"
      :title="actorHandle(reply.actor)"
      :aria-label="actorHandle(reply.actor)"
    >
      <MastodonIcon :size="13" />
    </span>
    <span v-if="!reply.verified" class="unverified-badge" title="Migrated from before moderation existed — actor identity was not signature-verified">
      unverified, migrated
    </span>
    <span
      v-if="reply.nested"
      class="nested-tag"
      :title="`Not a direct reply to the post — addressed to ${parentLabel}`"
    >
      <CornerDownRight :size="12" :stroke-width="2" aria-hidden="true" />
      Reply to {{ parentLabel }}
    </span>
    <span class="reply-tags">
      <span v-if="status" class="status-badge" :class="status">{{ status }}</span>
      <RouterLink
        v-if="postTag.known"
        :to="`/p/${reply.post_slug}`"
        target="_blank"
        rel="noopener noreferrer"
        class="post-tag"
        :title="`Open post: ${postTag.title}`"
      >
        <FileText :size="12" :stroke-width="2" aria-hidden="true" />
        {{ postTag.title }}
      </RouterLink>
      <span v-else class="post-tag post-tag-unknown" title="Post no longer exists">
        <FileText :size="12" :stroke-width="2" aria-hidden="true" />
        Post unavailable
      </span>
    </span>
  </p>
  <!-- eslint-disable-next-line vue/no-v-html -->
  <div class="reply-content" v-html="reply.content" />
  <time v-if="reply.received_at" class="reply-meta" :datetime="reply.received_at">
    {{ formatDate(reply.received_at) }}
  </time>
</template>

<style scoped>
.reply-tags {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  max-width: 100%;
  margin-left: auto;
}
.post-tag {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  max-width: 100%;
  padding: 0.15rem 0.55rem;
  border-radius: 999px;
  background: color-mix(in srgb, var(--accent) 12%, transparent);
  color: var(--accent);
  font-size: var(--text-xs);
  font-weight: 600;
  text-decoration: none;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.post-tag:hover {
  background: color-mix(in srgb, var(--accent) 20%, transparent);
  text-decoration: underline;
}
.post-tag-unknown {
  background: color-mix(in srgb, var(--muted) 15%, transparent);
  color: var(--muted);
  cursor: default;
}
.nested-tag {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  color: var(--muted);
  font-size: var(--text-xs);
  font-weight: 500;
  white-space: nowrap;
}
.status-badge {
  padding: 0.1rem 0.5rem;
  border-radius: var(--radius-sm);
  font-size: var(--text-xs);
  font-weight: 600;
  text-transform: capitalize;
  white-space: nowrap;
}
.status-badge.approved {
  background: color-mix(in srgb, var(--success, var(--accent)) 18%, transparent);
  color: var(--success, var(--accent));
}
.status-badge.rejected {
  background: color-mix(in srgb, var(--danger) 18%, transparent);
  color: var(--danger);
}
.status-badge.skipped {
  background: color-mix(in srgb, var(--muted) 20%, transparent);
  color: var(--muted);
}
.reply-author {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
  margin: 0 0 var(--space-1);
  font-weight: 600;
  font-size: var(--text-sm);
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
  border-radius: var(--radius-sm);
}
.reply-handle:hover,
.reply-handle:focus-visible {
  color: var(--accent);
  outline: none;
}
.unverified-badge {
  margin-left: var(--space-2);
  padding: 0.05rem 0.4rem;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--danger) 15%, transparent);
  color: var(--danger);
  font-size: var(--text-xs);
  font-weight: 500;
  text-transform: none;
}
.reply-content {
  margin: 0 0 var(--space-1);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
}
.reply-content :deep(p) {
  margin: 0 0 0.5em;
}
.reply-content :deep(p:last-child) {
  margin-bottom: 0;
}
.reply-meta {
  margin: 0 0 var(--space-2);
  color: var(--muted);
  font-size: var(--text-xs);
}
</style>
