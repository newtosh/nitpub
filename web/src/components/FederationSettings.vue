<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import InfoTip from './InfoTip.vue'
import {
  backfillFederation,
  connectBluesky,
  disconnectBluesky,
  disconnectReference,
  fetchBlueskyStatus,
  fetchFederationDeliveries,
  fetchFederationInfo,
  fetchReferenceStatus,
  redeliverShared,
  resendAccepts,
  resolveReferencePermalinks,
  startReferenceConnect,
  type BlueskyStatus,
  type FederationDelivery,
  type FederationInfo,
  type ReferenceStatus,
} from '../lib/federationAdmin'
import BlueskyIcon from './icons/BlueskyIcon.vue'
import { fetchAdminSite, saveManifest } from '../lib/adminSite'
import { clearSiteConfigCache } from '../lib/site'
import { formatDate } from '../lib/posts'
import {
  avatarsEnabledFromConfig,
  crossPostDefaultFromConfig,
  moderationEnabledFromConfig,
  repliesCollapsedByDefaultFromConfig,
} from '../lib/federationDelivery'

const route = useRoute()
const router = useRouter()

const loading = ref(true)
const saving = ref(false)
const error = ref('')
const message = ref('')
const crossPostDefault = ref(true)
const showAvatarsDefault = ref(true)
const moderationEnabled = ref(true)
const repliesCollapsedDefault = ref(true)
const referenceInstance = ref('')
const federationInfo = ref<FederationInfo | null>(null)

const referenceStatus = ref<ReferenceStatus | null>(null)
const referenceConnecting = ref(false)
const referenceDisconnecting = ref(false)
const referenceMessage = ref('')

async function loadReferenceStatus() {
  try {
    referenceStatus.value = await fetchReferenceStatus()
  } catch {
    referenceStatus.value = null
  }
}

async function connectReference() {
  referenceConnecting.value = true
  referenceMessage.value = ''
  try {
    const { redirect_url } = await startReferenceConnect()
    window.location.href = redirect_url
  } catch (e) {
    referenceMessage.value = e instanceof Error ? e.message : 'Failed to start connect'
    referenceConnecting.value = false
  }
}

async function doDisconnectReference() {
  referenceDisconnecting.value = true
  referenceMessage.value = ''
  try {
    await disconnectReference()
    await loadReferenceStatus()
  } catch (e) {
    referenceMessage.value = e instanceof Error ? e.message : 'Failed to disconnect'
  } finally {
    referenceDisconnecting.value = false
  }
}

// Bluesky connection panel (R1) — same shape as the reference-instance
// panel above it: a status ref, loading flags for connect/disconnect, and
// a shared message ref for both success and error text.
const blueskyStatus = ref<BlueskyStatus | null>(null)
const blueskyHandle = ref('')
const blueskyAppPassword = ref('')
const blueskyConnecting = ref(false)
const blueskyDisconnecting = ref(false)
const blueskyMessage = ref('')

async function loadBlueskyStatus() {
  try {
    blueskyStatus.value = await fetchBlueskyStatus()
  } catch {
    blueskyStatus.value = null
  }
}

async function doConnectBluesky() {
  blueskyConnecting.value = true
  blueskyMessage.value = ''
  try {
    await connectBluesky(blueskyHandle.value.trim(), blueskyAppPassword.value)
    blueskyAppPassword.value = ''
    await loadBlueskyStatus()
  } catch (e) {
    blueskyMessage.value = e instanceof Error ? e.message : 'Failed to connect'
  } finally {
    blueskyConnecting.value = false
  }
}

async function doDisconnectBluesky() {
  blueskyDisconnecting.value = true
  blueskyMessage.value = ''
  try {
    await disconnectBluesky()
    await loadBlueskyStatus()
  } catch (e) {
    blueskyMessage.value = e instanceof Error ? e.message : 'Failed to disconnect'
  } finally {
    blueskyDisconnecting.value = false
  }
}

const deliveries = ref<FederationDelivery[]>([])
const deliveriesError = ref('')
const deliveriesLoading = ref(true)

type DeliveryActionKey = 'resend' | 'backfill' | 'redeliver' | 'resolvePermalinks'
type DeliveryActionResult = { sent: number; skipped?: number }
const deliveryActions = reactive<Record<DeliveryActionKey, { loading: boolean; message: string }>>({
  resend: { loading: false, message: '' },
  backfill: { loading: false, message: '' },
  redeliver: { loading: false, message: '' },
  resolvePermalinks: { loading: false, message: '' },
})

async function loadDeliveries() {
  deliveriesLoading.value = true
  deliveriesError.value = ''
  try {
    const data = await fetchFederationDeliveries(50)
    deliveries.value = data.deliveries
  } catch {
    deliveriesError.value = 'Could not load delivery log.'
  } finally {
    deliveriesLoading.value = false
  }
}

