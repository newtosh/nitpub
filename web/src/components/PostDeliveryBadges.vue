<script setup lang="ts">
import { Globe } from '@lucide/vue'
import { computed } from 'vue'
import type { Post } from '../lib/posts'
import { deliveryBadges } from '../lib/federationDelivery'
import MastodonIcon from './icons/MastodonIcon.vue'
import BlueskyIcon from './icons/BlueskyIcon.vue'

const props = defineProps<{
  post: Post
}>()

const badges = computed(() => deliveryBadges(props.post))
</script>

<template>
  <span class="delivery-badges" aria-label="Published to">
    <component
      :is="badge.href ? 'a' : 'span'"
      v-for="badge in badges"
      :key="badge.label"
      class="badge"
      :class="badge.tone"
      :title="badge.title"
      :href="badge.href"
      :target="badge.href ? '_blank' : undefined"
      :rel="badge.href ? 'noopener noreferrer nofollow' : undefined"
    >
      <Globe
        v-if="badge.icon === 'site'"
        :size="11"
        :stroke-width="2"
        aria-hidden="true"
        class="badge-icon"
      />
      <MastodonIcon v-else-if="badge.icon === 'fediverse'" :size="11" class="badge-icon" />
      <BlueskyIcon v-else :size="11" class="badge-icon" />
      <span class="badge-label">{{ badge.label }}</span>
    </component>
  </span>
</template>

<style scoped>
.delivery-badges {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  align-items: center;
}
.badge {
  display: inline-flex;
  align-items: center;
  gap: 0.28rem;
  padding: 0.1rem 0.45rem 0.1rem 0.35rem;
  border-radius: 999px;
  font-size: 0.68rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  text-transform: uppercase;
  border: 1px solid var(--border);
  color: var(--muted);
  background: color-mix(in srgb, var(--surface) 80%, var(--border));
  text-decoration: none;
}
.badge-icon {
  flex-shrink: 0;
}
.badge-label {
  line-height: 1;
}
.badge.default {
  color: var(--accent);
  border-color: color-mix(in srgb, var(--accent) 35%, var(--border));
  background: color-mix(in srgb, var(--accent) 8%, transparent);
}
.badge.warn {
  color: var(--warn);
  border-color: color-mix(in srgb, var(--warn) 45%, var(--border));
  background: color-mix(in srgb, var(--warn) 10%, transparent);
}
</style>
