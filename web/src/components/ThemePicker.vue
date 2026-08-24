<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import MarkdownBody from './MarkdownBody.vue'
import { fetchAppearance, saveTheme } from '../lib/auth'
import { THEMES } from '../lib/theme-catalog'
import { useTheme } from '../composables/useTheme'

const { applyTheme, activeTheme, activeScheme } = useTheme()

const previewId = ref('github')
const savedId = ref('github')
const saving = ref(false)
const saved = ref(false)
const error = ref('')

const dirty = computed(() => previewId.value !== savedId.value)

const sampleMarkdown = `## Sample post

A **note** with a [link](https://example.com) and a short quote:

> Themes should feel readable in feed and permalink views.

\`\`\`
const theme = 'nitpub'
\`\`\`

> [!NOTE]
> Callouts and code blocks follow the active palette.
`

onMounted(async () => {
  const appearance = await fetchAppearance()
  const id = appearance?.theme_id ?? activeTheme.value
  savedId.value = id
  previewId.value = id
})

function selectTheme(id: string) {
  previewId.value = id
  saved.value = false
}

async function commit() {
  saving.value = true
  error.value = ''
  saved.value = false
  const result = await saveTheme(previewId.value)
  saving.value = false
  if (!result.ok) {
    error.value = result.error ?? 'Could not save palette'
    return
  }
  savedId.value = previewId.value
  applyTheme(previewId.value)
  saved.value = true
}

function resetPreview() {
  previewId.value = savedId.value
  saved.value = false
}
</script>

<template>
  <section class="theme-picker stack">
    <p class="text-muted">
      Choose the site-wide palette. Light/dark mode is personal — use the icons in the header. Preview below does not affect the public blog until you save.
    </p>

    <div class="theme-grid" role="listbox" aria-label="Theme palettes">
      <button
        v-for="theme in THEMES"
        :key="theme.id"
        type="button"
        class="theme-card"
        :class="{ selected: previewId === theme.id }"
        :aria-selected="previewId === theme.id"
        :title="`${theme.name} — ${theme.description}`"
        :aria-label="`Preview ${theme.name} theme`"
        @click="selectTheme(theme.id)"
      >
        <div
          class="tile-mockup"
          :data-theme="theme.id"
          :data-scheme="activeScheme"
          aria-hidden="true"
        >
          <header class="tile-header">
            <span class="tile-brand">nitpub</span>
            <nav class="tile-nav">
              <span class="tile-nav-link">RSS</span>
              <span class="tile-nav-icon tile-nav-icon--active" />
              <span class="tile-nav-icon" />
            </nav>
          </header>
          <div class="tile-main">
            <span class="tile-kind">note</span>
            <span class="tile-line tile-line--heading" />
            <span class="tile-line" />
            <span class="tile-line tile-line--short" />
            <span class="tile-code" />
          </div>
        </div>
        <span class="tile-caption">
          <strong>{{ theme.name }}</strong>
          <span>{{ theme.description }}</span>
        </span>
      </button>
    </div>

    <div class="theme-preview card" :data-theme="previewId" :data-scheme="activeScheme">
      <div class="preview-chrome">
        <span class="preview-brand">nitpub</span>
        <span class="preview-pill">Palette preview</span>
      </div>
      <article class="preview-article">
        <p class="preview-meta"><span class="kind">note</span> Preview</p>
        <MarkdownBody :content="sampleMarkdown" />
      </article>
    </div>

    <div class="form-actions">
      <button
        v-if="dirty"
        type="button"
        class="btn btn-ghost"
        title="Revert preview"
        aria-label="Revert preview"
        @click="resetPreview"
      >
        Revert
      </button>
      <button
        type="button"
        class="btn btn-primary"
        :disabled="saving || !dirty"
        title="Save palette"
        aria-label="Save palette"
        @click="commit"
      >
        {{ saving ? 'Saving…' : 'Save palette' }}
      </button>
    </div>
    <p v-if="saved" class="text-muted">Palette saved. Reload any open tabs to see it everywhere.</p>
    <p v-if="error" class="alert alert-error">{{ error }}</p>
  </section>
</template>

<style scoped>
.theme-grid {
  display: grid;
  gap: var(--space-3);
  grid-template-columns: repeat(auto-fill, minmax(11rem, 1fr));
}
.theme-card {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-2);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
  cursor: pointer;
  text-align: left;
  font: inherit;
  color: inherit;
}
.theme-card.selected {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 25%, transparent);
}
.tile-mockup {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  overflow: hidden;
  background: var(--bg);
  min-height: 6.5rem;
}
.tile-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  padding: 0.35rem 0.45rem;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
}
.tile-brand {
  font-family: var(--font-serif);
  font-size: 0.62rem;
  font-weight: 600;
  color: var(--text);
  line-height: 1;
}
.tile-nav {
  display: flex;
  align-items: center;
  gap: 0.3rem;
}
.tile-nav-link {
  font-size: 0.5rem;
  color: var(--muted);
  line-height: 1;
}
.tile-nav-icon {
  width: 0.45rem;
  height: 0.45rem;
  border-radius: 50%;
  background: var(--muted);
  flex-shrink: 0;
}
.tile-nav-icon--active {
  background: var(--accent);
}
.tile-main {
  display: flex;
  flex-direction: column;
  gap: 0.28rem;
  padding: 0.45rem;
  flex: 1;
}
.tile-kind {
  font-size: 0.45rem;
  font-weight: 600;
  color: var(--accent);
  text-transform: capitalize;
  line-height: 1;
}
.tile-line {
  display: block;
  height: 0.28rem;
  border-radius: 1px;
  background: var(--border);
}
.tile-line--heading {
  width: 72%;
  height: 0.34rem;
  background: var(--text);
  opacity: 0.35;
}
.tile-line--short {
  width: 55%;
}
.tile-code {
  display: block;
  height: 0.9rem;
  margin-top: 0.15rem;
  border-radius: 2px;
  border: 1px solid var(--code-border);
  background: var(--code-bg);
}
.tile-caption {
  display: grid;
  gap: 0.1rem;
  padding: 0 0.15rem;
  font-size: var(--text-xs);
  color: var(--muted);
}
.tile-caption strong {
  color: var(--text);
  font-size: var(--text-sm);
}
.theme-preview {
  padding: var(--space-4);
}
.preview-chrome {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--space-4);
  padding-bottom: var(--space-3);
  border-bottom: 1px solid var(--border);
}
.preview-brand {
  font-family: var(--font-serif);
  font-weight: 600;
}
.preview-pill {
  font-size: var(--text-xs);
  color: var(--muted);
}
.preview-meta {
  margin: 0 0 var(--space-2);
  font-size: var(--text-xs);
  color: var(--muted);
}
.preview-meta .kind {
  color: var(--accent);
  font-weight: 600;
  text-transform: capitalize;
  margin-right: var(--space-2);
}
</style>
