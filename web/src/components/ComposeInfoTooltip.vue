<script setup lang="ts">
import { Info } from '@lucide/vue'
import { onBeforeUnmount, ref, watch } from 'vue'

defineProps<{
  content: string
}>()

const open = ref(false)
const root = ref<HTMLElement | null>(null)

function toggle() {
  open.value = !open.value
}

function close() {
  open.value = false
}

function onDocumentClick(ev: MouseEvent) {
  if (root.value && !root.value.contains(ev.target as Node)) close()
}

function onKeydown(ev: KeyboardEvent) {
  if (ev.key === 'Escape') close()
}

watch(open, (isOpen) => {
  if (isOpen) {
    document.addEventListener('click', onDocumentClick)
    document.addEventListener('keydown', onKeydown)
  } else {
    document.removeEventListener('click', onDocumentClick)
    document.removeEventListener('keydown', onKeydown)
  }
})

onBeforeUnmount(() => {
  document.removeEventListener('click', onDocumentClick)
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div ref="root" class="info-tooltip">
    <button
      type="button"
      class="info-tooltip-trigger"
      :aria-expanded="open"
      aria-controls="compose-info-tooltip-content"
      aria-label="Format info"
      @click="toggle"
    >
      <Info :size="16" :stroke-width="1.75" aria-hidden="true" />
    </button>
    <div v-if="open" id="compose-info-tooltip-content" role="note" class="info-tooltip-content">
      {{ content }}
    </div>
  </div>
</template>

<style scoped>
.info-tooltip {
  position: relative;
  display: inline-flex;
}
.info-tooltip-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.75rem;
  height: 1.75rem;
  border-radius: 50%;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--muted);
  cursor: pointer;
}
.info-tooltip-trigger:hover,
.info-tooltip-trigger[aria-expanded='true'] {
  color: var(--accent);
  border-color: var(--accent);
}
.info-tooltip-content {
  position: absolute;
  top: calc(100% + 0.4rem);
  /* The trigger sits at the row's right edge (right after the kind
     pill toggle), so anchoring from the left and growing rightward
     runs the content straight off the viewport on mobile. Anchor from
     the right instead, and cap width to what's actually available
     inside the page's side padding either way — this trigger isn't
     always this close to the edge, so still needs a hard viewport
     clamp, not just a right anchor. */
  right: 0;
  left: auto;
  z-index: 20;
  width: max(16rem, 60vw);
  max-width: min(20rem, calc(100vw - 2 * var(--header-pad-x)));
  padding: var(--space-3) var(--space-4);
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  font-size: var(--text-sm);
  line-height: 1.5;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
}
</style>
