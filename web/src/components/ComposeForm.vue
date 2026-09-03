<script setup lang="ts">
import { FileUp, Save, Send, Trash2, X } from '@lucide/vue'
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import MarkdownEditor from './MarkdownEditor.vue'
import MarkdownBody from './MarkdownBody.vue'
import EditorTabs from './EditorTabs.vue'
import ComposeInfoTooltip from './ComposeInfoTooltip.vue'
import InfoTip from './InfoTip.vue'
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
import { fetchSiteConfig, getCachedSiteConfig } from '../lib/site'
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
  // True when editing an existing post whose status is 'draft' (set only
  // by EditPostView — ComposeView's own stray-autosave case routes around
  // the problem instead, see its publish()). Quote posts have no
  // representation in the draft system (SaveDraft rejects kind=quote
  // outright — see internal/outbox), and there's currently no path to
  // convert an existing draft's kind to quote and publish/save it (both
  // SaveDraft and updatePost reject a draft-status post one way or the
  // other), so the Quote pill is hidden here rather than letting the user
  // reach a dead end.
  existingDraft?: boolean
}>()

// Quote posts (R2) have no composed-markdown "content" field on the wire —
// the structured fields below travel alongside kind:"quote" instead (U3's
// createPostRequest/updatePostRequest). Optional here since note/article
// payloads never set them.
type QuotePayloadFields = {
  source_url?: string
  title?: string
  excerpt?: string
  commentary?: string
  via?: string
}

const emit = defineEmits<{
  publish: [payload: { kind: string; content: string; federate: boolean } & QuotePayloadFields]
  save: [payload: { kind: string; content: string } & QuotePayloadFields]
  cancel: []
  delete: []
  'draft-change': [payload: { kind: 'note' | 'article'; title: string; content: string }]
}>()

const isEdit = () => !!props.post

const kind = ref<'note' | 'article' | 'quote'>('note')
const noteContent = ref('')
const articleTitle = ref('')
const articleBody = ref('')
const articleTab = ref<'write' | 'preview'>('write')
const clientError = ref('')

// Quote-post compose state (R2) — bypasses the draft-autosave system
// entirely (U1 decision: the composer's required-field validation would
// block every autosave tick), so there's no equivalent of noteContent's
// server-draft wiring here.
const quotePostsEnabled = ref(false)
const quoteSourceUrl = ref('')
const quoteExcerpt = ref('')
const quoteCommentary = ref('')
const quoteVia = ref('')
// Prefilled from /api/unfurl's title (R3); sent to the API as `title` and
// used as the published link text by BuildQuoteContent (internal/outbox),
// falling back to the source URL's hostname when blank.
const quoteLinkTitle = ref('')
const quoteTitleFetching = ref(false)
const quoteTab = ref<'write' | 'preview'>('write')
const baselineQuote = ref<{
  sourceUrl: string
  title: string
  excerpt: string
  commentary: string
  via: string
} | null>(null)
let quoteFetchToken = 0
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

// Mirrors BuildQuoteContent (internal/outbox) exactly — link, blockquoted
// excerpt, optional commentary, optional via — so the preview shows what
// will actually publish. Kept in sync manually; there is no shared
// implementation between Go and this client-side stitch.
// Mirrors escapeMarkdownLinkText (internal/outbox) — a "]" or "\" in the
// link text (e.g. from a fetched page's <title>) could otherwise terminate
// the [...] span early and redirect the preview's rendered link away from
// sourceUrl.
function escapeMarkdownLinkText(text: string): string {
  return text.replaceAll('\\', '\\\\').replaceAll('[', '\\[').replaceAll(']', '\\]')
}

// Mirrors quoteBlockquote (internal/outbox) — every line of the excerpt,
// including blank separator lines, needs its own "> " so a multi-paragraph
// excerpt stays inside the blockquote instead of CommonMark ending it at
// the first unprefixed blank line.
function quoteBlockquote(excerpt: string): string {
  return excerpt
    .split('\n')
    .map((line) => (line === '' ? '>' : `> ${line}`))
    .join('\n')
}

const quoteStitchedContent = computed(() => {
  const sourceUrl = quoteSourceUrl.value.trim()
  const excerpt = quoteExcerpt.value.trim()
  if (!sourceUrl || !excerpt) return ''
  let linkText = quoteLinkTitle.value.trim()
  if (!linkText) {
    linkText = sourceUrl
    try {
      const host = new URL(sourceUrl).host
      if (host) linkText = host
    } catch {
      // Not a parseable URL yet — fall back to the raw string, same as
      // BuildQuoteContent does when url.Parse fails.
    }
  }
  let out = `[${escapeMarkdownLinkText(linkText)}](${sourceUrl})\n\n${quoteBlockquote(excerpt)}`
  const commentary = quoteCommentary.value.trim()
  if (commentary) out += `\n\n${commentary}`
  const via = quoteVia.value.trim()
  if (via) out += `\n\n(via ${via})`
  return out
})

