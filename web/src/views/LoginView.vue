<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { login, verify2FA, webauthnLogin } from '../lib/auth'
import { loadRememberMePreference, saveRememberMePreference } from '../lib/remember-me'
import { useSession } from '../composables/useSession'

const route = useRoute()
const router = useRouter()
const { refresh } = useSession()

const error = ref('')
const username = ref('')
const password = ref('')
const rememberMe = ref(false)
const step = ref<'login' | '2fa'>('login')
const pendingToken = ref('')
const methods = ref<string[]>([])
const totpCode = ref('')

function redirectTarget(): string {
  const raw = route.query.redirect
  if (typeof raw === 'string' && raw.startsWith('/') && !raw.startsWith('//')) {
    return raw
  }
  return '/author'
}

async function afterAuth() {
  // Blur before navigating, not after: the username/password field is
  // still focused when login succeeds, and router.replace() swaps the
  // whole page's DOM out from under it. If the on-screen keyboard is
  // still mid-dismiss-animation when that happens, iOS Safari's visual
  // viewport gets permanently stuck offset from the (correctly at 0)
  // layout scroll position — every route after that renders "scrolled
  // down" until the user manually scrolls. Blurring first lets the
  // keyboard start dismissing against the still-current page.
  ;(document.activeElement as HTMLElement | null)?.blur()
  await refresh()
  await router.replace(redirectTarget())
}

async function doLogin() {
  error.value = ''
  if (!username.value.trim() || !password.value) {
    return
  }
  saveRememberMePreference(rememberMe.value)
  const result = await login(username.value, password.value, rememberMe.value)
  if (result.ok) {
    step.value = 'login'
    await afterAuth()
    return
  }
  if (result.needs2FA) {
    // Same iOS Safari visual-viewport-stuck issue as afterAuth() below —
    // this swap replaces the username/password fields with the 2FA form
    // while the keyboard may still be up, and it's an in-place re-render
    // with no route change at all, so router-level fixes don't reach it.
    ;(document.activeElement as HTMLElement | null)?.blur()
    step.value = '2fa'
    pendingToken.value = result.pendingToken
    methods.value = result.methods
    return
  }
  error.value = result.error ?? 'Login failed'
}

async function doVerifyTotp() {
  error.value = ''
  const result = await verify2FA(pendingToken.value, 'totp', totpCode.value, rememberMe.value)
  if (result.ok) {
    step.value = 'login'
    await afterAuth()
    return
  }
  error.value = result.error ?? 'Invalid code'
}

async function doWebAuthn() {
  error.value = ''
  const result = await webauthnLogin(pendingToken.value, rememberMe.value)
  if (result.ok) {
    step.value = 'login'
    await afterAuth()
    return
  }
  error.value = result.error ?? 'Passkey sign-in failed'
}

onMounted(() => {
  rememberMe.value = loadRememberMePreference()
})
</script>

<template>
  <header class="page-header">
    <h1>Sign in</h1>
    <p class="text-muted">Access the author workspace for this instance.</p>
  </header>

  <form v-if="step === 'login'" class="login card stack" @submit.prevent="doLogin">
    <label class="label">
      Username
      <input
        v-model="username"
        class="input"
        type="text"
        name="username"
        autocomplete="username"
        required
      />
    </label>
    <label class="label">
      Password
      <input
        v-model="password"
        class="input"
        type="password"
        name="password"
        autocomplete="current-password"
        required
      />
    </label>
    <label class="remember-me">
      <input v-model="rememberMe" type="checkbox" name="remember_me" />
      Remember me on this device
    </label>
    <button type="submit" class="btn btn-primary" title="Sign in">Sign in</button>
  </form>

  <section v-else class="login card stack">
    <h2>Second factor</h2>
    <template v-if="methods.includes('totp')">
      <label class="label">
        Authenticator code
        <input
          v-model="totpCode"
          class="input"
          type="text"
          inputmode="numeric"
          autocomplete="one-time-code"
          @keyup.enter="totpCode.length >= 6 && doVerifyTotp()"
        />
      </label>
      <button type="button" class="btn btn-primary" title="Verify authenticator code" @click="doVerifyTotp">
        Verify code
      </button>
    </template>
    <button
      v-if="methods.includes('webauthn')"
      type="button"
      class="btn btn-primary"
      title="Sign in with passkey"
      @click="doWebAuthn"
    >
      Use passkey
    </button>
    <button type="button" class="btn btn-ghost" title="Back to sign in" @click="step = 'login'">Back</button>
  </section>

  <p v-if="error" class="alert alert-error">{{ error }}</p>
</template>

<style scoped>
.page-header h1 {
  margin: 0;
}
.login h2 {
  margin: 0;
  font-size: var(--text-lg);
}
.remember-me {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--text-sm);
  font-weight: 400;
  color: var(--muted);
  cursor: pointer;
}
.remember-me input {
  margin: 0;
  accent-color: var(--accent);
}
</style>
