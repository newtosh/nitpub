<script setup lang="ts">
import { computed, ref } from 'vue'
import InfoTip from './InfoTip.vue'
import MarkdownBody from './MarkdownBody.vue'
import type { EditorProfile } from '../lib/federationProfile'
import { federationHint } from '../lib/federationProfile'
import { renderFederatedNotePreview } from '../lib/federationPreview'
import { fetchIconCatalog, searchIconCatalog, type IconCatalogEntry } from '../lib/iconCatalog'
import { toggleBold, toggleItalic, toggleLink, toggleWrap } from '../lib/markdownToggle'
import { fetchIconSVG } from '../lib/phosphorIcons'
import { noteBody, noteTitle } from '../lib/posts'

defineOptions({ inheritAttrs: false })

const model = defineModel<string>({ required: true })

// A Boolean-typed prop that's absent (not just undefined) is cast to
// false by Vue when no default is declared — withDefaults is required
// here, a `?? true` fallback on props.showPreviewTab never sees true.
const props = withDefaults(
  defineProps<{
    placeholder?: string
    rows?: number
    profile?: EditorProfile
    showHint?: boolean
    // Hides the Write/Preview tab bar for an editor embedded inside a
    // larger form that already has its own combined preview (e.g.
    // ComposeForm's quote fields) — the per-field preview would just be
    // redundant chrome.
    showPreviewTab?: boolean
    // Applied to the underlying textarea (not the component root) so an
    // outer <label for="..."> can target the actual control — the
    // toolbar's buttons are labelable and precede the textarea in the
    // DOM, so an implicit label association would otherwise bind to the
    // first button instead of the textarea.
    id?: string
  }>(),
  { showPreviewTab: true, showHint: true },
)

const profile = computed(() => props.profile ?? 'article')
const isNote = computed(() => profile.value === 'note')

const tab = ref<'write' | 'preview'>('write')
const previewMode = ref<'site' | 'fediverse'>('site')
const textarea = ref<HTMLTextAreaElement | null>(null)
const editorRoot = ref<HTMLDivElement | null>(null)
const uploadError = ref('')
const fileInput = ref<HTMLInputElement | null>(null)

const iconCatalog = ref<IconCatalogEntry[]>([])
const iconAutocompleteOpen = ref(false)
const iconQuery = ref('')
const iconResults = ref<IconCatalogEntry[]>([])
const iconActiveIndex = ref(0)
const iconPreviewSvg = ref<Record<string, string>>({})
let iconTriggerStart = 0

function scrollToolbarIntoView() {
  editorRoot.value?.scrollIntoView({ block: 'start', behavior: 'instant' })
}

// iOS Safari's own scroll-into-view on focus fires before the keyboard
// finishes animating and guesses wrong, cutting the toolbar off above the
// fold. Wait for visualViewport to actually settle (keyboard fully open),
// then override it once — pinning the toolbar's top edge, not the
// textarea's, to the top of the shrunk viewport.
function onTextareaFocus() {
  window.visualViewport?.addEventListener('resize', scrollToolbarIntoView, { once: true })
}

const preview = computed(() => model.value)
// Mirrors PostPage.vue's own note rendering (optional leading H1 stripped
// into a title, same as the live site) — the site-preview tab should show
// exactly what a reader will see, not the raw draft including that marker.
const notePreviewTitle = computed(() => noteTitle(model.value))
const notePreviewBody = computed(() => noteBody(model.value))
const federatedPreview = computed(() =>
  isNote.value ? renderFederatedNotePreview(model.value) : '',
)

function focusTextarea() {
  tab.value = 'write'
  requestAnimationFrame(() => textarea.value?.focus())
}

function applyEdit(result: { text: string; selStart: number; selEnd: number }) {
  model.value = result.text
  requestAnimationFrame(() => {
    const el = textarea.value
    if (!el) return
    el.focus()
    el.setSelectionRange(result.selStart, result.selEnd)
  })
}

