<script setup lang="ts">
import { Ban, Check, ChevronDown, History, Inbox, Plus, Trash2, Undo2 } from '@lucide/vue'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import SearchField from './SearchField.vue'
import AdminReplyBody from './AdminReplyBody.vue'
import { avatarsEnabledFromConfig } from '../lib/federationDelivery'
import { fetchPosts, postDisplayTitle, postSlug, type Post } from '../lib/posts'
import { fetchSiteConfig } from '../lib/site'
import {
  addBlockedActor,
  addTrustedActor,
  approveReply,
  fetchBlockedActors,
  fetchPendingReplies,
  fetchReviewedReplies,
  fetchTrustedActors,
  rejectReply,
  removeBlockedActor,
  removeTrustedActor,
  revertReply,
  skipReply,
  type PendingReply,
} from '../lib/moderationAdmin'
import { useModerationBadge } from '../composables/useModerationBadge'

const { setCount } = useModerationBadge()

const loading = ref(true)
const error = ref('')
const message = ref('')

const pending = ref<PendingReply[]>([])
const trusted = ref<string[]>([])
const blocked = ref<string[]>([])
const confirming = ref<string | null>(null)

const openMenu = ref<string | null>(null)

const replyView = ref<'pending' | 'reviewed'>('pending')
const reviewed = ref<PendingReply[]>([])
const reviewedLoaded = ref(false)
const reviewedLoading = ref(false)

const activeList = ref<'trusted' | 'blocked'>('trusted')
const actorFilter = ref('')
const newActor = ref('')
const showAvatars = ref(true)
const postsBySlug = ref<Map<string, Post>>(new Map())

function postTag(slug: string): { title: string; known: boolean } {
  const post = postsBySlug.value.get(slug)
  return post ? { title: postDisplayTitle(post), known: true } : { title: slug, known: false }
}

async function loadReviewed() {
  reviewedLoading.value = true
  try {
    reviewed.value = await fetchReviewedReplies()
    reviewedLoaded.value = true
  } catch {
    error.value = 'Could not load reviewed replies.'
  } finally {
    reviewedLoading.value = false
  }
}

function selectReplyView(view: 'pending' | 'reviewed') {
  replyView.value = view
  if (view === 'reviewed' && !reviewedLoaded.value) loadReviewed()
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [p, t, b] = await Promise.all([
      fetchPendingReplies(),
      fetchTrustedActors(),
      fetchBlockedActors(),
    ])
    pending.value = p
    trusted.value = t
    blocked.value = b
    setCount(p.length)
  } catch {
    error.value = 'Could not load moderation queue.'
  } finally {
    loading.value = false
  }

  // Keep the Reviewed tab in sync once it's been opened, since actions here
  // move replies between the two views.
  if (reviewedLoaded.value) loadReviewed()

  // Post titles are a display nicety for the reply tag — don't fail the
  // whole queue load if this call has trouble.
  try {
    const posts = await fetchPosts()
    postsBySlug.value = new Map(posts.map((post) => [postSlug(post.id), post]))
  } catch {
    // Fall back to showing the raw slug per-reply (see postTag).
  }
}

function confirmKey(key: string, action: 'approve' | 'reject') {
  return `${key}:${action}`
}

function requestConfirm(key: string, action: 'approve' | 'reject') {
  confirming.value = confirmKey(key, action)
}

function cancelConfirm() {
  confirming.value = null
}

function menuKey(key: string, action: 'approve' | 'block') {
  return `${key}:${action}`
}

function toggleMenu(key: string, action: 'approve' | 'block') {
  const target = menuKey(key, action)
  openMenu.value = openMenu.value === target ? null : target
}

function closeMenu() {
  openMenu.value = null
}

function handleDocumentClick(e: MouseEvent) {
  if (openMenu.value && !(e.target as HTMLElement).closest('.split-btn')) closeMenu()
}

onMounted(() => document.addEventListener('click', handleDocumentClick))
onUnmounted(() => document.removeEventListener('click', handleDocumentClick))

async function doApprove(reply: PendingReply) {
  error.value = ''
  try {
    await approveReply(reply.key)
    confirming.value = null
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Approve failed'
  }
}

