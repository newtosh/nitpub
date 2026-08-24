import { ref } from 'vue'
import { fetchPendingReplies } from '../lib/moderationAdmin'

const pendingCount = ref(0)

export function useModerationBadge() {
  async function refresh() {
    try {
      const pending = await fetchPendingReplies()
      pendingCount.value = pending.length
    } catch {
      pendingCount.value = 0
    }
  }

  function setCount(n: number) {
    pendingCount.value = n
  }

  return { pendingCount, refresh, setCount }
}