async function runDeliveryAction<T extends DeliveryActionResult>(
  key: DeliveryActionKey,
  fn: () => Promise<T>,
  formatMessage: (result: T) => string,
  failMessage: string,
) {
  const action = deliveryActions[key]
  action.loading = true
  action.message = ''
  try {
    const result = await fn()
    action.message = formatMessage(result)
    await loadDeliveries()
  } catch (e) {
    action.message = e instanceof Error ? e.message : failMessage
  } finally {
    action.loading = false
  }
}

const doResendAccepts = () =>
  runDeliveryAction('resend', resendAccepts, (r) => `Sent ${r.sent}.`, 'Resend accepts failed')
const doBackfill = () =>
  runDeliveryAction('backfill', backfillFederation, (r) => `Sent ${r.sent}, skipped ${r.skipped}.`, 'Backfill failed')
const doRedeliverShared = () =>
  runDeliveryAction('redeliver', redeliverShared, (r) => `Sent ${r.sent}, skipped ${r.skipped}.`, 'Redeliver failed')
const doResolvePermalinks = () =>
  runDeliveryAction(
    'resolvePermalinks',
    resolveReferencePermalinks,
    (r) => `Resolved ${r.sent}, skipped ${r.skipped}.`,
    'Failed to resolve permalinks',
  )

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [data, info] = await Promise.all([fetchAdminSite(), fetchFederationInfo()])
    crossPostDefault.value = crossPostDefaultFromConfig(data.manifest.federation)
    showAvatarsDefault.value = avatarsEnabledFromConfig(data.manifest.federation)
    moderationEnabled.value = moderationEnabledFromConfig(data.manifest.federation)
    repliesCollapsedDefault.value = repliesCollapsedByDefaultFromConfig(data.manifest.federation)
    referenceInstance.value = data.manifest.federation?.reference_instance || ''
    federationInfo.value = info
  } catch {
    error.value = 'Could not load federation settings.'
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  error.value = ''
  message.value = ''
  try {
    const data = await fetchAdminSite()
    data.manifest.federation = {
      cross_post_default: crossPostDefault.value,
      show_avatars_default: showAvatarsDefault.value,
      moderation_enabled: moderationEnabled.value,
      replies_collapsed_default: repliesCollapsedDefault.value,
      reference_instance: referenceInstance.value.trim(),
    }
    await saveManifest(data.manifest)
    clearSiteConfigCache()
    message.value = 'Saved.'
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Save failed'
  } finally {
    saving.value = false
  }
}

onMounted(load)
onMounted(loadDeliveries)
onMounted(loadReferenceStatus)
onMounted(loadBlueskyStatus)
onMounted(() => {
  const result = route.query.reference
  if (result === 'connected') {
    referenceMessage.value = 'Connected.'
  } else if (result === 'error') {
    referenceMessage.value = "Couldn't connect — check the instance domain and try again."
  }
  if (result) {
    router.replace({ path: route.path })
  }
})
</script>