async function doReject(reply: PendingReply) {
  error.value = ''
  try {
    await rejectReply(reply.key)
    confirming.value = null
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Reject failed'
  }
}

// Blocking an actor from the queue is a quick action composed from the two
// calls U3 already exposes — no new API needed.
async function doBlockActor(reply: PendingReply) {
  error.value = ''
  try {
    await addBlockedActor(reply.actor)
    await rejectReply(reply.key)
    closeMenu()
    confirming.value = null
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Block failed'
  }
}

// "Always approve" trusts the actor going forward and approves this reply
// in one step, mirroring the existing "Block actor" quick action.
async function doAlwaysApprove(reply: PendingReply) {
  error.value = ''
  try {
    await addTrustedActor(reply.actor)
    await approveReply(reply.key)
    closeMenu()
    confirming.value = null
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Approve failed'
  }
}

// Skip is a lightweight set-aside — no confirm step, no effect on the
// sending actor's trust/block status, and reversible from the Reviewed tab.
async function doSkip(reply: PendingReply) {
  error.value = ''
  try {
    await skipReply(reply.key)
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Skip failed'
  }
}

async function doRevert(reply: PendingReply) {
  error.value = ''
  try {
    await revertReply(reply.key)
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Revert failed'
  }
}

const activeActors = computed(() => (activeList.value === 'trusted' ? trusted.value : blocked.value))

const filteredActors = computed(() => {
  const q = actorFilter.value.trim().toLowerCase()
  if (!q) return activeActors.value
  return activeActors.value.filter((actor) => actor.toLowerCase().includes(q))
})

function selectList(list: 'trusted' | 'blocked') {
  activeList.value = list
  actorFilter.value = ''
  newActor.value = ''
}

async function submitActor() {
  const actor = newActor.value.trim()
  if (!actor) return
  error.value = ''
  try {
    if (activeList.value === 'trusted') await addTrustedActor(actor)
    else await addBlockedActor(actor)
    newActor.value = ''
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : `Add ${activeList.value} actor failed`
  }
}

async function removeActor(actor: string) {
  error.value = ''
  try {
    if (activeList.value === 'trusted') await removeTrustedActor(actor)
    else await removeBlockedActor(actor)
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : `Remove ${activeList.value} actor failed`
  }
}

onMounted(async () => {
  load()
  try {
    showAvatars.value = avatarsEnabledFromConfig((await fetchSiteConfig()).federation)
  } catch {
    showAvatars.value = true
  }
})
</script>

