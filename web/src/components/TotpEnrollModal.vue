<script setup lang="ts">
import { ref, watch } from 'vue'
import QRCode from 'qrcode'
import { cleanupTOTP, confirmTOTP, regenerateBackupCodes } from '../lib/adminSecurity'
import BackupCodesDisplay from './BackupCodesDisplay.vue'

const props = defineProps<{ open: boolean; password: string; secret: string; qrUrl: string }>()
const emit = defineEmits<{ 'update:open': [value: boolean]; enabled: [] }>()

type Phase = 'enroll' | 'offer-backup' | 'backup-codes'
const phase = ref<Phase>('enroll')

const code = ref('')
const error = ref('')
const saving = ref(false)
const qrDataUrl = ref('')

const backupCodes = ref<string[]>([])
const backupError = ref('')
const backupSaving = ref(false)

// Rendered entirely client-side from the otpauth URL already in hand —
// the secret never leaves the browser for this.
watch(
  () => props.qrUrl,
  async (url) => {
    qrDataUrl.value = url ? await QRCode.toDataURL(url, { margin: 1, width: 200 }) : ''
  },
)

function reset() {
  phase.value = 'enroll'
  code.value = ''
  error.value = ''
  saving.value = false
  backupCodes.value = []
  backupError.value = ''
  backupSaving.value = false
}

async function cancel() {
  if (saving.value) return
  if (props.secret) {
    try {
      await cleanupTOTP(props.secret)
    } catch {
      // Best-effort cleanup — an abandoned, unconfirmed enrollment is a
      // known residual risk; nothing actionable to show the operator here.
    }
  }
  reset()
  emit('update:open', false)
}

function finish() {
  reset()
  emit('update:open', false)
}

function onBackdropClick(ev: MouseEvent) {
  if (ev.target !== ev.currentTarget) return
  if (phase.value === 'enroll') void cancel()
  else finish()
}

async function submitConfirm() {
  error.value = ''
  saving.value = true
  try {
    await confirmTOTP(props.password, code.value)
    phase.value = 'offer-backup'
    emit('enabled')
  } catch (e) {
    // Invalid code: secret/QR stay as-is for retry — do not clear them here.
    error.value = e instanceof Error ? e.message : 'Invalid code, try again.'
  } finally {
    saving.value = false
  }
}

async function submitGenerateBackupCodes() {
  backupError.value = ''
  backupSaving.value = true
  try {
    backupCodes.value = await regenerateBackupCodes(props.password)
    phase.value = 'backup-codes'
  } catch (e) {
    backupError.value = e instanceof Error ? e.message : 'Could not generate backup codes.'
  } finally {
    backupSaving.value = false
  }
}
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="modal-backdrop" @click="onBackdropClick">
      <div class="modal card stack" role="dialog" aria-modal="true" aria-labelledby="totp-enroll-title" @click.stop>
        <h2 id="totp-enroll-title" class="modal-title">Enable TOTP</h2>

        <template v-if="phase === 'enroll'">
          <p>Scan this in your authenticator app, or enter the secret manually:</p>
          <div class="totp-enroll">
            <img v-if="qrDataUrl" :src="qrDataUrl" width="160" height="160" alt="TOTP enrollment QR code" />
            <div class="stack-sm">
              <p class="totp-secret">{{ secret }}</p>
              <a class="text-muted" :href="qrUrl" target="_blank" rel="noopener noreferrer">
                Open in authenticator app
              </a>
            </div>
          </div>
          <label class="label">
            Code from your authenticator app
            <input
              v-model="code"
              class="input"
              type="text"
              inputmode="numeric"
              autocomplete="one-time-code"
              @keyup.enter="submitConfirm"
            />
          </label>
          <p v-if="error" class="status error">{{ error }}</p>
          <div class="form-actions">
            <button type="button" class="btn btn-ghost" :disabled="saving" @click="cancel">Cancel</button>
            <button type="button" class="btn btn-primary" :disabled="saving" @click="submitConfirm">
              {{ saving ? 'Confirming…' : 'Confirm' }}
            </button>
          </div>
        </template>

        <template v-else-if="phase === 'offer-backup'">
          <p class="status ok">TOTP enabled. Your account now requires a code at sign-in.</p>
          <p>
            Generate backup codes now — each works once and lets you sign in if you lose access to
            your authenticator app.
          </p>
          <p v-if="backupError" class="status error">{{ backupError }}</p>
          <div class="form-actions">
            <button type="button" class="btn btn-ghost" :disabled="backupSaving" @click="finish">Skip for now</button>
            <button type="button" class="btn btn-primary" :disabled="backupSaving" @click="submitGenerateBackupCodes">
              {{ backupSaving ? 'Generating…' : 'Generate backup codes' }}
            </button>
          </div>
        </template>

        <BackupCodesDisplay v-else :codes="backupCodes" @done="finish" />
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 100;
  display: grid;
  place-items: center;
  padding: var(--space-4);
  background: rgb(0 0 0 / 0.45);
}
.modal {
  width: min(100%, 28rem);
  min-height: 24rem;
  margin: 0;
}
.modal-title {
  margin: 0;
  font-family: var(--font-serif);
  font-size: var(--text-lg);
}
.totp-enroll {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  align-items: center;
  gap: var(--space-4);
}
.totp-enroll img {
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
}
.totp-secret {
  font-family: var(--font-mono, monospace);
  font-size: var(--text-lg);
  letter-spacing: 0.05em;
  word-break: break-all;
}
</style>
