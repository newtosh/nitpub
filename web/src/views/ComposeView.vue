<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import ComposeForm from '../components/ComposeForm.vue'
import RecentPostsSheet from '../components/RecentPostsSheet.vue'
import { splitArticleContent } from '../lib/contentKinds'
import { deletePost, postSlug, publishDraft, relativeTime, saveDraft } from '../lib/posts'

const router = useRouter()
const error = ref('')

// Minted once per compose session and reused for every autosave, including
// the first — SaveDraft is an upsert keyed on this slug (see
// internal/outbox.SaveDraft's doc comment), so a lost response (e.g. the
// server process restarting mid-request) doesn't leave the client with no
// ID to retry against. Without this, an empty-slug "first save" that the
// client never got a response for would look identical, next attempt, to
// never having tried at all — producing a second, separate draft row for
// the same in-progress note instead of updating the first.
const clientDraftId = crypto.randomUUID()
// Set only once a save against clientDraftId has actually been confirmed
// by the server — this is "does a draft row exist yet," distinct from
// clientDraftId, which exists from the start regardless.
const draftSlug = ref<string | undefined>(undefined)
const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const savedAt = ref<string | undefined>(undefined)
// A single chained promise, not an overwritable slot — every draftChange
// call awaits the prior one before issuing its own SaveDraft, so two
// concurrent autosaves (e.g. a slow request outlasting the 800ms debounce)
// are strictly ordered against the same clientDraftId instead of racing.
let pendingSave: Promise<void> = Promise.resolve()

const statusText = computed(() => {
  if (saveState.value === 'saving') return 'Saving…'
  if (saveState.value === 'saved') return `Saved ${savedAt.value ? relativeTime(savedAt.value) : ''}`.trim()
  if (saveState.value === 'error') return 'Autosave failed — will retry on next edit'
  return ''
})

function draftChange(payload: { kind: 'note' | 'article'; title: string; content: string }) {
  pendingSave = pendingSave.then(async () => {
    saveState.value = 'saving'
    try {
      const post = await saveDraft({
        kind: payload.kind,
        title: payload.title,
        content: payload.content,
        slug: clientDraftId,
      })
      draftSlug.value = postSlug(post.id)
      savedAt.value = post.updated_at ?? post.created_at
      saveState.value = 'saved'
    } catch {
      // Keep the in-progress compose state intact — a failed autosave must
      // never clear or corrupt what the author has typed.
      saveState.value = 'error'
    }
  })
  return pendingSave
}

async function publish(payload: { kind: string; content: string; federate: boolean }) {
  error.value = ''
  // Wait for every autosave already queued before deciding which path to
  // take — and before publishing a draft, so publish never races an
  // in-flight autosave.
  await pendingSave

  try {
    // Quote posts have no representation in the draft-publish payload
    // (source_url/excerpt/etc. have nowhere to go in {kind,title,content}),
    // and the server rejects a draft-shaped save with kind=quote outright.
    // If the author typed a note/article first (autosaving draftSlug),
    // then switched to Quote, that stray draft is disposable — publish the
    // quote directly and clean it up, rather than routing through it.
    if (draftSlug.value && payload.kind !== 'quote') {
      // Publish exactly what's on screen, not whatever the last autosave
      // happened to persist — the author may have kept typing since then.
      const { title, body } =
        payload.kind === 'article'
          ? splitArticleContent(payload.content)
          : { title: '', body: payload.content }
      await publishDraft(draftSlug.value, {
        kind: payload.kind,
        title,
        content: body,
        federate: payload.federate,
      })
      router.push('/author')
      return
    }
    const staleDraftSlug = draftSlug.value
    const res = await fetch('/api/posts', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
    if (!res.ok) {
      const text = await res.text()
      error.value = text || 'Publish failed'
      return
    }
    if (staleDraftSlug) {
      // Best-effort — an orphaned draft is clutter, not a correctness
      // problem, so a delete failure here must never block the publish
      // that already succeeded.
      deletePost(staleDraftSlug).catch(() => {})
    }
    router.push('/author')
  } catch {
    error.value = 'Publish failed — check your connection and try again'
  }
}
</script>

<template>
  <div class="compose-page">
    <header class="page-header">
      <h1>Compose</h1>
      <p class="text-muted">Compose notes and articles for this instance.</p>
    </header>

    <ComposeForm
      :status-text="statusText"
      :status-variant="saveState"
      @publish="publish"
      @draft-change="draftChange"
    />

    <p v-if="error" class="alert alert-error">{{ error }}</p>

    <RecentPostsSheet />
  </div>
</template>

<style scoped>
.page-header h1 {
  margin: 0;
}

@media (max-width: 47.99rem) {
  .page-header {
    display: none;
  }
  .compose-page {
    display: flex;
    flex-direction: column;
    /* svh (not dvh) so the layout doesn't reflow/jump when the iOS keyboard
       opens — dvh recalculates on every keyboard transition, which is what
       causes the visible jump/clip. svh stays fixed at the viewport size
       with the browser chrome shown; the keyboard just overlaps it. */
    min-height: 100svh;
    /* Reserve room for RecentPostsSheet's fixed-to-viewport-bottom
       handle so it doesn't sit on top of the Publish row. */
    padding-bottom: calc(2.75rem + env(safe-area-inset-bottom));
  }
  .compose-page :deep(.compose) {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .compose-page :deep(.editor-field) {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .compose-page :deep(.editor-field .md-editor) {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .compose-page :deep(.editor-field .md-editor textarea) {
    flex: 1;
    min-height: 0;
  }
}
</style>
