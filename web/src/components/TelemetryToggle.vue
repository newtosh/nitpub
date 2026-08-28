<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { fetchTelemetryStatus, setTelemetryEnabled, type TelemetryStatus } from '../lib/telemetry'

const status = ref<TelemetryStatus | null>(null)
const loading = ref(true)
const saving = ref(false)
const error = ref('')

onMounted(async () => {
  try {
    status.value = await fetchTelemetryStatus()
  } catch {
    error.value = 'Could not load telemetry status.'
  } finally {
    loading.value = false
  }
})

async function onChange(e: Event) {
  const next = (e.target as HTMLInputElement).checked
  saving.value = true
  error.value = ''
  try {
    status.value = await setTelemetryEnabled(next)
  } catch (err) {
    // status is untouched on failure, so the checkbox falls back to the
    // last confirmed state on its own — it must not appear to have
    // succeeded when the POST failed (e.g. registration error).
    error.value = err instanceof Error ? err.message : 'Failed to update telemetry setting.'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="telemetry-toggle">
    <h3 class="section-title">Telemetry</h3>
    <p class="text-muted">
      Opt in to send this instance's version, a random instance ID, and OS/arch to the nitpub
      project's telemetry collector — off by default, no data leaves this instance without this.
    </p>
    <label class="field checkbox">
      <input
        type="checkbox"
        :checked="status?.enabled ?? false"
        :disabled="!status || saving"
        @change="onChange"
      />
      <span>{{
        loading
          ? 'Loading…'
          : saving
            ? 'Updating…'
            : !status
              ? 'Status unavailable'
              : status.enabled
                ? 'Telemetry enabled'
                : 'Telemetry disabled'
      }}</span>
    </label>
    <p v-if="error" class="status error">{{ error }}</p>
  </div>
</template>

<style scoped>
.telemetry-toggle {
  display: grid;
  gap: var(--space-2);
}
.section-title {
  margin: var(--space-4) 0 var(--space-2);
  font-size: var(--text-base);
}
.section-title:first-child {
  margin-top: 0;
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
.status {
  margin: 0;
}
.status.error {
  color: var(--danger);
}
</style>
