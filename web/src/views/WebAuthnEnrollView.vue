<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { enrollWebAuthn } from '../lib/auth'

const route = useRoute()
const status = ref<'idle' | 'working' | 'ok' | 'error'>('idle')
const message = ref('')

onMounted(async () => {
  const token = String(route.query.token ?? '')
  if (!token) {
    status.value = 'error'
    message.value = 'Missing enrollment token.'
    return
  }
  status.value = 'working'
  const result = await enrollWebAuthn(token)
  if (result.ok) {
    status.value = 'ok'
    message.value = 'Passkey registered. You can close this tab.'
  } else {
    status.value = 'error'
    message.value = result.error ?? 'Registration failed.'
  }
})
</script>

<template>
  <header class="page-header">
    <h1>Register passkey</h1>
  </header>
  <p v-if="status === 'working'">Follow the browser prompt to register your passkey…</p>
  <p v-else :class="status">{{ message }}</p>
</template>

<style scoped>
.page-header {
  margin-bottom: 1rem;
}
.page-header h1 {
  font-family: var(--font-serif);
  font-size: 2rem;
  margin: 0;
}
.ok {
  color: var(--accent);
}
.error {
  color: var(--danger);
}
</style>
