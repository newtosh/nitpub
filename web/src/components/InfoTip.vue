<script setup lang="ts">
import { Info } from '@lucide/vue'

defineProps<{
  label: string
  warn?: boolean
}>()
</script>

<template>
  <button
    type="button"
    class="info-tip"
    :class="{ warn }"
    :title="label"
    :aria-label="label"
  >
    <Info :size="14" :stroke-width="2" aria-hidden="true" />
  </button>
</template>

<style scoped>
.info-tip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin: 0;
  padding: 0;
  border: none;
  background: none;
  color: var(--muted);
  cursor: help;
  vertical-align: middle;
  line-height: 0;
}
.info-tip:hover,
.info-tip:focus-visible {
  color: var(--text);
}
.info-tip.warn {
  color: var(--danger);
}
</style>
