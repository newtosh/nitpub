<script setup lang="ts">
import { FileUp, Save, Send, Trash2, X } from '@lucide/vue'
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import MarkdownEditor from './MarkdownEditor.vue'
import ComposeInfoTooltip from './ComposeInfoTooltip.vue'
import MastodonIcon from './icons/MastodonIcon.vue'
import type { Post } from '../lib/posts'
import { postSlug } from '../lib/posts'
import {
  NOTE_MAX_CHARS,
  combineArticleContent,
  noteCharCount,
  noteLengthLabel,
  shouldAutoConvertToArticle,
  splitArticleContent,
} from '../lib/contentKinds'
import { federationHint } from '../lib/federationProfile'
import { crossPostDefaultFromConfig } from '../lib/federationDelivery'
import { fetchSiteConfig } from '../lib/site'
import { readMarkdownFile } from '../lib/markdownFile'
import {
  clearComposeDraft,
  loadComposeDraft,
  saveComposeDraft,
} from '../lib/composeDraft'

const props = defineProps<{
  post?: Post | null
  statusText?: string
  statusVariant?: 'idle' | 'saving' | 'saved' | 'error'
}>()

const emit = defineEmits<{
  publish: [payload: { kind: string; content: string; federate: boolean }]
  save: [payload: { kind: string; content: string }]
  cancel: []
  delete: []
  'draft-change': [payload: { kind: 'note' | 'article'; title: string; content: string }]
}>()

const isEdit = () => !!props.post

const kind = ref<'note' | 'article'>('note')
const noteContent = ref('')
const articleTitle = ref('')
const articleBody = ref('')
const clientError = ref('')
const convertNotice = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const openNotice = ref('')
const shareToFediverse = ref(true)
const federateTrack = ref<HTMLButtonElement | null>(null)
const federateThumb = ref<HTMLSpanElement | null>(null)
const federateThumbTravel = ref(0)

// The thumb (icon + label) is the whole tappable slider, not a separate
// switch bezel — it needs to land flush against the track's opposite
// edge regardless of how wide "Fediverse" renders, so the travel
// distance is measured rather than guessed as a fixed rem value.
function measureFederateTravel() {
  const track = federateTrack.value
  const thumb = federateThumb.value
  if (!track || !thumb) return
  const trackStyle = getComputedStyle(track)
  const inset = parseFloat(trackStyle.paddingLeft) + parseFloat(trackStyle.paddingRight)
  federateThumbTravel.value = Math.max(0, track.clientWidth - thumb.offsetWidth - inset)
}

let federateResizeObserver: ResizeObserver | undefined
const defaultLoaded = ref(false)
const draftNotice = ref('')
const baseline = ref<{ kind: 'note' | 'article'; content: string } | null>(null)

let autosaveTimer: ReturnType<typeof setTimeout> | undefined

const notePlaceholder = `Share a short note for the fediverse.

**Bold**, _italic_, links, quotes, and lists federate. Keep it under ${NOTE_MAX_CHARS} characters.`

const articleBodyPlaceholder = `Write the article body in **Markdown**.

Use callouts, images, embeds, and longer-form structure here.`

const noteCount = computed(() => noteCharCount(noteContent.value))
const noteCounterClass = computed(() => {
  if (noteCount.value > NOTE_MAX_CHARS) return 'over'
  if (noteCount.value > NOTE_MAX_CHARS * 0.9) return 'warn'
  return ''
})

const composedContent = computed(() => {
  if (kind.value === 'article') {
    return combineArticleContent(articleTitle.value, articleBody.value)
  }
  return noteContent.value
})

function currentPayload(): { kind: 'note' | 'article'; content: string } {
  return { kind: kind.value, content: composedContent.value }
}

function normalizePayload(payload: {
  kind: 'note' | 'article'
  content: string
}): { kind: 'note' | 'article'; content: string } {
  if (payload.kind === 'article') {
    const { title, body } = splitArticleContent(payload.content)
    return { kind: 'article', content: combineArticleContent(title, body) }
  }
  return payload
}

