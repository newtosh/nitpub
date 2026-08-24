<script setup lang="ts">
import { watch } from 'vue'

const props = withDefaults(
  defineProps<{
    open: boolean
    title: string
    message: string
    confirmLabel?: string
    cancelLabel?: string
    danger?: boolean
    busy?: boolean
  }>(),
  {
    confirmLabel: 'Confirm',
    cancelLabel: 'Cancel',
    danger: false,
    busy: false,
  },
)

const emit = defineEmits<{
  'update:open': [value: boolean]
  confirm: []
}>()

function close() {
  if (props.busy) return
  emit('update:open', false)
}

function onConfirm() {
  if (props.busy) return
  emit('confirm')
}

function onBackdropClick(ev: MouseEvent) {
  if (ev.target === ev.currentTarget) close()
}

watch(
  () => props.open,
  (isOpen) => {
    document.body.style.overflow = isOpen ? 'hidden' : ''
  },
)
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="modal-backdrop" @click="onBackdropClick">
      <div
        class="modal card"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="title.replace(/\s+/g, '-').toLowerCase()"
        @click.stop
      >
        <h2 :id="title.replace(/\s+/g, '-').toLowerCase()" class="modal-title">{{ title }}</h2>
        <p class="modal-message">{{ message }}</p>
        <div class="form-actions">
          <button type="button" class="btn btn-ghost" :disabled="busy" @click="close">
            {{ cancelLabel }}
          </button>
          <button
            type="button"
            class="btn"
            :class="danger ? 'btn-danger' : 'btn-primary'"
            :disabled="busy"
            @click="onConfirm"
          >
            {{ busy ? 'Working…' : confirmLabel }}
          </button>
        </div>
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
  width: min(100%, 26rem);
  margin: 0;
}
.modal-title {
  margin: 0 0 var(--space-2);
  font-family: var(--font-serif);
  font-size: var(--text-lg);
}
.modal-message {
  margin: 0 0 var(--space-5);
  color: var(--muted);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
}
</style>