<template>
  <p v-if="loading" class="status">Loading…</p>
  <p v-else-if="error && !federationInfo" class="status error">{{ error }}</p>

  <section v-else class="stack">
    <h3 class="section-title">
      Actor
      <InfoTip label="ActivityPub identity used when posts are shared to the fediverse." />
    </h3>
    <p v-if="federationInfo" class="actor-handle">
      <span class="handle-label">@{{ federationInfo.actor }}@{{ federationInfo.domain }}</span>
    </p>
    <p v-if="federationInfo" class="follow-policy">
      <span class="policy-badge">Open follows</span>
      Anyone can follow instantly — no approval queue.
      <span v-if="federationInfo.follower_count > 0" class="follower-count">
        {{ federationInfo.follower_count }} follower{{ federationInfo.follower_count === 1 ? '' : 's' }} stored.
      </span>
    </p>
    <p class="hint">
      To rename the posting handle, set <code>actor</code> in <code>config.toml</code> and restart
      nitpub. Followers may need to re-follow after a handle change.
    </p>

    <h3 class="section-title">
      Publishing
      <InfoTip label="Default for the “Share to fediverse” checkbox when composing a new post." />
    </h3>
    <label class="field checkbox">
      <input v-model="crossPostDefault" type="checkbox" />
      <span>Share new posts to the fediverse by default</span>
    </label>

    <h3 class="section-title">
      Replies
      <InfoTip label="Controls whether reply authors' profile avatars are shown on the public thread and moderation queue." />
    </h3>
    <label class="field checkbox">
      <input v-model="showAvatarsDefault" type="checkbox" />
      <span>Show reply author avatars by default</span>
    </label>
    <label class="field checkbox">
      <input v-model="moderationEnabled" type="checkbox" />
      <span>Require approval before replies are published</span>
    </label>
    <p v-if="!moderationEnabled" class="hint moderation-off-hint">
      Every incoming reply auto-approves and publishes immediately — no review queue.
      Blocked actors are still rejected.
    </p>
    <label class="field checkbox">
      <input v-model="repliesCollapsedDefault" type="checkbox" />
      <span>Collapse replies by default on post pages</span>
    </label>

    <div class="form-actions">
      <button type="button" class="btn btn-primary" :disabled="saving" @click="save">
        {{ saving ? 'Saving…' : 'Save settings' }}
      </button>
    </div>
    <p v-if="message" class="status ok">{{ message }}</p>
    <p v-if="error" class="status error">{{ error }}</p>

    <h3 class="section-title">
      Reference instance
      <InfoTip label="Optional. Resolves each shared post's permalink on one Mastodon instance, so a link to how it looks there can be shown on the post. Every instance assigns its own local ID, so this only ever reflects one instance's mirror, not a universal fediverse URL." />
    </h3>
    <label class="field">
      <span>Instance domain</span>
      <input v-model="referenceInstance" type="text" placeholder="mastodon.social" />
    </label>

    <p v-if="referenceMessage" class="status">{{ referenceMessage }}</p>

    <template v-if="referenceStatus?.connected">
      <p class="follow-policy">
        <span class="policy-badge">Connected</span>
        {{ referenceStatus.instance }}
      </p>
      <div class="delivery-actions">
        <div class="delivery-action">
          <button type="button" class="btn" :disabled="deliveryActions.resolvePermalinks.loading" @click="doResolvePermalinks">
            {{ deliveryActions.resolvePermalinks.loading ? 'Resolving…' : 'Resolve permalinks for already-shared posts' }}
          </button>
          <span v-if="deliveryActions.resolvePermalinks.message" class="delivery-action-result">{{ deliveryActions.resolvePermalinks.message }}</span>
        </div>
        <div class="delivery-action">
          <button type="button" class="btn btn-ghost" :disabled="referenceDisconnecting" @click="doDisconnectReference">
            {{ referenceDisconnecting ? 'Disconnecting…' : 'Disconnect' }}
          </button>
        </div>
      </div>
    </template>
    <div v-else class="delivery-actions">
      <div class="delivery-action">
        <button type="button" class="btn" :disabled="referenceConnecting" @click="connectReference">
          {{ referenceConnecting ? 'Redirecting…' : 'Connect reference instance' }}
        </button>
      </div>
    </div>

    <h3 class="section-title">
      <span class="section-title-icon"><BlueskyIcon :size="16" />Bluesky</span>
      <InfoTip label="Direct crosspost target alongside the fediverse. Connect a Bluesky account with an app password (not your main account password) to enable it in the composer." />
    </h3>
    <p v-if="blueskyMessage" class="status">{{ blueskyMessage }}</p>

    <template v-if="blueskyStatus?.connected">
      <p class="follow-policy">
        <span class="policy-badge">Connected</span>
        @{{ blueskyStatus.handle }}
      </p>
      <p v-if="blueskyStatus.needs_reconnect" class="hint moderation-off-hint">
        This connection needs to be re-established — reconnect with a fresh app password below.
      </p>
      <div class="delivery-actions">
        <div class="delivery-action">
          <button type="button" class="btn btn-ghost" :disabled="blueskyDisconnecting" @click="doDisconnectBluesky">
            {{ blueskyDisconnecting ? 'Disconnecting…' : 'Disconnect' }}
          </button>
        </div>
      </div>
    </template>
    <template v-else>
      <label class="field">
        <span>Handle</span>
        <input v-model="blueskyHandle" type="text" placeholder="you.bsky.social" autocomplete="off" />
      </label>
      <label class="field">
        <span>App password</span>
        <input
          v-model="blueskyAppPassword"
          type="password"
          placeholder="xxxx-xxxx-xxxx-xxxx"
          autocomplete="off"
        />
      </label>
      <div class="delivery-actions">
        <div class="delivery-action">
          <button
            type="button"
            class="btn"
            :disabled="blueskyConnecting || !blueskyHandle.trim() || !blueskyAppPassword"
            @click="doConnectBluesky"
          >
            {{ blueskyConnecting ? 'Connecting…' : 'Connect Bluesky' }}
          </button>
        </div>
      </div>
    </template>

    <h3 class="section-title">
      Delivery
      <InfoTip label="Manually trigger federation delivery recovery actions and see each post's current delivery status." />
    </h3>
    <div class="delivery-actions">
      <div class="delivery-action">
        <button type="button" class="btn" :disabled="deliveryActions.resend.loading" @click="doResendAccepts">
          {{ deliveryActions.resend.loading ? 'Resending…' : 'Resend Accepts' }}
        </button>
        <span v-if="deliveryActions.resend.message" class="delivery-action-result">{{ deliveryActions.resend.message }}</span>
      </div>
      <div class="delivery-action">
        <button type="button" class="btn" :disabled="deliveryActions.backfill.loading" @click="doBackfill">
          {{ deliveryActions.backfill.loading ? 'Backfilling…' : 'Backfill' }}
        </button>
        <span v-if="deliveryActions.backfill.message" class="delivery-action-result">{{ deliveryActions.backfill.message }}</span>
      </div>
      <div class="delivery-action">
        <button type="button" class="btn" :disabled="deliveryActions.redeliver.loading" @click="doRedeliverShared">
          {{ deliveryActions.redeliver.loading ? 'Redelivering…' : 'Redeliver Shared' }}
        </button>
        <span v-if="deliveryActions.redeliver.message" class="delivery-action-result">{{ deliveryActions.redeliver.message }}</span>
      </div>
    </div>

    <p v-if="deliveriesLoading" class="status">Loading delivery log…</p>
    <p v-else-if="deliveriesError" class="status error">{{ deliveriesError }}</p>
    <p v-else-if="deliveries.length === 0" class="status">No posts yet.</p>
    <ul v-else class="delivery-log">
      <li v-for="d in deliveries" :key="d.slug" class="delivery-row">
        <span class="delivery-status-badge" :class="d.status">{{ d.status }}</span>
        <RouterLink :to="`/p/${d.slug}`" target="_blank" rel="noopener noreferrer" class="delivery-slug">
          {{ d.slug }}
        </RouterLink>
        <time class="delivery-date" :datetime="d.created_at">{{ formatDate(d.created_at) }}</time>
        <span v-if="d.error" class="delivery-error" :title="d.error">{{ d.error }}</span>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: var(--space-4) 0 var(--space-2);
  font-size: var(--text-base);
}
.section-title:first-of-type {
  margin-top: 0;
}
.section-title-icon {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  font-size: var(--text-sm);
}
.field.checkbox {
  flex-direction: row;
  align-items: center;
  gap: var(--space-2);
}
.actor-handle {
  margin: 0 0 var(--space-2);
}
.handle-label {
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: var(--text-sm);
  color: var(--text);
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--surface) 85%, var(--border));
}
.follow-policy {
  margin: 0 0 var(--space-4);
  color: var(--muted);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
}
.policy-badge {
  display: inline-block;
  margin-right: var(--space-2);
  padding: 0.1rem 0.45rem;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--accent) 18%, transparent);
  color: var(--accent);
  font-size: var(--text-xs);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}
