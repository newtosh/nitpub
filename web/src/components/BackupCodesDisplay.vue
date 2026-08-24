<script setup lang="ts">
import { ref } from 'vue'

const props = defineProps<{ codes: string[] }>()
defineEmits<{ done: [] }>()

const copied = ref(false)

async function copyCodes() {
  await navigator.clipboard.writeText(props.codes.join('\n'))
  copied.value = true
  setTimeout(() => {
    copied.value = false
  }, 1500)
}

function downloadCodes() {
  const blob = new Blob([props.codes.join('\n') + '\n'], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'nitpub-backup-codes.txt'
  a.click()
  URL.revokeObjectURL(url)
}
</script>

<template>
  <div class="backup-codes-body">
    <p>Save these now — they won't be shown again. Each works once.</p>
    <pre class="backup-codes">{{ codes.join('\n') }}</pre>
  </div>
  <div class="form-actions">
    <button type="button" class="btn btn-ghost" @click="copyCodes">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <rect x="9" y="9" width="13" height="13" rx="2" />
        <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
      </svg>
      {{ copied ? 'Copied!' : 'Copy' }}
    </button>
    <button type="button" class="btn btn-ghost" @click="downloadCodes">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
        <polyline points="7 10 12 15 17 10" />
        <line x1="12" y1="15" x2="12" y2="3" />
      </svg>
      Download
    </button>
    <button type="button" class="btn btn-primary" @click="$emit('done')">Done</button>
  </div>
</template>

<style scoped>
.backup-codes-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: var(--space-3);
}
.backup-codes {
  font-family: var(--font-mono, monospace);
  padding: var(--space-3);
  background: var(--surface-2, var(--surface));
  border-radius: var(--radius-md);
  white-space: pre-wrap;
  text-align: center;
  user-select: all;
}
</style>
