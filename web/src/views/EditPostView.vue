<script setup lang="ts">
import { ArrowLeft } from '@lucide/vue'
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { onBeforeRouteLeave, useRouter } from 'vue-router'
import ComposeForm from '../components/ComposeForm.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import PostDeliveryBadges from '../components/PostDeliveryBadges.vue'
import { useSession } from '../composables/useSession'
import { combineArticleContent, splitArticleContent } from '../lib/contentKinds'
import {
  deletePost,
  fetchPost,
  postDisplayTitle,
  postSlug,
  publishDraft,
  relativeTime,
  saveDraft,
  updatePost,
  type Post,
} from '../lib/posts'

const props = defineProps<{ slug: string }>()

const router = useRouter()
const { refresh } = useSession()
const post = ref<Post | null>(null)
const error = ref('')
const loading = ref(true)
const deleteOpen = ref(false)
const deleteBusy = ref(false)
const leaveOpen = ref(false)
const composeRef = ref<InstanceType<typeof ComposeForm> | null>(null)
const publishBusy = ref(false)
const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')

let leaveAction: (() => void) | null = null
let leaveConfirming = false
let allowLeave = false

const deleteMessage = computed(() => {
  if (!post.value) return ''
  return `“${postDisplayTitle(post.value)}” will be removed from your site. This cannot be undone.`
})

const statusText = computed(() => {
  if (!post.value) return ''
  const lastSaved = post.value.updated_at ?? post.value.created_at
  if (post.value.status === 'draft') {
    if (saveState.value === 'saving') return 'Saving…'
    if (saveState.value === 'error') return 'Draft — autosave failed, will retry on next edit'
    return `Draft — saved ${relativeTime(lastSaved)}`
  }
  return `Published ${relativeTime(lastSaved)}`
})

// A draft article stores Title and Content separately (KTD7) — Content is
// body-only, not title-embedded the way a published article's is. Combine
// them before handing the post to ComposeForm, which always expects the
// canonical title-embedded shape for an article; otherwise it treats the
// body's first line as the title and corrupts both on the next save.
const composeFormPost = computed<Post | null>(() => {
  if (!post.value) return null
  if (post.value.status === 'draft' && post.value.kind === 'article') {
    return { ...post.value, content: combineArticleContent(post.value.title ?? '', post.value.content) }
  }
  return post.value
})

function isDirty(): boolean {
  return composeRef.value?.getDirty() ?? false
}

function attemptLeave(action: () => void) {
  if (isDirty()) {
    leaveAction = action
    leaveOpen.value = true
    return
  }
  action()
}

function confirmDiscardLeave() {
  leaveConfirming = true
  composeRef.value?.discardChanges()
  const action = leaveAction
  leaveAction = null
  leaveOpen.value = false
  allowLeave = true
  action?.()
  void nextTick(() => {
    allowLeave = false
  })
  leaveConfirming = false
}

function onLeaveModalOpen(open: boolean) {
  leaveOpen.value = open
  if (!open && !leaveConfirming) {
    leaveAction = null
  }
}

function onBeforeUnload(ev: BeforeUnloadEvent) {
  if (isDirty()) {
    ev.preventDefault()
    ev.returnValue = ''
  }
}

onBeforeRouteLeave((_to, _from, next) => {
  if (allowLeave || !isDirty()) {
    next()
    return
  }
  leaveAction = () => next()
  leaveOpen.value = true
  next(false)
})

onMounted(async () => {
  window.addEventListener('beforeunload', onBeforeUnload)
  await refresh()
  try {
    post.value = await fetchPost(props.slug)
  } catch (e) {
    error.value =
      e instanceof Error && e.message === 'not-found'
        ? 'Post not found.'
        : 'Could not load post.'
  } finally {
    loading.value = false
  }
})

onUnmounted(() => {
  window.removeEventListener('beforeunload', onBeforeUnload)
})

// Chained, not an overwritable slot — see the matching comment in
// ComposeView.vue.
let pendingSave: Promise<void> = Promise.resolve()

async function save(payload: { kind: string; content: string }) {
  error.value = ''
  try {
    await pendingSave
    if (post.value?.status === 'draft') {
      const { title, body } =
        payload.kind === 'article'
          ? splitArticleContent(payload.content)
          : { title: '', body: payload.content }
      post.value = await saveDraft({ kind: payload.kind, title, content: body, slug: props.slug })
    } else {
      await updatePost(props.slug, payload)
    }
    composeRef.value?.markSaved()
    router.push('/author')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Save failed'
  }
}

