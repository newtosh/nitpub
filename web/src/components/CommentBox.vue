<script setup lang="ts">
import { ArrowLeft } from '@lucide/vue'
import { onMounted, ref } from 'vue'
import {
  clearCommentReturnState,
  fetchCommentSession,
  logoutComment,
  readCommentReturnState,
  startCommentAuth,
  type CommentSession,
} from '../lib/comments'
import MastodonHelpModal from './MastodonHelpModal.vue'
import MastodonIcon from './icons/MastodonIcon.vue'

const props = defineProps<{ postSlug: string }>()

type Phase = 'signed-out' | 'entering-instance' | 'signed-in'

const loading = ref(true)
const phase = ref<Phase>('signed-out')
const session = ref<CommentSession | null>(null)
const instance = ref('')
const draftText = ref('')
const submitting = ref(false)
const error = ref('')
const returnStatus = ref<'success' | null>(null)
const helpOpen = ref(false)

onMounted(async () => {
  const { status, draft } = readCommentReturnState()
  if (status) clearCommentReturnState()

  try {
    session.value = await fetchCommentSession()
  } catch {
    session.value = null
  }

  if (session.value) {
    // Every post (even a remembered visitor's) round-trips through OAuth
    // (revoke-every-use), so a failure here still needs the draft restored.
    phase.value = 'signed-in'
    if (status === 'success') {
      returnStatus.value = 'success'
    } else if (status === 'error_auth') {
      draftText.value = draft
      error.value = "Something went wrong finishing sign-in. Your comment is still here — try again."
    } else if (status === 'error_expired') {
      error.value = 'That took too long and your session expired — please retype your comment.'
    }
  } else if (status === 'error_instance') {
    phase.value = 'entering-instance'
    error.value = "Couldn't reach that instance. Check the domain and try again."
  } else if (status === 'error_auth' || status === 'error_expired') {
    // First-time sign-in was abandoned or timed out before a session was
    // ever created — nothing to restore, just let them try again.
    phase.value = 'entering-instance'
    error.value =
      status === 'error_expired'
        ? 'That took too long — please try again.'
        : 'Sign-in was cancelled or failed — try again.'
  } else {
    phase.value = 'signed-out'
  }

  loading.value = false
})

function beginSignIn() {
  phase.value = 'entering-instance'
}

async function submitInstance() {
  const targetInstance = instance.value.trim()
  if (!targetInstance) {
    error.value = 'Enter your Mastodon handle or server (e.g. username@mastodon.social).'
    return
  }
  error.value = ''
  submitting.value = true
  try {
    // No draft yet at this phase — the visitor hasn't seen a comment box,
    // only identified themselves. They compose after returning, signed in.
    await startCommentAuth(props.postSlug, targetInstance, '')
  } catch (e) {
    submitting.value = false
    error.value = e instanceof Error ? e.message : 'Something went wrong. Try again.'
  }
}

async function submitComment() {
  if (submitting.value || !draftText.value.trim() || !session.value) return
  error.value = ''
  submitting.value = true
  try {
    const outcome = await startCommentAuth(props.postSlug, session.value.instance, draftText.value)
    // 'redirected' navigates the browser away, so leaving submitting=true
    // (button reads "Redirecting…") is deliberate — no need to reset it.
    if (outcome === 'posted') {
      draftText.value = ''
      returnStatus.value = 'success'
      submitting.value = false
    }
  } catch (e) {
    submitting.value = false
    error.value = e instanceof Error ? e.message : 'Something went wrong. Try again.'
  }
}

async function logout() {
  try {
    await logoutComment()
  } finally {
    session.value = null
    phase.value = 'signed-out'
  }
}
</script>