// Callers only ever invoke this when kind.value is 'note'/'article' (every
// call site is guarded), but kind's declared type is wider ('quote' too),
// so an explicit narrow here keeps the return type honest instead of
// relying on caller discipline.
function currentPayload(): { kind: 'note' | 'article'; content: string } {
  return { kind: kind.value === 'article' ? 'article' : 'note', content: composedContent.value }
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
  if (!isEdit()) return false
  if (baselineQuote.value) {
    return (
      kind.value !== 'quote' ||
      quoteSourceUrl.value.trim() !== baselineQuote.value.sourceUrl ||
      quoteLinkTitle.value.trim() !== baselineQuote.value.title ||
      quoteExcerpt.value.trim() !== baselineQuote.value.excerpt ||
      quoteCommentary.value.trim() !== baselineQuote.value.commentary ||
      quoteVia.value.trim() !== baselineQuote.value.via
    )
  }
  if (!baseline.value) return false
  // Converting an existing note/article to quote is always a real change —
  // currentPayload()/payloadsEqual() only compare note/article shapes, so
  // without this the conversion would read as clean and navigating away
  // could silently discard it with no unsaved-changes prompt.
  if (kind.value === 'quote') return true
  return !payloadsEqual(currentPayload(), baseline.value)
})

function applyContent(kindVal: 'note' | 'article', content: string) {
  kind.value = kindVal
  if (kindVal === 'article') {
    const split = splitArticleContent(content)
    articleTitle.value = split.title
    articleBody.value = split.body
    articleTab.value = 'write'
    noteContent.value = ''
  } else {
    noteContent.value = content
    articleTitle.value = ''
    articleBody.value = ''
  }
}

function resetQuoteFields() {
  quoteSourceUrl.value = ''
  quoteExcerpt.value = ''
  quoteCommentary.value = ''
  quoteVia.value = ''
  quoteLinkTitle.value = ''
  quoteTitleFetching.value = false
  quoteTab.value = 'write'
  quoteFetchToken++
}

function applyPost(post: Post | null | undefined) {
  convertNotice.value = ''
  clientError.value = ''
  draftNotice.value = ''
  baselineQuote.value = null
  if (!post) {
    baseline.value = null
    kind.value = 'note'
    noteContent.value = ''
    articleTitle.value = ''
    articleBody.value = ''
    resetQuoteFields()
    return
  }

  // Quote posts bypass the draft-autosave/local-restore path entirely (U1) —
  // load its structured fields straight from the server post.
  if (post.kind === 'quote') {
    baseline.value = null
    kind.value = 'quote'
    quoteTab.value = 'write'
    noteContent.value = ''
    articleTitle.value = ''
    articleBody.value = ''
    quoteSourceUrl.value = post.quote?.source_url ?? ''
    quoteExcerpt.value = post.quote?.excerpt ?? ''
    quoteCommentary.value = post.quote?.commentary ?? ''
    quoteVia.value = post.quote?.via ?? ''
    quoteLinkTitle.value = post.quote?.title ?? ''
    baselineQuote.value = {
      sourceUrl: quoteSourceUrl.value,
      title: quoteLinkTitle.value,
      excerpt: quoteExcerpt.value,
      commentary: quoteCommentary.value,
      via: quoteVia.value,
    }
    return
  }
  resetQuoteFields()

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
  // Quote posts bypass draft-autosave entirely (U1 KTD2/step 7): the
  // composer's required-field validation would reject every autosave tick
  // while an author is still mid-way through filling the form.
  if (kind.value === 'quote') return
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
  if (kind.value === 'quote') {
    baselineQuote.value = {
      sourceUrl: quoteSourceUrl.value.trim(),
      title: quoteLinkTitle.value.trim(),
      excerpt: quoteExcerpt.value.trim(),
      commentary: quoteCommentary.value.trim(),
      via: quoteVia.value.trim(),
    }
  } else {
    baseline.value = { ...currentPayload() }
  }
  draftNotice.value = ''
}

function clearDraft() {
  if (!props.post) return
  clearComposeDraft(postSlug(props.post.id))
}