function withSelection(fn: (text: string, start: number, end: number) => ReturnType<typeof toggleWrap>) {
  const el = textarea.value
  if (!el) return
  applyEdit(fn(model.value, el.selectionStart, el.selectionEnd))
}

function insertLine(prefix: string) {
  const el = textarea.value
  if (!el) return
  const start = el.selectionStart
  const before = model.value.lastIndexOf('\n', start - 1) + 1
  const next = model.value.slice(0, before) + prefix + model.value.slice(before)
  model.value = next
  requestAnimationFrame(() => {
    el.focus()
    const cursor = start + prefix.length
    el.setSelectionRange(cursor, cursor)
  })
}

function onToolbar(action: string) {
  focusTextarea()
  switch (action) {
    case 'bold':
      withSelection(toggleBold)
      break
    case 'italic':
      withSelection(toggleItalic)
      break
    case 'code':
      withSelection((text, start, end) =>
        toggleWrap(text, start, end, { before: '`', after: '`', placeholder: 'code' }),
      )
      break
    case 'link':
      withSelection(toggleLink)
      break
    case 'quote':
      insertLine('> ')
      break
    case 'ul':
      insertLine('- ')
      break
    case 'ol':
      insertLine('1. ')
      break
    case 'note':
      insertBlock('> [!NOTE]\n> ')
      break
    case 'image':
      fileInput.value?.click()
      break
  }
}

function insertBlock(block: string) {
  const el = textarea.value
  if (!el) return
  const start = el.selectionStart
  const prefix = start > 0 && model.value[start - 1] !== '\n' ? '\n\n' : '\n'
  model.value = model.value.slice(0, start) + prefix + block + '\n' + model.value.slice(start)
  focusTextarea()
}

// Trigger: a `:` preceded by whitespace or the start of the text, with
// nothing but lowercase/digit/hyphen characters typed since — matches the
// same shape markdown.ts's iconShortcode rule expects, minus the closing
// colon (which isn't typed yet while the dropdown is open).
const iconTriggerRe = /(?:^|\s)(:[a-z0-9-]*)$/

function closeIconAutocomplete() {
  iconAutocompleteOpen.value = false
  iconResults.value = []
  iconActiveIndex.value = 0
}

async function detectIconTrigger() {
  const el = textarea.value
  if (!el) return
  const cursor = el.selectionStart
  const before = model.value.slice(0, cursor)
  const match = before.match(iconTriggerRe)
  if (!match) {
    closeIconAutocomplete()
    return
  }
  iconTriggerStart = cursor - match[1].length
  iconQuery.value = match[1].slice(1)
  iconAutocompleteOpen.value = true
  iconActiveIndex.value = 0

  if (iconCatalog.value.length === 0) {
    iconCatalog.value = await fetchIconCatalog()
    // The trigger may have closed (or moved on) while the fetch was in
    // flight — re-check before using a possibly-stale query position.
    if (!iconAutocompleteOpen.value) return
  }
  iconResults.value = searchIconCatalog(iconCatalog.value, iconQuery.value)
  for (const result of iconResults.value) {
    if (iconPreviewSvg.value[result.name]) continue
    fetchIconSVG(result.name)
      .then((svg) => {
        iconPreviewSvg.value = { ...iconPreviewSvg.value, [result.name]: svg }
      })
      .catch(() => {})
  }
}

function insertIcon(name: string) {
  const el = textarea.value
  if (!el) return
  const cursor = el.selectionStart
  const snippet = `:${name}: `
  model.value = model.value.slice(0, iconTriggerStart) + snippet + model.value.slice(cursor)
  closeIconAutocomplete()
  requestAnimationFrame(() => {
    el.focus()
    const pos = iconTriggerStart + snippet.length
    el.setSelectionRange(pos, pos)
  })
}

function onTextareaInput() {
  detectIconTrigger()
}

