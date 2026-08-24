<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import ModerationQueue from '../components/ModerationQueue.vue'
import { useSession } from '../composables/useSession'

const { authed, refresh } = useSession()

onMounted(() => refresh())
</script>

<template>
  <header class="page-header">
    <h1>Moderation</h1>
    <p class="text-muted">Review pending replies and manage trusted/blocked actors.</p>
  </header>

  <section v-if="!authed" class="card stack">
    <p>Sign in to manage moderation.</p>
    <div class="form-actions">
      <RouterLink class="btn btn-primary" to="/login" title="Sign in" aria-label="Sign in">
        Sign in
      </RouterLink>
    </div>
  </section>

  <article v-else class="card stack">
    <ModerationQueue />
  </article>
</template>

<style scoped>
@media (max-width: 47.99rem) {
  .page-header {
    display: none;
  }
}
</style>
