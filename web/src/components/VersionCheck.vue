<script setup lang="ts">
import { ref } from 'vue'
import { fetchAdminVersion, type VersionCheck } from '../lib/version'

const result = ref<VersionCheck | null>(null)
const checking = ref(false)
const error = ref('')

async function check() {
  checking.value = true
  error.value = ''
  try {
    result.value = await fetchAdminVersion()
  } catch {
    error.value = 'Could not check for updates.'
  } finally {
    checking.value = false
  }
}
</script>

<template>
  <div class="version-check">
    <p v-if="!result" class="text-muted">
      Check the running build against the latest version published on GitHub.
    </p>
    <p v-else-if="result.update_available" class="status update-available">
      Update available: <strong>{{ result.current }} → {{ result.latest }}</strong>
      <a v-if="result.latest_url" :href="result.latest_url" target="_blank" rel="noopener noreferrer">
        release notes
      </a>
    </p>
    <p v-else class="status">
      Up to date (<strong>{{ result.current }}</strong>).
    </p>
    <p v-if="result?.check_error" class="status error">{{ result.check_error }}</p>
    <p v-if="error" class="status error">{{ error }}</p>
    <p v-if="result?.update_available" class="text-muted update-hint">
      Run <code>nitpub update --apply</code> on the server to install it (see
      <a href="https://github.com/newtosh/nitpub/blob/main/deploy/README.md" target="_blank" rel="noopener noreferrer"
        >deploy/README.md</a
      >).
    </p>
    <div class="form-actions">
      <button type="button" class="btn btn-ghost" :disabled="checking" @click="check">
        {{ checking ? 'Checking…' : 'Check for updates' }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.version-check {
  display: grid;
  gap: var(--space-2);
}
.status {
  margin: 0;
}
.status.error {
  color: var(--danger);
}
.update-available {
  color: var(--accent);
}
.update-available a {
  margin-left: var(--space-2);
  font-size: var(--text-sm);
}
.update-hint code {
  font-size: 0.85em;
}
</style>
