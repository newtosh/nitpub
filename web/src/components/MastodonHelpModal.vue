<script setup lang="ts">
defineProps<{ open: boolean }>()
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
      <div class="modal card stack" role="dialog" aria-modal="true" aria-labelledby="mastodon-help-title" @click.stop>
        <h2 id="mastodon-help-title" class="modal-title">Comments are powered by Mastodon</h2>
        <p>
          Mastodon is part of the fediverse — a network of independently run social sites that all speak the same
          open protocol, so accounts on different sites can follow and reply to each other, the same way email
          works across providers.
        </p>
        <p>
          Comments here work the same way: when you sign in with your Mastodon account, your comment is posted as a
          real reply from your account, and it's moderated the same as any other reply before it appears — there's
          no separate comment system to trust with your data.
        </p>
        <p>
          Don't have an account yet? Pick a server at
          <a href="https://joinmastodon.org/servers" target="_blank" rel="noopener noreferrer">joinmastodon.org/servers</a>
          — any of them will work here.
        </p>
        <div class="form-actions">
          <button type="button" class="btn btn-primary" @click="close">Got it</button>
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
  width: min(100%, 28rem);
  margin: 0;
}
.modal-title {
  margin: 0;
  font-family: var(--font-serif);
  font-size: var(--text-lg);
}
.form-actions {
  display: flex;
  justify-content: flex-end;
}
</style>