function payloadsEqual(
  a: { kind: 'note' | 'article'; content: string },
  b: { kind: 'note' | 'article'; content: string },
): boolean {
  const left = normalizePayload(a)
  const right = normalizePayload(b)
  return left.kind === right.kind && left.content === right.content
}

const isDirty = computed(() => {
  if (!baseline.value || !isEdit()) return false
  return !payloadsEqual(currentPayload(), baseline.value)
})

function applyContent(kindVal: 'note' | 'article', content: string) {
  kind.value = kindVal
  if (kindVal === 'article') {
    const split = splitArticleContent(content)
    articleTitle.value = split.title
    articleBody.value = split.body
    noteContent.value = ''
  } else {
    noteContent.value = content
    articleTitle.value = ''
    articleBody.value = ''
  }
}

function applyPost(post: Post | null | undefined) {
  convertNotice.value = ''
  clientError.value = ''
  draftNotice.value = ''
  if (!post) {
    baseline.value = null
    kind.value = 'note'
    noteContent.value = ''
    articleTitle.value = ''
    articleBody.value = ''
    return
  }

  const serverKind: 'note' | 'article' = post.kind === 'article' ? 'article' : 'note'
  baseline.value = { kind: serverKind, content: post.content }

  const slug = postSlug(post.id)
  const draft = loadComposeDraft(slug)
  if (draft && (draft.kind !== serverKind || draft.content !== post.content)) {
    applyContent(draft.kind, draft.content)
    draftNotice.value = 'Restored unsaved draft from this browser.'
  } else {
    applyContent(serverKind, post.content)
    if (draft) clearComposeDraft(slug)
  }
}

function scheduleAutosave() {
  clearTimeout(autosaveTimer)
  if (isEdit() && props.post?.status !== 'draft') {
    if (!props.post || !isDirty.value) return
    autosaveTimer = setTimeout(() => {
      if (!props.post) return
      saveComposeDraft(postSlug(props.post.id), currentPayload())
    }, 800)
    return
  }

  // New-post mode, or resuming a draft in edit mode (U7): server-side
  // draft autosave (U5), once there's some content — a partial title or
  // partial body.
  const title = kind.value === 'article' ? articleTitle.value.trim() : ''
  const content = kind.value === 'article' ? articleBody.value.trim() : noteContent.value.trim()
  if (!title && !content) return
  autosaveTimer = setTimeout(() => {
    emit('draft-change', { kind: kind.value, title, content })
  }, 800)
}

function markSaved() {
  if (!props.post) return
  clearComposeDraft(postSlug(props.post.id))
  baseline.value = { ...currentPayload() }
  draftNotice.value = ''
}

function clearDraft() {
  if (!props.post) return
  clearComposeDraft(postSlug(props.post.id))
}

function discardChanges() {
  if (!props.post || !baseline.value) return
  applyContent(baseline.value.kind, baseline.value.content)
  clearDraft()
  draftNotice.value = ''
  convertNotice.value = ''
  clientError.value = ''
}

function getDraftPayload(): { kind: 'note' | 'article'; title: string; content: string } {
  return {
    kind: kind.value,
    title: kind.value === 'article' ? articleTitle.value.trim() : '',
    content: kind.value === 'article' ? articleBody.value.trim() : noteContent.value.trim(),
  }
}

defineExpose({
  getDirty: () => isDirty.value,
  markSaved,
  clearDraft,
  discardChanges,
  getDraftPayload,
})

watch([kind, noteContent, articleTitle, articleBody], scheduleAutosave)

onUnmounted(() => {
  clearTimeout(autosaveTimer)
  federateResizeObserver?.disconnect()
})

watch(defaultLoaded, async (loaded) => {
  if (!loaded) return
  await nextTick()
  measureFederateTravel()
  if (federateTrack.value) {
    federateResizeObserver = new ResizeObserver(measureFederateTravel)
    federateResizeObserver.observe(federateTrack.value)
  }
})

watch(() => props.post, applyPost, { immediate: true })

onMounted(async () => {
  if (isEdit()) return
  try {
    const site = await fetchSiteConfig()
    shareToFediverse.value = crossPostDefaultFromConfig(site.federation)
  } catch {
    shareToFediverse.value = true
  } finally {
    defaultLoaded.value = true
  }
})

