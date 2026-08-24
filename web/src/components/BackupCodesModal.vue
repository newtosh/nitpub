<script setup lang="ts">
import BackupCodesDisplay from './BackupCodesDisplay.vue'

defineProps<{ open: boolean; codes: string[] }>()
const emit = defineEmits<{ 'update:open': [value: boolean] }>()

function close() {
  emit('update:open', false)
}

function onBackdropClick(ev: MouseEvent) {
  if (ev.target === ev.currentTarget) close()
}
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="modal-backdrop" @click="onBackdropClick">
      <div class="modal card stack" role="dialog" aria-modal="true" aria-labelledby="backup-codes-title" @click.stop>
        <h2 id="backup-codes-title" class="modal-title">Backup codes</h2>
        <BackupCodesDisplay :codes="codes" @done="close" />
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
</style>