function draftChange(payload: { kind: 'note' | 'article'; title: string; content: string }) {
  pendingSave = pendingSave.then(async () => {
    saveState.value = 'saving'
    try {
      post.value = await saveDraft({
        kind: payload.kind,
        title: payload.title,
        content: payload.content,
        slug: props.slug,
      })
      saveState.value = 'saved'
      composeRef.value?.markSaved()
    } catch {
      saveState.value = 'error'
    }
  })
  return pendingSave
}

async function publish() {
  error.value = ''
  publishBusy.value = true
  try {
    // Wait for queued autosaves, then flush exactly what's on screen right
    // now — never republish a possibly-stale prior autosave.
    await pendingSave
    if (!composeRef.value) {
      throw new Error('Form not ready — try again in a moment')
    }
    const live = composeRef.value.getDraftPayload()
    await publishDraft(props.slug, live)
    router.push('/author')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Publish failed'
  } finally {
    publishBusy.value = false
  }
}

function cancel() {
  attemptLeave(() => router.push('/author'))
}

function goBack() {
  attemptLeave(() => router.push('/author'))
}

async function confirmDelete() {
  deleteBusy.value = true
  error.value = ''
  try {
    await deletePost(props.slug)
    composeRef.value?.clearDraft()
    deleteOpen.value = false
    router.push('/author')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Delete failed'
  } finally {
    deleteBusy.value = false
  }
}
</script>

<template>
  <p class="back">
    <button type="button" class="text-link" @click="goBack">
      <ArrowLeft :size="16" :stroke-width="1.75" aria-hidden="true" />
      <span>Author</span>
    </button>
  </p>

  <header class="page-header">
    <h1>Edit post</h1>
  </header>

  <p v-if="loading" class="status">Loading…</p>
  <p v-else-if="error && !post" class="error">{{ error }}</p>

  <template v-else-if="post">
    <div class="meta-row">
      <p v-if="post.status !== 'draft'" class="meta">
        Permalink:
        <RouterLink :to="`/p/${postSlug(post.id)}`">/p/{{ postSlug(post.id) }}</RouterLink>
      </p>
      <PostDeliveryBadges :post="post" class="delivery" />
      <button
        v-if="post.status === 'draft'"
        type="button"
        class="btn btn-primary"
        :disabled="publishBusy"
        @click="publish"
      >
        {{ publishBusy ? 'Publishing…' : 'Publish' }}
      </button>
    </div>
    <ComposeForm
      ref="composeRef"
      :post="composeFormPost"
      :status-text="statusText"
      :status-variant="saveState"
      @save="save"
      @cancel="cancel"
      @delete="deleteOpen = true"
      @draft-change="draftChange"
    />
    <p v-if="error" class="error">{{ error }}</p>
  </template>

  <ConfirmModal
    v-model:open="deleteOpen"
    title="Delete post?"
    :message="deleteMessage"
    confirm-label="Delete"
    :danger="true"
    :busy="deleteBusy"
    @confirm="confirmDelete"
  />

  <ConfirmModal
    :open="leaveOpen"
    title="Discard unsaved changes?"
    message="You have unsaved edits. Leaving now will discard them unless you save first."
    confirm-label="Discard"
    :danger="true"
    @update:open="onLeaveModalOpen"
    @confirm="confirmDiscardLeave"
  />
</template>

<style scoped>
@media (max-width: 47.99rem) {
  .page-header {
    display: none;
  }
}
.back {
  margin: 0 0 1.5rem;
  font-size: 0.9rem;
}
.text-link {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0;
  border: none;
  background: none;
  font: inherit;
  color: var(--muted);
  cursor: pointer;
}
.text-link:hover {
  color: var(--accent);
}
.page-header {
  margin-bottom: 1.5rem;
}
.page-header h1 {
  font-family: var(--font-serif);
  font-size: 2rem;
  margin: 0;
}
.meta-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.75rem 1.5rem;
  margin: 0 0 0.75rem;
}
.meta {
  margin: 0;
  font-size: 0.9rem;
  color: var(--muted);
}
.delivery {
  margin-left: auto;
}
.status {
  color: var(--muted);
}
.error {
  color: var(--danger);
}
</style>