function promoteToArticle(fromNote: string) {
  const trimmed = fromNote.trim()
  const newline = trimmed.indexOf('\n')
  if (newline === -1) {
    articleTitle.value = trimmed.slice(0, 80)
    articleBody.value = trimmed
  } else {
    const split = splitArticleContent(trimmed)
    articleTitle.value = split.title || trimmed.slice(0, 80)
    articleBody.value = split.body || trimmed.slice(newline + 1).trimStart()
  }
  noteContent.value = ''
  kind.value = 'article'
  convertNotice.value =
    `Switched to article — notes are limited to ${NOTE_MAX_CHARS} characters for fediverse compatibility.`
}

watch(noteContent, (value) => {
  if (kind.value !== 'note') return
  if (shouldAutoConvertToArticle(value)) {
    promoteToArticle(value)
  }
})

watch(kind, (value, previous) => {
  convertNotice.value = ''
  if (value === 'article' && previous === 'note' && noteContent.value.trim()) {
    promoteToArticle(noteContent.value)
    return
  }
  if (value === 'note' && previous === 'article') {
    const merged = combineArticleContent(articleTitle.value, articleBody.value)
    if (shouldAutoConvertToArticle(merged)) {
      kind.value = 'article'
      clientError.value = `Too long for a note (max ${NOTE_MAX_CHARS} characters). Use article instead.`
      return
    }
    noteContent.value = merged
    articleTitle.value = ''
    articleBody.value = ''
  }
})

function applyParsedFile(parsed: { kind: 'note' | 'article'; content: string; filename: string }) {
  convertNotice.value = ''
  clientError.value = ''
  kind.value = parsed.kind
  if (parsed.kind === 'article') {
    const split = splitArticleContent(parsed.content)
    articleTitle.value = split.title
    articleBody.value = split.body
    noteContent.value = ''
  } else {
    noteContent.value = parsed.content
    articleTitle.value = ''
    articleBody.value = ''
  }
  openNotice.value = `Loaded ${parsed.filename}`
}

async function onOpenFile(ev: Event) {
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  clientError.value = ''
  openNotice.value = ''
  try {
    const parsed = await readMarkdownFile(file)
    applyParsedFile(parsed)
  } catch (e) {
    clientError.value = e instanceof Error ? e.message : 'Could not open file'
  }
}

function openFilePicker() {
  fileInput.value?.click()
}

function submit() {
  clientError.value = ''
  const content = composedContent.value.trim()
  if (!content) {
    clientError.value = 'Content is required'
    return
  }
  if (kind.value === 'article' && !articleTitle.value.trim()) {
    clientError.value = 'Article title is required'
    return
  }
  if (kind.value === 'note' && shouldAutoConvertToArticle(content)) {
    promoteToArticle(content)
    clientError.value = `Too long for a note — switched to article (max ${NOTE_MAX_CHARS} characters).`
    return
  }

  const payload = { kind: kind.value, content, federate: shareToFediverse.value }
  if (isEdit()) {
    emit('save', { kind: kind.value, content })
  } else {
    emit('publish', payload)
    noteContent.value = ''
    articleTitle.value = ''
    articleBody.value = ''
    kind.value = 'note'
    openNotice.value = ''
  }
}
</script>