function onKeydown(event: KeyboardEvent) {
  if (iconAutocompleteOpen.value && iconResults.value.length > 0) {
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      iconActiveIndex.value = (iconActiveIndex.value + 1) % iconResults.value.length
      return
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault()
      iconActiveIndex.value =
        (iconActiveIndex.value - 1 + iconResults.value.length) % iconResults.value.length
      return
    }
    if (event.key === 'Tab' || event.key === 'Enter') {
      event.preventDefault()
      insertIcon(iconResults.value[iconActiveIndex.value].name)
      return
    }
    if (event.key === 'Escape') {
      event.preventDefault()
      closeIconAutocomplete()
      return
    }
  }

  if (!(event.ctrlKey || event.metaKey) || event.altKey) return
  const key = event.key.toLowerCase()
  if (key === 'b') {
    event.preventDefault()
    onToolbar('bold')
  } else if (key === 'i') {
    event.preventDefault()
    onToolbar('italic')
  } else if (key === 'k') {
    event.preventDefault()
    onToolbar('link')
  }
}

async function onImageSelected(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  uploadError.value = ''
  const form = new FormData()
  form.append('file', file)
  try {
    const res = await fetch('/api/media', {
      method: 'POST',
      credentials: 'include',
      body: form,
    })
    if (!res.ok) {
      uploadError.value = (await res.text()) || 'Upload failed'
      return
    }
    const data = (await res.json()) as { url: string }
    const alt = file.name.replace(/\.[^.]+$/, '')
    const el = textarea.value
    const start = el?.selectionStart ?? model.value.length
    const snippet = `![${alt}](${data.url})`
    model.value = model.value.slice(0, start) + snippet + model.value.slice(start)
    focusTextarea()
  } catch {
    uploadError.value = 'Upload failed'
  }
}

function onDrop(event: DragEvent) {
  if (isNote.value) return
  const file = event.dataTransfer?.files?.[0]
  if (!file || !file.type.startsWith('image/')) return
  event.preventDefault()
  const dt = new DataTransfer()
  dt.items.add(file)
  if (fileInput.value) {
    fileInput.value.files = dt.files
    fileInput.value.dispatchEvent(new Event('change'))
  }
}

function onPaste(event: ClipboardEvent) {
  if (isNote.value) return
  const file = event.clipboardData?.files?.[0]
  if (!file || !file.type.startsWith('image/')) return
  event.preventDefault()
  const dt = new DataTransfer()
  dt.items.add(file)
  if (fileInput.value) {
    fileInput.value.files = dt.files
    fileInput.value.dispatchEvent(new Event('change'))
  }
}
</script>