function discardChanges() {
  if (!props.post) return
  if (baselineQuote.value) {
    quoteSourceUrl.value = baselineQuote.value.sourceUrl
    quoteLinkTitle.value = baselineQuote.value.title
    quoteExcerpt.value = baselineQuote.value.excerpt
    quoteCommentary.value = baselineQuote.value.commentary
    quoteVia.value = baselineQuote.value.via
    clientError.value = ''
    return
  }
  if (!baseline.value) return
  applyContent(baseline.value.kind, baseline.value.content)
  clearDraft()
  draftNotice.value = ''
  convertNotice.value = ''
  clientError.value = ''
}

// Fetch-and-handle-failure shape follows linkCard.ts's fetchLinkPreview:
// any non-2xx status, network error, or timeout is swallowed and leaves
// the title field empty and editable — auto-fetch must never block
// publishing (R8, AE3). quoteFetchToken guards against a response that
// resolves after the author has already moved on (edited the field
// themselves, or submitted the form) from overwriting anything.
async function onSourceUrlBlur() {
  const raw = quoteSourceUrl.value.trim()
  if (!raw || quoteLinkTitle.value.trim()) return
  try {
    const parsed = new URL(raw)
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return
  } catch {
    return
  }

  const token = ++quoteFetchToken
  quoteTitleFetching.value = true
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 10000)
  try {
    const res = await fetch(`/api/unfurl?url=${encodeURIComponent(raw)}`, { signal: controller.signal })
    if (token !== quoteFetchToken || !res.ok) return
    const data = (await res.json()) as { title?: string }
    // The Source URL field stays editable while this request is in flight
    // (only quoteFetchToken guards the response, and that only changes on
    // the *next* blur) — if the author changes the URL before this
    // response resolves, quoteSourceUrl no longer matches raw, and this
    // fetched title belongs to a page that's no longer what's in the field.
    if (
      token === quoteFetchToken &&
      quoteSourceUrl.value.trim() === raw &&
      data.title &&
      !quoteLinkTitle.value.trim()
    ) {
      quoteLinkTitle.value = data.title
    }
  } catch {
    // Network error, abort/timeout — leave the field empty and editable.
  } finally {
    clearTimeout(timeout)
    if (token === quoteFetchToken) quoteTitleFetching.value = false
  }
}