.follower-count {
  display: block;
  margin-top: var(--space-1);
}
.hint {
  margin: 0 0 var(--space-4);
  color: var(--muted);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
}
.moderation-off-hint {
  margin: var(--space-2) 0 var(--space-4);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--danger) 12%, transparent);
  color: var(--danger);
}
.hint code {
  font-size: 0.92em;
}
.status {
  margin: 0;
  color: var(--muted);
  font-size: var(--text-sm);
}
.status.ok {
  color: var(--accent);
}
.status.error {
  color: var(--danger);
}
.delivery-actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-4);
  margin: 0 0 var(--space-4);
}
.delivery-action {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  align-items: flex-start;
}
.delivery-action-result {
  color: var(--muted);
  font-size: var(--text-xs);
}
.delivery-log {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}
.delivery-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--space-2);
  padding: var(--space-2) 0;
  border-bottom: 1px solid var(--border);
  font-size: var(--text-sm);
}
.delivery-row:last-child {
  border-bottom: none;
}
.delivery-status-badge {
  padding: 0.1rem 0.5rem;
  border-radius: var(--radius-sm);
  font-size: var(--text-xs);
  font-weight: 600;
  text-transform: capitalize;
  white-space: nowrap;
}
.delivery-status-badge.delivered {
  background: color-mix(in srgb, var(--success, var(--accent)) 18%, transparent);
  color: var(--success, var(--accent));
}
.delivery-status-badge.pending {
  background: color-mix(in srgb, var(--muted) 20%, transparent);
  color: var(--muted);
}
.delivery-status-badge.error {
  background: color-mix(in srgb, var(--danger) 18%, transparent);
  color: var(--danger);
}
.delivery-slug {
  color: var(--text);
  text-decoration: none;
  font-family: var(--font-mono, ui-monospace, monospace);
}
.delivery-slug:hover {
  color: var(--accent);
  text-decoration: underline;
}
.delivery-date {
  color: var(--muted);
  font-size: var(--text-xs);
  white-space: nowrap;
}
.delivery-error {
  color: var(--danger);
  font-size: var(--text-xs);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}
</style>