<template>
  <form class="compose" @submit.prevent="submit">
    <div class="kind-row">
      <div class="kind-toggle" role="group" aria-label="Kind">
        <button
          type="button"
          class="pill"
          :class="{ active: kind === 'note' }"
          :aria-pressed="kind === 'note'"
          @click="kind = 'note'"
        >
          Note
        </button>
        <button
          type="button"
          class="pill"
          :class="{ active: kind === 'article' }"
          :aria-pressed="kind === 'article'"
          @click="kind = 'article'"
        >
          Article
        </button>
      </div>
      <ComposeInfoTooltip :content="federationHint(kind)" />
    </div>

    <p v-if="convertNotice" class="notice">{{ convertNotice }}</p>
    <p v-if="draftNotice" class="notice draft-notice">{{ draftNotice }}</p>
    <p v-if="openNotice" class="notice open-notice">
      <span>{{ openNotice }}</span>
      <button type="button" class="notice-dismiss" aria-label="Dismiss" @click="openNotice = ''">
        <X :size="14" :stroke-width="1.75" aria-hidden="true" />
      </button>
    </p>

    <template v-if="kind === 'note'">
      <label class="editor-field">
        Note
        <MarkdownEditor
          v-model="noteContent"
          profile="note"
          :placeholder="notePlaceholder"
          :show-hint="false"
        />
      </label>
    </template>

    <template v-else>
      <label class="title-field">
        Title
        <input
          v-model="articleTitle"
          type="text"
          class="article-title-input"
          placeholder="Article headline"
          autocomplete="off"
        />
      </label>
      <label class="editor-field">
        Body
        <MarkdownEditor
          v-model="articleBody"
          profile="article"
          :placeholder="articleBodyPlaceholder"
          :show-hint="false"
        />
      </label>
    </template>

    <div v-if="props.statusText || kind === 'note'" class="status-row">
      <p class="status-line" :class="props.statusVariant">{{ props.statusText }}</p>
      <p v-if="kind === 'note'" class="counter" :class="noteCounterClass">{{ noteLengthLabel(noteCount) }}</p>
    </div>

    <p v-if="clientError" class="error">{{ clientError }}</p>
    <div class="form-actions">
      <input
        ref="fileInput"
        type="file"
        accept=".md,text/markdown"
        class="sr-only"
        aria-hidden="true"
        tabindex="-1"
        @change="onOpenFile"
      />
      <div class="form-actions-start">
        <button
          type="button"
          class="btn btn-ghost"
          title="Open markdown from a file"
          aria-label="Open markdown from a file"
          @click="openFilePicker"
        >
          <FileUp :size="16" :stroke-width="1.75" aria-hidden="true" />
          <span>Open from file</span>
        </button>
        <button
          v-if="isEdit()"
          type="button"
          class="btn btn-danger"
          title="Delete"
          aria-label="Delete"
          @click="emit('delete')"
        >
          <Trash2 :size="16" :stroke-width="1.75" aria-hidden="true" />
          <span>Delete</span>
        </button>
      </div>
      <div class="form-actions-end">
        <button
          v-if="!isEdit() && defaultLoaded"
          ref="federateTrack"
          type="button"
          class="federate-toggle"
          :class="{ active: shareToFediverse }"
          :aria-pressed="shareToFediverse"
          title="Share to fediverse"
          aria-label="Share to fediverse"
          @click="shareToFediverse = !shareToFediverse"
        >
          <span
            ref="federateThumb"
            class="federate-thumb"
            :style="{ transform: shareToFediverse ? `translateX(${federateThumbTravel}px)` : 'translateX(0)' }"
          >
            <MastodonIcon :size="14" />
            <span>Fediverse</span>
          </span>
        </button>
        <button
          type="submit"
          class="btn btn-primary"
          :title="isEdit() ? 'Save' : 'Publish post'"
          :aria-label="isEdit() ? 'Save' : 'Publish post'"
        >
          <Save v-if="isEdit()" :size="16" :stroke-width="1.75" aria-hidden="true" />
          <Send v-else :size="16" :stroke-width="1.75" aria-hidden="true" />
          <span>{{ isEdit() ? 'Save' : 'Publish' }}</span>
        </button>
        <button
          v-if="isEdit()"
          type="button"
          class="btn btn-ghost"
          title="Cancel editing"
          aria-label="Cancel editing"
          @click="emit('cancel')"
        >
          <X :size="16" :stroke-width="1.75" aria-hidden="true" />
          <span>Cancel</span>
        </button>
      </div>
    </div>
  </form>
</template>

<style scoped>
.compose {
  display: grid;
  gap: 0.75rem;
  margin-bottom: 1.5rem;
}
label {
  display: grid;
  gap: 0.35rem;
}
.kind-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
/* The track is the whole button (tap anywhere to toggle); the thumb is
   the icon+label pill itself, which slides the full measured width of
   the track's empty space to sit flush against the opposite edge, and
   flips color to mark on/off — no separate switch bezel. */
