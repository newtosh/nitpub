<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import {
  changePassword,
  disablePasskey,
  disableTOTP,
  enableTOTP,
  passkeyEnrollLink,
  regenerateBackupCodes,
} from '../lib/adminSecurity'
import { enrollWebAuthn, fetchSettings } from '../lib/auth'
import BackupCodesModal from './BackupCodesModal.vue'
import TotpEnrollModal from './TotpEnrollModal.vue'

type Panel = 'password' | 'totp' | 'backup' | 'passkey' | null

const activePanel = ref<Panel>(null)
const totpEnabled = ref(false)
const passkeyEnabled = ref(false)
const statusError = ref('')
const totpModalOpen = ref(false)
const totpModalSecret = ref('')
const totpModalQrUrl = ref('')
const backupModalOpen = ref(false)
const backupModalCodes = ref<string[]>([])

async function loadStatus() {
  const settings = await fetchSettings()
  if (!settings) {
    statusError.value = 'Status unavailable — reload the page.'
    return
  }
  statusError.value = ''
  totpEnabled.value = settings.totp_enabled
  passkeyEnabled.value = settings.webauthn_enabled
}

onMounted(loadStatus)

function setActivePanel(next: Panel) {
  totpError.value = ''
  totpMessage.value = ''
  pwError.value = ''
  pwMessage.value = ''
  backupError.value = ''
  passkeyError.value = ''
  activePanel.value = activePanel.value === next ? null : next
}

function togglePanel(panel: Exclude<Panel, null>) {
  setActivePanel(activePanel.value === panel ? null : panel)
}

async function onTotpEnabled() {
  await loadStatus()
}

// The modal needs totpPassword to survive past the 'enabled' event (it
// keeps using it to generate backup codes) — clear it only once the modal
// has actually closed.
watch(totpModalOpen, (isOpen) => {
  if (!isOpen) totpPassword.value = ''
})

// --- Change password ---

const pwCurrent = ref('')
const pwNew = ref('')
const pwConfirm = ref('')
const pwError = ref('')
const pwMessage = ref('')
const pwSaving = ref(false)

async function submitChangePassword() {
  pwError.value = ''
  pwMessage.value = ''
  if (pwNew.value !== pwConfirm.value) {
    pwError.value = 'New password and confirmation do not match.'
    return
  }
  pwSaving.value = true
  try {
    await changePassword(pwCurrent.value, pwNew.value)
    pwMessage.value = 'Password changed.'
    pwCurrent.value = ''
    pwNew.value = ''
    pwConfirm.value = ''
    await setActivePanel(null)
  } catch (e) {
    pwError.value = e instanceof Error ? e.message : 'Save failed.'
  } finally {
    pwSaving.value = false
  }
}

// --- TOTP ---

const totpPassword = ref('')
const totpError = ref('')
const totpMessage = ref('')
const totpSaving = ref(false)

async function submitStartTOTP() {
  totpError.value = ''
  totpSaving.value = true
  try {
    const result = await enableTOTP(totpPassword.value)
    totpModalSecret.value = result.secret
    totpModalQrUrl.value = result.url
    totpModalOpen.value = true
    setActivePanel(null)
  } catch (e) {
    totpError.value = e instanceof Error ? e.message : 'Could not start TOTP enrollment.'
  } finally {
    totpSaving.value = false
  }
}

async function submitDisableTOTP() {
  totpError.value = ''
  totpMessage.value = ''
  totpSaving.value = true
  try {
    await disableTOTP(totpPassword.value)
    totpMessage.value = 'TOTP disabled.'
    totpPassword.value = ''
    await loadStatus()
    setActivePanel(null)
  } catch (e) {
    totpError.value = e instanceof Error ? e.message : 'Save failed.'
  } finally {
    totpSaving.value = false
  }
}

// --- Backup codes ---

const backupPassword = ref('')
const backupError = ref('')
const backupSaving = ref(false)

