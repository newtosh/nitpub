<script setup lang="ts">
import { RouterLink } from 'vue-router'

export type AdminSection = {
  id: string
  title: string
}

defineProps<{
  sections: AdminSection[]
  activeId: string
}>()
</script>

<template>
  <nav class="admin-subnav" aria-label="Admin sections">
    <div class="admin-subnav-inner cluster">
      <RouterLink
        v-for="section in sections"
        :key="section.id"
        :to="`/admin/${section.id}`"
        class="subnav-link"
        :class="{ active: section.id === activeId }"
        :aria-current="section.id === activeId ? 'page' : undefined"
      >
        {{ section.title }}
      </RouterLink>
    </div>
  </nav>
</template>

<style scoped>
.admin-subnav {
  width: 100%;
  background: var(--surface);
}
.admin-subnav-inner {
  max-width: var(--content-max);
  margin: 0 auto;
  padding: var(--space-2) var(--header-pad-x);
  gap: var(--space-1);
  flex-wrap: wrap;
  justify-content: center;
}
.subnav-link {
  padding: 0.35rem 0.7rem;
  border-radius: var(--radius);
  font-size: var(--text-sm);
  color: var(--muted);
  text-decoration: none;
  line-height: 1.3;
}
.subnav-link:hover {
  color: var(--text);
  background: color-mix(in srgb, var(--border) 40%, transparent);
}
.subnav-link.active {
  color: var(--accent);
  background: color-mix(in srgb, var(--accent) 12%, transparent);
  font-weight: 600;
}
</style>
