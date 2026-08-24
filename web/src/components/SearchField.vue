<script setup lang="ts">
import { Search, X } from '@lucide/vue'
import { ref } from 'vue'

const model = defineModel<string>({ required: true })

withDefaults(
  defineProps<{
    placeholder?: string
    ariaLabel?: string
    bordered?: boolean
  }>(),
  {
    placeholder: 'Search…',
    ariaLabel: 'Search',
    bordered: true,
  },
)

const inputRef = ref<HTMLInputElement | null>(null)

defineEmits<{
  keydown: [event: KeyboardEvent]
}>()

function clear() {
  model.value = ''
  inputRef.value?.focus()
}

defineExpose({
  focus: () => inputRef.value?.focus(),
})
</script>

<template>
  <label class="search-field" :class="{ bordered }">
    <Search :size="16" :stroke-width="1.75" class="search-icon" aria-hidden="true" />
    <input
      ref="inputRef"
      v-model="model"
      type="search"
      :placeholder="placeholder"
      :aria-label="ariaLabel"
      autocomplete="off"
      @keydown="$emit('keydown', $event)"
    />
    <button
      v-if="model"
      type="button"
      class="clear-btn"
      aria-label="Clear search"
      title="Clear search"
      @click="clear"
    >
      <X :size="14" :stroke-width="2" aria-hidden="true" />
    </button>
    <slot name="suffix" />
  </label>
</template>

<style scoped>
.search-field {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-width: 0;
  flex: 1;
}
.search-field.bordered {
  padding: 0.35rem 0.65rem;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--surface);
}
.search-icon {
  flex-shrink: 0;
  color: var(--muted);
}
.search-field input {
  flex: 1;
  min-width: 0;
  border: none;
  background: transparent;
  color: var(--text);
  font: inherit;
  font-size: var(--text-sm);
  outline: none;
}
@media (max-width: 47.99rem) {
  .search-field input {
    /* iOS Safari auto-zooms on focus when an input's font-size is under 16px */
    font-size: 16px;
  }
}
.search-field input::placeholder {
  color: var(--muted);
}
.clear-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  min-width: 2rem;
  min-height: 2rem;
  padding: 0.4rem;
  margin: -0.25rem;
  border: none;
  border-radius: var(--radius);
  background: none;
  color: var(--muted);
  cursor: pointer;
  line-height: 0;
}
.clear-btn:hover {
  color: var(--text);
  background: var(--surface-2, color-mix(in srgb, var(--border) 50%, transparent));
}
</style>
