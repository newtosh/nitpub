<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import ComposeForm from '../components/ComposeForm.vue'
import RecentPostsSheet from '../components/RecentPostsSheet.vue'
import { splitArticleContent } from '../lib/contentKinds'
import { postSlug, publishDraft, relativeTime, saveDraft } from '../lib/posts'

const router = useRouter()
const error = ref('')

const draftSlug = ref<string | undefined>(undefined)
const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const savedAt = ref<string | undefined>(undefined)
// A single chained promise, not an overwritable slot — every draftChange
// call awaits the prior one before issuing its own SaveDraft, so two
// concurrent autosaves (e.g. a slow request outlasting the 800ms debounce)
// can never both see an empty slug and create two separate draft rows.
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
        slug: draftSlug.value,
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
    if (draftSlug.value) {
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
      :existing-draft="!!draftSlug"
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