<template>
  <p v-if="loading" class="status">Loading…</p>
  <p v-else-if="error && pending.length === 0" class="status error">{{ error }}</p>

  <section v-else class="stack">
    <h3 class="section-title">Replies</h3>

    <div class="pill-toggle" role="tablist" aria-label="Reply queue">
      <button
        type="button"
        role="tab"
        :aria-selected="replyView === 'pending'"
        class="pill pill-pending"
        :class="{ active: replyView === 'pending' }"
        @click="selectReplyView('pending')"
      >
        <Inbox :size="14" :stroke-width="2" aria-hidden="true" />
        Pending
        <span class="pill-count">{{ pending.length }}</span>
      </button>
      <button
        type="button"
        role="tab"
        :aria-selected="replyView === 'reviewed'"
        class="pill pill-reviewed"
        :class="{ active: replyView === 'reviewed' }"
        @click="selectReplyView('reviewed')"
      >
        <History :size="14" :stroke-width="2" aria-hidden="true" />
        Reviewed
        <span v-if="reviewedLoaded" class="pill-count">{{ reviewed.length }}</span>
      </button>
    </div>

    <template v-if="replyView === 'pending'">
      <p v-if="pending.length === 0" class="hint">No pending replies.</p>
      <ul v-else class="reply-list">
        <li v-for="reply in pending" :key="reply.key" class="reply-item">
          <AdminReplyBody :reply="reply" :show-avatars="showAvatars" :post-tag="postTag(reply.post_slug)" />

          <div v-if="confirming === confirmKey(reply.key, 'approve')" class="form-actions">
            <div class="form-actions-start">
              <span>Approve and publish this reply?</span>
            </div>
            <div class="form-actions-end">
              <button type="button" class="btn btn-primary" @click="doApprove(reply)">Confirm approve</button>
              <button type="button" class="btn btn-ghost" @click="cancelConfirm">Cancel</button>
            </div>
          </div>
          <div v-else-if="confirming === confirmKey(reply.key, 'reject')" class="form-actions">
            <div class="form-actions-start">
              <span>Reject this reply?</span>
            </div>
            <div class="form-actions-end">
              <button type="button" class="btn btn-primary" @click="doReject(reply)">Confirm reject</button>
              <button type="button" class="btn btn-ghost" @click="cancelConfirm">Cancel</button>
            </div>
          </div>
          <div v-else class="form-actions">
            <div class="split-btn">
              <button
                type="button"
                class="btn btn-primary split-btn-main"
                @click="requestConfirm(reply.key, 'approve')"
              >
                Approve
              </button>
              <button
                type="button"
                class="btn btn-primary split-btn-caret"
                aria-haspopup="true"
                :aria-expanded="openMenu === menuKey(reply.key, 'approve')"
                title="More approve actions"
                aria-label="More approve actions"
                @click.stop="toggleMenu(reply.key, 'approve')"
              >
                <ChevronDown :size="14" :stroke-width="2" aria-hidden="true" />
              </button>
              <div v-if="openMenu === menuKey(reply.key, 'approve')" class="split-menu" role="menu">
                <button type="button" role="menuitem" class="split-menu-item" @click="doAlwaysApprove(reply)">
                  Always approve
                  <span class="split-menu-hint">Trust this actor going forward</span>
                </button>
              </div>
            </div>

            <button type="button" class="btn btn-ghost" @click="doSkip(reply)">
              Skip
            </button>

            <div class="split-btn">
              <button
                type="button"
                class="btn btn-ghost split-btn-main"
                @click="requestConfirm(reply.key, 'reject')"
              >
                Block
              </button>
              <button
                type="button"
                class="btn btn-ghost split-btn-caret"
                aria-haspopup="true"
                :aria-expanded="openMenu === menuKey(reply.key, 'block')"
                title="More block actions"
                aria-label="More block actions"
                @click.stop="toggleMenu(reply.key, 'block')"
              >
                <ChevronDown :size="14" :stroke-width="2" aria-hidden="true" />
              </button>
              <div v-if="openMenu === menuKey(reply.key, 'block')" class="split-menu" role="menu">
                <button type="button" role="menuitem" class="split-menu-item" @click="doBlockActor(reply)">
                  Always block
                  <span class="split-menu-hint">Block this actor going forward</span>
                </button>
              </div>
            </div>
          </div>
        </li>
      </ul>
    </template>

    <template v-else>
      <p v-if="reviewedLoading" class="status">Loading…</p>
      <p v-else-if="reviewed.length === 0" class="hint">No reviewed replies yet.</p>
      <ul v-else class="reply-list">
        <li v-for="reply in reviewed" :key="reply.key" class="reply-item">
          <AdminReplyBody
            :reply="reply"
            :show-avatars="showAvatars"
            :post-tag="postTag(reply.post_slug)"
            :status="reply.status === 'pending' ? undefined : reply.status"
          />

          <div class="form-actions">
            <button type="button" class="btn btn-ghost" @click="doRevert(reply)">
              <Undo2 :size="14" :stroke-width="2" aria-hidden="true" />
              Revert to pending
            </button>
          </div>
        </li>
      </ul>
    </template>

    <h3 class="section-title">Actor lists</h3>
    <p class="hint">Trusted actors auto-approve, skipping the queue. Blocked actors are auto-rejected at ingestion.</p>

    <div class="pill-toggle" role="tablist" aria-label="Actor list">
      <button
        type="button"
        role="tab"
        :aria-selected="activeList === 'trusted'"
        class="pill pill-trusted"
        :class="{ active: activeList === 'trusted' }"
        @click="selectList('trusted')"
      >
        <Check :size="14" :stroke-width="2.5" aria-hidden="true" />
        Trusted
        <span class="pill-count">{{ trusted.length }}</span>
      </button>
      <button
        type="button"
        role="tab"
        :aria-selected="activeList === 'blocked'"
        class="pill pill-blocked"
        :class="{ active: activeList === 'blocked' }"
        @click="selectList('blocked')"
      >
        <Ban :size="14" :stroke-width="2" aria-hidden="true" />
        Blocked
        <span class="pill-count">{{ blocked.length }}</span>
      </button>
    </div>

    <SearchField
      v-model="actorFilter"
      class="actor-filter"
      :placeholder="`Filter ${activeList} actors…`"
      :aria-label="`Filter ${activeList} actors`"
    />

    <table class="actor-table">
      <thead>
        <tr>
          <th scope="col" class="actor-table-icon-col"></th>
          <th scope="col">Actor</th>
          <th scope="col"></th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="activeActors.length === 0">
          <td colspan="3" class="hint">No {{ activeList }} actors.</td>
        </tr>
        <tr v-else-if="filteredActors.length === 0">
          <td colspan="3" class="hint">No {{ activeList }} actors match “{{ actorFilter }}”.</td>
        </tr>
        <tr v-for="actor in filteredActors" :key="actor">
          <td class="actor-row-icon" :class="activeList === 'trusted' ? 'is-trusted' : 'is-blocked'">
            <Check v-if="activeList === 'trusted'" :size="14" :stroke-width="2.5" aria-hidden="true" />
            <Ban v-else :size="14" :stroke-width="2" aria-hidden="true" />
          </td>
          <td class="actor-uri">{{ actor }}</td>
          <td class="actor-uri-actions">
            <button
              type="button"
              class="btn btn-ghost btn-icon btn-danger"
              :title="`Remove ${actor}`"
              :aria-label="`Remove ${actor}`"
              @click="removeActor(actor)"
            >
              <Trash2 :size="16" :stroke-width="1.75" aria-hidden="true" />
            </button>
          </td>
        </tr>
      </tbody>
    </table>

    <form class="actor-form" @submit.prevent="submitActor">
      <label class="actor-add-field">
        <Plus :size="16" :stroke-width="1.75" class="actor-add-icon" aria-hidden="true" />
        <input
          v-model="newActor"
          type="text"
          :placeholder="
            activeList === 'trusted'
              ? 'https://example.social/users/alice'
              : 'https://example.social/users/mallory'
          "
        />
      </label>
      <button type="submit" class="btn btn-primary actor-submit-btn" :disabled="!newActor.trim()">
        Add {{ activeList }}
      </button>
    </form>

    <p v-if="error" class="status error">{{ error }}</p>
  </section>