<template>
  <section class="comment-box">
    <p class="powered-by" :class="{ centered: phase === 'signed-out' || phase === 'entering-instance' }">
      Comments are powered by Mastodon
      <button type="button" class="help-btn" aria-label="What is Mastodon?" @click="helpOpen = true">?</button>
    </p>

    <p v-if="loading" class="status">Loading…</p>

    <p v-else-if="returnStatus === 'success'" class="status ok">
      Comment sent — it's pending review before it appears here.
    </p>

    <div v-else-if="phase === 'signed-out' || phase === 'entering-instance'" class="stack">
      <p class="sign-in-prompt" :class="{ error }">
        {{ error || 'To comment, sign in with your Mastodon account.' }}
      </p>

      <!-- Same row, same width, same center point in both phases — the
           button morphs into the instance input in place instead of the
           page reflowing to a new position for the next step. -->
      <Transition name="row-swap" mode="out-in">
        <div v-if="phase === 'signed-out'" key="signed-out" class="sign-in-row sign-in-row-centered">
          <button type="button" class="btn btn-primary btn-mastodon" @click="beginSignIn">
            <MastodonIcon :size="16" />
            Sign in with Mastodon
          </button>
        </div>
        <form v-else key="entering-instance" class="sign-in-row" @submit.prevent="submitInstance">
          <button
            type="button"
            class="btn btn-ghost btn-icon back-icon-btn"
            aria-label="Back"
            title="Back"
            @click="phase = 'signed-out'"
          >
            <ArrowLeft :size="16" :stroke-width="1.75" aria-hidden="true" />
          </button>
          <input
            v-model="instance"
            class="input"
            type="text"
            placeholder="username@mastodon.social"
            autocomplete="off"
            autofocus
          />
          <button type="submit" class="btn btn-primary" :disabled="submitting">
            {{ submitting ? 'Redirecting…' : 'Continue' }}
          </button>
        </form>
      </Transition>
    </div>

    <form v-else class="stack" @submit.prevent="submitComment">
      <p v-if="error" class="status error">{{ error }}</p>

      <p class="signed-in-as">
        Commenting as <strong>@{{ session?.handle }}@{{ session?.instance }}</strong>
        <button type="button" class="text-link" @click="logout">Log out</button>
      </p>

      <label class="label">
        Comment
        <textarea v-model="draftText" class="input" rows="3" placeholder="Say something…"></textarea>
      </label>

      <div class="form-actions">
        <button type="submit" class="btn btn-primary" :disabled="submitting">
          {{ submitting ? 'Redirecting…' : 'Post comment' }}
        </button>
      </div>
    </form>

    <MastodonHelpModal v-model:open="helpOpen" />
  </section>
</template>

<style scoped>
.comment-box {
  margin-top: 1.5rem;
  padding-top: 1.5rem;
  border-top: 1px solid var(--border);
}
.powered-by {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin: 0 0 1rem;
  font-size: 0.85rem;
  color: var(--muted);
}
.powered-by.centered {
  justify-content: center;
}
.help-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.25rem;
  height: 1.25rem;
  padding: 0;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--muted);
  font-size: 0.75rem;
  line-height: 1;
  cursor: pointer;
}
.help-btn:hover {
  color: var(--text);
  border-color: var(--muted);
}
.btn-mastodon {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
}
.sign-in-prompt {
  margin: 0;
  font-size: 0.9rem;
  color: var(--text);
  text-align: center;
}
.sign-in-prompt.error {
  color: var(--danger);
}
/* Fixed width + auto margins so the row's center point is identical
   whether it holds the centered sign-in button or the input+Continue
   pair — the button morphs in place instead of the page reflowing. */
.sign-in-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  width: 100%;
  max-width: 28rem;
  margin: 0 auto;
}
.sign-in-row-centered {
  justify-content: center;
}
.sign-in-row .input {
  flex: 1;
}
.back-icon-btn {
  flex-shrink: 0;
}
.row-swap-enter-active,
.row-swap-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}
.row-swap-enter-from,
.row-swap-leave-to {
  opacity: 0;
  transform: translateY(2px);
}
.signed-in-as {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  margin: 0;
  font-size: 0.9rem;
}
.text-link {
  background: none;
  border: none;
  padding: 0;
  color: var(--accent);
  text-decoration: underline;
  cursor: pointer;
  font-size: inherit;
}
.form-actions {
  display: flex;
  justify-content: flex-end;
}
.status {
  color: var(--muted);
}
.status.ok {
  color: var(--accent);
}
.status.error {
  color: var(--danger);
}
</style>