.federate-toggle {
  display: flex;
  align-items: center;
  /* form-actions-end centers children by default; stretch just this
     one so its height matches Publish (the tallest sibling) exactly,
     instead of guessing padding/line-height numbers to hit the same
     px value. */
  align-self: stretch;
  width: 9rem;
  padding: 0.2rem;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: transparent;
  cursor: pointer;
  transition: border-color 0.18s ease, background 0.18s ease;
}
.federate-toggle:hover {
  border-color: var(--muted);
}
.federate-toggle.active {
  border-color: color-mix(in srgb, var(--accent) 35%, var(--border));
  background: color-mix(in srgb, var(--accent) 8%, transparent);
}
.federate-thumb {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.35rem 0.7rem;
  border-radius: 999px;
  background: color-mix(in srgb, var(--muted) 16%, transparent);
  color: var(--muted);
  font-size: 0.8rem;
  font-weight: 600;
  white-space: nowrap;
  transition: transform 0.18s ease, background 0.18s ease, color 0.18s ease;
}
.federate-toggle.active .federate-thumb {
  background: var(--accent);
  color: #fff;
}
.federate-thumb :deep(svg) {
  flex-shrink: 0;
}
.notice {
  margin: 0;
  padding: var(--space-3) var(--space-4);
  border-radius: var(--radius-md);
  background: var(--notice-bg);
  border: 1px solid var(--notice-border);
  color: var(--notice-text);
  font-size: var(--text-sm);
}
.open-notice {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
}
.notice-dismiss {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  padding: 0;
  border: none;
  background: none;
  color: inherit;
  opacity: 0.7;
  cursor: pointer;
}
.notice-dismiss:hover {
  opacity: 1;
}
.title-field {
  margin-top: 0.25rem;
}
.article-title-input {
  font: inherit;
  font-family: var(--font-serif);
  font-size: 1.45rem;
  font-weight: 600;
  line-height: 1.25;
  padding: 0.65rem 0.85rem;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface);
  color: var(--text);
}
.article-title-input:focus {
  outline: 2px solid color-mix(in srgb, var(--accent) 35%, transparent);
  border-color: var(--accent);
}
.status-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-3);
  margin: -0.35rem 0 0;
}
.status-line {
  margin: 0;
  font-size: 0.82rem;
  color: var(--muted);
  text-align: left;
}
.status-line.error {
  color: var(--danger);
}
.counter {
  margin: 0;
  font-size: 0.82rem;
  color: var(--muted);
  text-align: right;
  white-space: nowrap;
}
.counter.warn {
  color: var(--warn);
}
.counter.over {
  color: var(--danger);
}
.kind-toggle {
  display: flex;
  flex: 1;
  gap: 0.15rem;
  padding: 0.2rem;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: var(--surface);
}
.kind-toggle .pill {
  flex: 1;
  padding: 0.3rem 0.9rem;
  border: none;
  border-radius: 999px;
  background: none;
  font: inherit;
  font-size: 0.85rem;
  color: var(--muted);
  cursor: pointer;
  text-align: center;
}
.kind-toggle .pill.active {
  background: var(--accent);
  color: var(--surface);
  font-weight: 600;
}
.open-notice {
  color: var(--accent);
  background: color-mix(in srgb, var(--accent) 8%, transparent);
  border-color: color-mix(in srgb, var(--accent) 25%, transparent);
}
.draft-notice {
  color: var(--muted);
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}
.error {
  color: var(--danger);
}

@media (max-width: 47.99rem) {
  /* .form-actions-end's margin-left:auto (primitives.css) is meant to
     push it flush right on a shared row with .form-actions-start. Once
     the row wraps to two lines on mobile, that same rule flushes the
     whole group right on its own line, leaving a lopsided gap next to
     the full-width start group above it. Stack both groups full-width
     instead so Save/Cancel match the Open-from-file/Delete row. */
  .form-actions {
    flex-direction: column;
    align-items: stretch;
  }
  .form-actions-start,
  .form-actions-end {
    width: 100%;
    margin-left: 0;
  }
  .form-actions-start .btn,
  .form-actions-end .btn,
  .form-actions-end .federate-toggle {
    flex: 1;
  }
}
</style>