</template>

<style scoped>
.section-title {
  margin: var(--space-4) 0 var(--space-2);
  font-size: var(--text-base);
}
.section-title:first-of-type {
  margin-top: 0;
}
.hint {
  margin: 0 0 var(--space-2);
  color: var(--muted);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
}
.reply-list,
.actor-list {
  list-style: none;
  margin: 0 0 var(--space-3);
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}
.reply-item {
  display: flex;
  flex-direction: column;
  padding: var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--surface) 85%, var(--border));
}
.form-actions-start span {
  font-size: var(--text-sm);
  color: var(--muted);
}
.split-btn {
  position: relative;
  display: inline-flex;
}
.split-btn-main {
  border-top-right-radius: 0;
  border-bottom-right-radius: 0;
}
.split-btn-caret {
  padding-left: 0.5rem;
  padding-right: 0.5rem;
  border-top-left-radius: 0;
  border-bottom-left-radius: 0;
  border-left: 1px solid color-mix(in srgb, currentColor 25%, transparent);
}
.split-menu {
  position: absolute;
  z-index: 10;
  top: calc(100% + 0.3rem);
  right: 0;
  min-width: 13rem;
  padding: 0.3rem;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
}
.split-menu-item {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.15rem;
  width: 100%;
  padding: 0.4rem 0.6rem;
  border: none;
  border-radius: var(--radius-sm);
  background: none;
  color: var(--text);
  font: inherit;
  font-size: var(--text-sm);
  font-weight: 500;
  text-align: left;
  cursor: pointer;
}
.split-menu-item:hover {
  background: color-mix(in srgb, var(--accent) 12%, transparent);
}
.split-menu-hint {
  font-size: var(--text-xs);
  font-weight: 400;
  color: var(--muted);
}
.pill-toggle {
  display: flex;
  gap: var(--space-1);
  padding: 0.2rem;
  margin: 0 0 var(--space-3);
  border-radius: 999px;
  background: color-mix(in srgb, var(--surface) 85%, var(--border));
  border: 1px solid var(--border);
}
.pill {
  flex: 1 1 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.35rem;
  padding: 0.3rem 0.85rem;
  border: none;
  border-radius: 999px;
  background: transparent;
  color: var(--muted);
  font: inherit;
  font-size: var(--text-sm);
  font-weight: 500;
  cursor: pointer;
  transition: background var(--transition-fast), color var(--transition-fast);
}
.pill:hover {
  color: var(--text);
}
.pill.active.pill-trusted {
  background: color-mix(in srgb, var(--success, var(--accent)) 18%, transparent);
  color: var(--success, var(--accent));
}
.pill.active.pill-blocked {
  background: color-mix(in srgb, var(--danger) 18%, transparent);
  color: var(--danger);
}
.pill.active.pill-pending {
  background: color-mix(in srgb, var(--accent) 18%, transparent);
  color: var(--accent);
}
.pill.active.pill-reviewed {
  background: color-mix(in srgb, var(--muted) 20%, transparent);
  color: var(--text);
}
.pill-count {
  padding: 0 0.4rem;
  border-radius: 999px;
  background: color-mix(in srgb, currentColor 18%, transparent);
  font-size: var(--text-xs);
  font-weight: 700;
}
.actor-filter {
  width: 100%;
  margin: 0 0 var(--space-3);
  border-radius: var(--radius-md);
}
.actor-table {
  width: 100%;
  border-collapse: collapse;
  margin: 0 0 var(--space-3);
  font-size: var(--text-sm);
}
.actor-table th {
  padding: var(--space-1) var(--space-2);
  text-align: left;
  color: var(--muted);
  font-size: var(--text-xs);
  text-transform: uppercase;
  letter-spacing: 0.03em;
  border-bottom: 1px solid var(--border);
}
.actor-table td {
  padding: var(--space-2);
  border-bottom: 1px solid var(--border);
  vertical-align: middle;
}
.actor-table tr:last-child td {
  border-bottom: none;
}
.actor-uri {
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: var(--text-xs);
  word-break: break-all;
}
.actor-uri-actions {
  width: 1%;
  white-space: nowrap;
  text-align: right;
}
.actor-table-icon-col {
  width: 1%;
}
.actor-table td.actor-row-icon {
  width: 1%;
  padding-right: 0;
  text-align: center;
}
.actor-row-icon.is-trusted {
  color: var(--success, var(--accent));
}
.actor-row-icon.is-blocked {
  color: var(--danger);
}
.actor-form {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: var(--space-2);
  margin: 0 0 var(--space-4);
}
@media (max-width: 47.99rem) {
  /* Once the input wraps to its own full-width line, the button drops
     to a second line — give it breathing room from the field above it. */
  .actor-submit-btn {
    margin-top: var(--space-2);
  }
}
.actor-submit-btn {
  /* "Add trusted" / "Add blocked" — fixed to the wider of the two labels
     so toggling between lists doesn't resize this button and shrink/grow
     the input field beside it. */
  min-width: calc(11ch + 2 * var(--space-4));
}
.actor-add-field {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  flex: 1 1 16rem;
  min-width: 0;
  padding: 0.35rem 0.65rem;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
}
.actor-add-icon {
  flex-shrink: 0;
  color: var(--muted);
}
.actor-add-field input {
  flex: 1;
  min-width: 0;
  border: none;
  background: transparent;
  color: var(--text);
  font: inherit;
  font-size: var(--text-sm);
  outline: none;
}
.actor-add-field input::placeholder {
  color: var(--muted);
}
@media (max-width: 47.99rem) {
  /* iOS Safari auto-zooms on focus when an input's font-size is under 16px */
  .actor-add-field input {
    font-size: 16px;
  }
}
.status {
  margin: 0;
  color: var(--muted);
  font-size: var(--text-sm);
}
.status.error {
  color: var(--danger);
}
</style>