<template>
  <div ref="editorRoot" class="md-editor" :class="{ 'md-editor--note': isNote }">
    <div v-if="props.showPreviewTab" class="md-editor-tabs">
      <button type="button" :class="{ active: tab === 'write' }" title="Write" aria-label="Write" @click="tab = 'write'">
        Write
      </button>
      <button
        type="button"
        :class="{ active: tab === 'preview' }"
        title="Preview"
        aria-label="Preview"
        @click="tab = 'preview'"
      >
        Preview
      </button>
    </div>

    <div class="md-editor-toolbar">
      <button type="button" title="Bold (Ctrl+B)" aria-label="Bold" @click="onToolbar('bold')"><strong>B</strong></button>
      <button type="button" title="Italic (Ctrl+I)" aria-label="Italic" @click="onToolbar('italic')"><em>I</em></button>
      <button type="button" title="Inline code" aria-label="Inline code" @click="onToolbar('code')">`</button>
      <button type="button" title="Link (Ctrl+K)" aria-label="Insert link" @click="onToolbar('link')">Link</button>
      <span class="sep" />
      <button type="button" title="Quote" aria-label="Quote" @click="onToolbar('quote')">Quote</button>
      <button type="button" title="Bullet list" aria-label="Bullet list" @click="onToolbar('ul')">• List</button>
      <button type="button" title="Numbered list" aria-label="Numbered list" @click="onToolbar('ol')">1. List</button>
      <template v-if="!isNote">
        <span class="sep" />
        <button type="button" title="Note callout" aria-label="Note callout" @click="onToolbar('note')">Callout</button>
        <button type="button" title="Upload image" aria-label="Upload image" @click="onToolbar('image')">Image</button>
        <input
          ref="fileInput"
          type="file"
          accept="image/jpeg,image/png,image/gif,image/webp"
          hidden
          @change="onImageSelected"
        />
      </template>
    </div>

    <div v-show="tab === 'write'" class="md-editor-write">
      <textarea
        :id="props.id"
        ref="textarea"
        v-model="model"
        :rows="props.rows ?? 14"
        :placeholder="props.placeholder"
        @drop="onDrop"
        @dragover.prevent
        @paste="onPaste"
        @keydown="onKeydown"
        @input="onTextareaInput"
        @focus="onTextareaFocus"
        @click="detectIconTrigger"
        @blur="closeIconAutocomplete"
      />
      <ul v-if="iconAutocompleteOpen && iconResults.length > 0" class="icon-autocomplete">
        <li
          v-for="(result, i) in iconResults"
          :key="result.name"
          :class="{ active: i === iconActiveIndex }"
          @mousedown.prevent="insertIcon(result.name)"
          @mouseenter="iconActiveIndex = i"
        >
          <span class="icon-glyph" aria-hidden="true" v-html="iconPreviewSvg[result.name] ?? ''" />
          <span class="icon-name">{{ result.name }}</span>
        </li>
      </ul>
    </div>
    <div v-if="props.showPreviewTab" v-show="tab === 'preview'" class="md-editor-preview">
      <div v-if="isNote" class="preview-mode-switch">
        <button
          type="button"
          :class="{ active: previewMode === 'site' }"
          @click="previewMode = 'site'"
        >
          Site
        </button>
        <button
          type="button"
          :class="{ active: previewMode === 'fediverse' }"
          @click="previewMode = 'fediverse'"
        >
          Fediverse
        </button>
        <InfoTip
          label="Site is how this note appears on your blog. Fediverse is how it'll look to followers on Mastodon — a plainer HTML subset, no icons or link cards."
        />
      </div>
      <template v-if="preview.trim()">
        <div
          v-if="isNote && previewMode === 'fediverse'"
          class="markdown-body federation-preview"
          v-html="federatedPreview"
        />
        <template v-else-if="isNote">
          <h1 v-if="notePreviewTitle">{{ notePreviewTitle }}</h1>
          <MarkdownBody class="note-body" :content="notePreviewBody" :inline-link-cards="true" />
        </template>
        <MarkdownBody v-else :content="preview" />
      </template>
      <p v-else class="status">Nothing to preview yet.</p>
    </div>

    <p v-if="uploadError" class="md-editor-upload-error">{{ uploadError }}</p>
    <p v-if="props.showHint" class="md-editor-hint">{{ federationHint(profile) }}</p>
  </div>
</template>

<style scoped>
textarea {
  /* iOS Safari auto-zooms on focus when an input's font-size is under 16px */
  font-size: 1rem;
}
.status {
  color: var(--muted);
  margin: 0;
}
.md-editor--note .md-editor-hint {
  font-size: 0.8rem;
}
.md-editor-write {
  position: relative;
}
.icon-autocomplete {
  position: absolute;
  left: 0.5rem;
  right: 0.5rem;
  bottom: 0.5rem;
  max-height: 12rem;
  overflow-y: auto;
  margin: 0;
  padding: 0.25rem;
  list-style: none;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-md, 0 4px 12px rgb(0 0 0 / 0.2));
  z-index: 5;
}
.icon-autocomplete li {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.35rem 0.5rem;
  border-radius: var(--radius-sm, 4px);
  cursor: pointer;
  font-size: 0.85rem;
  color: var(--text);
}
.icon-autocomplete li.active {
  background: color-mix(in srgb, var(--accent) 15%, transparent);
}
.icon-glyph {
  display: inline-flex;
  flex-shrink: 0;
  width: 14px;
  height: 14px;
  color: var(--text);
}
.icon-glyph :deep(svg) {
  width: 100%;
  height: 100%;
  fill: currentColor;
}
.icon-name {
  font-family: var(--font-mono, monospace);
}
</style>
