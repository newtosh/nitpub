import { ref } from 'vue'
import { checkSession } from '../lib/auth'

const authed = ref(false)
const loaded = ref(false)

export function useSession() {
  async function refresh() {
    authed.value = await checkSession()
    loaded.value = true
    return authed.value
  }

  return { authed, loaded, refresh }
}
