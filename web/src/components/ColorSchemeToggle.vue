<script setup lang="ts">
import { Monitor, Moon, Sun } from '@lucide/vue'
import { onMounted, onUnmounted, ref } from 'vue'
import { COLOR_SCHEMES, type ColorScheme } from '../lib/theme-catalog'
import { useTheme } from '../composables/useTheme'

const { activeScheme, applyColorScheme } = useTheme()

const open = ref(false)
const root = ref<HTMLElement | null>(null)

const icons: Record<ColorScheme, typeof Sun> = {
  auto: Monitor,
  light: Sun,
  dark: Moon,
}

function select(scheme: ColorScheme) {
  applyColorScheme(scheme)
  open.value = false
}

function onDocClick(e: MouseEvent) {
  if (!root.value?.contains(e.target as Node)) {
    open.value = false
  }
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    open.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', onDocClick)
  document.addEventListener('keydown', onKeydown)
})

onUnmounted(() => {
  document.removeEventListener('click', onDocClick)
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div ref="root" class="scheme-dropdown">
    <button
      type="button"
      class="nav-icon"
      :class="{ active: open }"
      aria-haspopup="listbox"
      :aria-expanded="open"
      aria-label="Color mode"
      :title="COLOR_SCHEMES.find((s) => s.id === activeScheme)?.description"
      @click.stop="open = !open"
    >
      <component :is="icons[activeScheme]" :size="20" :stroke-width="1.75" aria-hidden="true" />
    </button>
    <ul v-if="open" class="scheme-menu" role="listbox" aria-label="Color mode">
      <li v-for="scheme in COLOR_SCHEMES" :key="scheme.id" role="presentation">
        <button
          type="button"
          class="scheme-option"
          role="option"
          :aria-selected="activeScheme === scheme.id"
          @click="select(scheme.id)"
        >
          <component :is="icons[scheme.id]" :size="16" :stroke-width="1.75" aria-hidden="true" />
          <span>{{ scheme.name }}</span>
        </button>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.scheme-dropdown {
  position: relative;
  display: inline-flex;
}
.nav-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 0;
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  color: var(--muted);
  font: inherit;
}
.nav-icon:hover,
.nav-icon.active {
  color: var(--accent);
}
.scheme-menu {
  position: absolute;
  top: calc(100% + var(--space-2));
  right: 0;
  z-index: 20;
  min-width: 8.5rem;
  margin: 0;
  padding: var(--space-1);
  list-style: none;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
  box-shadow: var(--shadow-md);
}
.scheme-option {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  width: 100%;
  padding: var(--space-2) var(--space-3);
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text);
  font: inherit;
  font-size: var(--text-sm);
  text-align: left;
  cursor: pointer;
}
.scheme-option:hover {
  background: var(--bg);
  color: var(--accent);
}
.scheme-option[aria-selected='true'] {
  color: var(--accent);
  font-weight: 600;
}
</style>