async function submitRegenerateBackupCodes() {
  backupError.value = ''
  backupSaving.value = true
  try {
    backupModalCodes.value = await regenerateBackupCodes(backupPassword.value)
    backupPassword.value = ''
    backupModalOpen.value = true
    setActivePanel(null)
  } catch (e) {
    backupError.value = e instanceof Error ? e.message : 'Save failed.'
  } finally {
    backupSaving.value = false
  }
}

// --- Passkey ---

const passkeyPassword = ref('')
const passkeyError = ref('')
const passkeySaving = ref(false)

async function submitRegisterPasskey() {
  passkeyError.value = ''
  passkeySaving.value = true
  try {
    const relative = await passkeyEnrollLink(passkeyPassword.value)
    const token = new URL(relative, window.location.origin).searchParams.get('token')
    if (!token) throw new Error('Could not generate enrollment link.')
    const result = await enrollWebAuthn(token)
    if (!result.ok) throw new Error(result.error ?? 'Registration failed.')
    passkeyPassword.value = ''
    await loadStatus()
    setActivePanel(null)
  } catch (e) {
    passkeyError.value = e instanceof Error ? e.message : 'Could not register passkey.'
  } finally {
    passkeySaving.value = false
  }
}

async function submitDisablePasskey() {
  passkeyError.value = ''
  passkeySaving.value = true
  try {
    await disablePasskey(passkeyPassword.value)
    passkeyPassword.value = ''
    await loadStatus()
    await setActivePanel(null)
  } catch (e) {
    passkeyError.value = e instanceof Error ? e.message : 'Save failed.'
  } finally {
    passkeySaving.value = false
  }
}
</script>