// Same narrowing note as currentPayload — the draft-publish path this
// feeds (EditPostView's dedicated Publish button for a draft-status post)
// only has a note/article shape to send; the Quote pill is hidden whenever
// existingDraft is true (see the template), so kind.value is never
// actually 'quote' here at runtime either.
function getDraftPayload(): { kind: 'note' | 'article'; title: string; content: string } {
  return {
    kind: kind.value === 'article' ? 'article' : 'note',
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
  // Seed synchronously from whatever's already cached so the Quote pill
  // doesn't flicker in, then always revalidate below — /author/edit/:slug
  // (EditPostView) is a top-level route, not a child of AdminShellView, so
  // a direct or first visit to an edit URL has no earlier /api/site fetch
  // to have populated this cache.
  quotePostsEnabled.value = !!getCachedSiteConfig()?.quote_posts_enabled
  try {
    const site = await fetchSiteConfig()
    quotePostsEnabled.value = !!site.quote_posts_enabled
    if (!isEdit()) shareToFediverse.value = crossPostDefaultFromConfig(site.federation)
  } catch {
    if (!isEdit()) shareToFediverse.value = true
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
    articleTab.value = 'write'
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

  if (kind.value === 'quote') {
    // Invalidate any unfurl request still in flight — its response, if it
    // arrives after this point, must never overwrite a title on a post
    // that's already been submitted.
    quoteFetchToken++
    const sourceUrl = quoteSourceUrl.value.trim()
    const excerpt = quoteExcerpt.value.trim()
    if (!sourceUrl) {
      clientError.value = 'Source URL is required'
      return
    }
    if (!excerpt) {
      clientError.value = 'Excerpt is required'
      return
    }
    const quoteFields = {
      source_url: sourceUrl,
      title: quoteLinkTitle.value.trim(),
      excerpt,
      commentary: quoteCommentary.value.trim(),
      via: quoteVia.value.trim(),
    }
    if (isEdit()) {
      emit('save', { kind: 'quote', content: '', ...quoteFields })
    } else {
      emit('publish', { kind: 'quote', content: '', federate: shareToFediverse.value, ...quoteFields })
      resetQuoteFields()
      kind.value = 'note'
      openNotice.value = ''
    }
    return
  }

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
    articleTab.value = 'write'
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
        <button
          v-if="(quotePostsEnabled && !existingDraft) || kind === 'quote'"
          type="button"
          class="pill"
          :class="{ active: kind === 'quote' }"
          :aria-pressed="kind === 'quote'"
          @click="kind = 'quote'"
        >
          Quote
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
      <label class="editor-field" for="note-content">
        Note
        <MarkdownEditor
          id="note-content"
          v-model="noteContent"
          profile="note"
          :placeholder="notePlaceholder"
          :show-hint="false"
        />
      </label>
    </template>

    <template v-else-if="kind === 'article'">
      <div class="md-editor article-editor">
        <EditorTabs v-model="articleTab" />
        <div v-show="articleTab === 'write'" class="stacked-editor-fields">
          <label class="quote-field">
            Title
            <input
              v-model="articleTitle"
              type="text"
              class="article-title-input"
              placeholder="Article headline"
              autocomplete="off"
            />
          </label>
          <label class="quote-field" for="article-body">
            Body
            <MarkdownEditor
              id="article-body"
              v-model="articleBody"
              profile="article"
              :placeholder="articleBodyPlaceholder"
              :show-hint="false"
              :show-preview-tab="false"
            />
          </label>
        </div>
        <div v-if="articleTab === 'preview'" class="md-editor-preview">
          <template v-if="articleTitle.trim() || articleBody.trim()">
            <h1 v-if="articleTitle.trim()">{{ articleTitle }}</h1>
            <MarkdownBody :content="articleBody" />
          </template>
          <p v-else class="status">Add a title or body to preview the article.</p>
        </div>
      </div>
    </template>

    <template v-else>
      <div class="md-editor quote-editor">
        <EditorTabs v-model="quoteTab" />
        <div v-show="quoteTab === 'write'" class="stacked-editor-fields">
          <label class="quote-field">
            Source URL
            <input
              v-model="quoteSourceUrl"
              type="url"
              class="quote-input"
              placeholder="https://example.com/article"
              autocomplete="off"
              required
              @blur="onSourceUrlBlur"
            />
          </label>
          <label class="quote-field" for="quote-link-title">
            <span class="quote-field-label">Link title <InfoTip label="The published link text — leave blank to use the source's domain name." /></span>
            <input
              id="quote-link-title"
              v-model="quoteLinkTitle"
              type="text"
              class="quote-input"
              :disabled="quoteTitleFetching"
              :placeholder="quoteTitleFetching ? 'Fetching title…' : 'Auto-fetched from the source URL'"
              autocomplete="off"
            />
          </label>
          <label class="quote-field" for="quote-excerpt">
            Excerpt
            <MarkdownEditor
              id="quote-excerpt"
              v-model="quoteExcerpt"
              profile="quote"
              :rows="4"
              placeholder="The quoted text from the source"
              :show-hint="false"
              :show-preview-tab="false"
            />
          </label>
          <label class="quote-field" for="quote-commentary">
            <span class="quote-field-label">Commentary <InfoTip label="Optional — your own take on the quote." /></span>
            <MarkdownEditor
              id="quote-commentary"
              v-model="quoteCommentary"
              profile="quote"
              :rows="4"
              placeholder="Your take"
              :show-hint="false"
              :show-preview-tab="false"
            />
          </label>
          <label class="quote-field" for="quote-via">
            <span class="quote-field-label">Via <InfoTip label="Optional — who pointed you to this, for a hat-tip." /></span>
            <input
              id="quote-via"
              v-model="quoteVia"
              type="text"
              class="quote-input"
              placeholder="Who pointed you to this"
              autocomplete="off"
            />
          </label>
        </div>
        <div v-if="quoteTab === 'preview'" class="md-editor-preview">
          <template v-if="quoteStitchedContent">
            <MarkdownBody :content="quoteStitchedContent" :inline-link-cards="true" />
          </template>
          <p v-else class="status">Add a source URL and excerpt to preview the full post.</p>
        </div>
      </div>
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
.quote-field-label {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
}
.stacked-editor-fields {
  display: grid;
  gap: 0.75rem;
  padding: 1rem;
}
.quote-input {
  font: inherit;
  /* Matches MarkdownEditor's textarea: iOS Safari auto-zooms on focus
     when an input's font-size is under 16px. */
  font-size: 1rem;
  padding: 0.55rem 0.75rem;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface);
  color: var(--text);
}
.quote-input:disabled {
  opacity: 0.65;
  cursor: wait;
}
.quote-input:focus {
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
