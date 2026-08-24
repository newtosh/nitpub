<script setup lang="ts">
import { navIcon } from '../lib/icons'
import type { LinkEntry } from '../lib/site'

defineProps<{
  title?: string
  links: LinkEntry[]
}>()
</script>

<template>
  <header v-if="title" class="page-header">
    <h1>{{ title }}</h1>
  </header>
  <ul class="link-list">
    <li v-for="(link, i) in links" :key="i">
      <a :href="link.url" target="_blank" rel="noopener noreferrer">
        <component :is="navIcon(link.icon)" v-if="navIcon(link.icon)" class="link-icon" :size="18" :stroke-width="1.75" aria-hidden="true" />
        <span class="link-text">
          <strong>{{ link.title }}</strong>
          <span v-if="link.description" class="desc">{{ link.description }}</span>
        </span>
      </a>
    </li>
  </ul>
</template>

<style scoped>
.page-header h1 {
  font-family: var(--font-serif);
  margin: 0 0 1.25rem;
}
.link-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: var(--space-3);
}
.link-list a {
  display: flex;
  align-items: flex-start;
  gap: var(--space-3);
  padding: var(--space-4);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  text-decoration: none;
  color: inherit;
}
.link-list a:hover {
  border-color: var(--accent);
}
.link-icon {
  flex-shrink: 0;
  color: var(--muted);
  margin-top: 0.1rem;
}
.link-text {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}
.desc {
  color: var(--muted);
  font-size: var(--text-sm);
}
</style>