<template>
  <div class="security-card stack">
    <p>
      Manage credentials and 2FA for <code>nitpub admin</code> directly here, or on the server with
      the CLI (init, password, totp, webauthn, reset-2fa).
    </p>
    <p v-if="statusError" class="status error">{{ statusError }}</p>

    <div class="security-controls">
      <button
        type="button"
        class="btn"
        :aria-expanded="activePanel === 'password'"
        aria-controls="security-panel"
        @click="togglePanel('password')"
      >
        Change password
      </button>
      <button
        type="button"
        class="btn"
        :aria-expanded="activePanel === 'totp'"
        aria-controls="security-panel"
        @click="togglePanel('totp')"
      >
        {{ totpEnabled ? 'Disable TOTP' : 'Enable TOTP' }}
      </button>
      <button
        type="button"
        class="btn"
        :aria-expanded="activePanel === 'backup'"
        aria-controls="security-panel"
        @click="togglePanel('backup')"
      >
        Regenerate backup codes
      </button>
      <button
        type="button"
        class="btn"
        :aria-expanded="activePanel === 'passkey'"
        aria-controls="security-panel"
        @click="togglePanel('passkey')"
      >
        {{ passkeyEnabled ? 'Disable passkey' : 'Manage passkeys' }}
      </button>
    </div>

    <div v-if="activePanel" id="security-panel" class="security-panel stack">
      <template v-if="activePanel === 'password'">
        <label class="label">
          Current password
          <input
            v-model="pwCurrent"
            class="input"
            type="password"
            autocomplete="current-password"
            @keyup.enter="submitChangePassword"
          />
        </label>
        <label class="label">
          New password
          <input
            v-model="pwNew"
            class="input"
            type="password"
            autocomplete="new-password"
            @keyup.enter="submitChangePassword"
          />
        </label>
        <label class="label">
          Confirm new password
          <input
            v-model="pwConfirm"
            class="input"
            type="password"
            autocomplete="new-password"
            @keyup.enter="submitChangePassword"
          />
        </label>
        <p v-if="pwError" class="status error">{{ pwError }}</p>
        <p v-if="pwMessage" class="status ok">{{ pwMessage }}</p>
        <div class="form-actions">
          <button type="button" class="btn btn-primary" :disabled="pwSaving" @click="submitChangePassword">
            {{ pwSaving ? 'Saving…' : 'Change password' }}
          </button>
        </div>
      </template>

      <template v-else-if="activePanel === 'totp' && totpEnabled">
        <label class="label">
          Current password
          <input
            v-model="totpPassword"
            class="input"
            type="password"
            autocomplete="current-password"
            @keyup.enter="submitDisableTOTP"
          />
        </label>
        <p v-if="totpError" class="status error">{{ totpError }}</p>
        <p v-if="totpMessage" class="status ok">{{ totpMessage }}</p>
        <div class="form-actions">
          <button type="button" class="btn btn-primary" :disabled="totpSaving" @click="submitDisableTOTP">
            {{ totpSaving ? 'Saving…' : 'Disable TOTP' }}
          </button>
        </div>
      </template>

      <template v-else-if="activePanel === 'totp'">
        <label class="label">
          Current password
          <input
            v-model="totpPassword"
            class="input"
            type="password"
            autocomplete="current-password"
            @keyup.enter="submitStartTOTP"
          />
        </label>
        <p v-if="totpError" class="status error">{{ totpError }}</p>
        <div class="form-actions">
          <button type="button" class="btn btn-primary" :disabled="totpSaving" @click="submitStartTOTP">
            {{ totpSaving ? 'Starting…' : 'Enable TOTP' }}
          </button>
        </div>
      </template>

      <template v-else-if="activePanel === 'backup'">
        <label class="label">
          Current password
          <input
            v-model="backupPassword"
            class="input"
            type="password"
            autocomplete="current-password"
            @keyup.enter="submitRegenerateBackupCodes"
          />
        </label>
        <p v-if="backupError" class="status error">{{ backupError }}</p>
        <div class="form-actions">
          <button type="button" class="btn btn-primary" :disabled="backupSaving" @click="submitRegenerateBackupCodes">
            {{ backupSaving ? 'Generating…' : 'Regenerate backup codes' }}
          </button>
        </div>
      </template>

      <template v-else-if="activePanel === 'passkey'">
        <template v-if="passkeyEnabled">
          <label class="label">
            Current password
            <input
              v-model="passkeyPassword"
              class="input"
              type="password"
              autocomplete="current-password"
              @keyup.enter="submitDisablePasskey"
            />
          </label>
          <p v-if="passkeyError" class="status error">{{ passkeyError }}</p>
          <div class="form-actions">
            <button type="button" class="btn btn-primary" :disabled="passkeySaving" @click="submitDisablePasskey">
              {{ passkeySaving ? 'Saving…' : 'Disable passkey' }}
            </button>
          </div>
        </template>
        <template v-else>
          <label class="label">
            Current password
            <input
              v-model="passkeyPassword"
              class="input"
              type="password"
              autocomplete="current-password"
              @keyup.enter="submitRegisterPasskey"
            />
          </label>
          <p v-if="passkeyError" class="status error">{{ passkeyError }}</p>
          <div class="form-actions">
            <button type="button" class="btn btn-primary" :disabled="passkeySaving" @click="submitRegisterPasskey">
              {{ passkeySaving ? 'Registering…' : 'Register passkey' }}
            </button>
          </div>
        </template>
      </template>
    </div>

    <TotpEnrollModal
      v-model:open="totpModalOpen"
      :password="totpPassword"
      :secret="totpModalSecret"
      :qr-url="totpModalQrUrl"
      @enabled="onTotpEnabled"
    />
    <BackupCodesModal v-model:open="backupModalOpen" :codes="backupModalCodes" />
  </div>
</template>

<style scoped>
.security-controls {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}
.security-controls .btn {
  width: 100%;
  justify-content: center;
}
.security-panel {
  width: 100%;
  padding-top: var(--space-4);
  border-top: 1px solid var(--border);
}
</style>
