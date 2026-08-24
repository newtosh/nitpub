export type AuthSettings = {
  totp_enabled: boolean
  webauthn_enabled: boolean
  theme_id?: string
  flags?: Record<string, boolean>
}

export type Appearance = {
  theme_id: string
}

export type LoginResult =
  | { ok: true }
  | { ok: false; needs2FA: true; methods: string[]; pendingToken: string }
  | { ok: false; needs2FA: false; error: string }

export async function checkSession(): Promise<boolean> {
  const res = await fetch('/api/auth/session', { credentials: 'include' })
  if (!res.ok) return false
  const data = (await res.json()) as { authenticated: boolean }
  return data.authenticated
}

export async function login(
  username: string,
  password: string,
  rememberMe = false,
): Promise<LoginResult> {
  const res = await fetch('/api/auth/login', {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password, remember_me: rememberMe }),
  })
  if (res.status === 204) {
    return { ok: true }
  }
  if (res.status === 401) {
    return { ok: false, needs2FA: false, error: 'Invalid username or password' }
  }
  if (res.ok) {
    const data = (await res.json()) as { status: string; methods?: string[]; pending_token?: string }
    if (data.status === '2fa_required' && data.pending_token) {
      return {
        ok: false,
        needs2FA: true,
        methods: data.methods ?? [],
        pendingToken: data.pending_token,
      }
    }
  }
  return { ok: false, needs2FA: false, error: 'Login failed' }
}

export async function verify2FA(
  pendingToken: string,
  method: string,
  code: string,
  rememberMe = false,
): Promise<{ ok: boolean; error?: string }> {
  const res = await fetch('/api/auth/verify', {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      pending_token: pendingToken,
      method,
      code,
      remember_me: rememberMe,
    }),
  })
  if (res.status === 204) return { ok: true }
  if (res.status === 429) {
    return { ok: false, error: 'Too many attempts. Wait a few minutes and try again.' }
  }
  return { ok: false, error: 'Verification failed' }
}

export async function webauthnLogin(
  pendingToken: string,
  rememberMe = false,
): Promise<{ ok: boolean; error?: string }> {
  const begin = await fetch('/api/auth/webauthn/login/begin', {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ pending_token: pendingToken }),
  })
  if (!begin.ok) return { ok: false, error: 'WebAuthn unavailable' }
  // go-webauthn's CredentialAssertion serializes as { publicKey: {...} }.
  const options = await begin.json()
  const assertion = await navigator.credentials.get({
    publicKey: PublicKeyCredential.parseRequestOptionsFromJSON(options.publicKey),
  })
  if (!assertion) return { ok: false, error: 'WebAuthn cancelled' }
  const rememberQuery = rememberMe ? '&remember_me=1' : ''
  const finish = await fetch(
    `/api/auth/webauthn/login/finish?pending=${encodeURIComponent(pendingToken)}${rememberQuery}`,
    {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      // PublicKeyCredential's binary fields (rawId, response.*) aren't
      // JSON-serializable directly — toJSON() base64url-encodes them into
      // the shape go-webauthn expects.
      body: JSON.stringify((assertion as PublicKeyCredential).toJSON()),
    },
  )
  if (finish.status === 204) return { ok: true }
  return { ok: false, error: 'WebAuthn verification failed' }
}

export async function logout(): Promise<void> {
  await fetch('/api/auth/logout', { method: 'POST', credentials: 'include' })
}

export async function fetchSettings(): Promise<AuthSettings | null> {
  const res = await fetch('/api/admin/settings', { credentials: 'include' })
  if (!res.ok) return null
  return (await res.json()) as AuthSettings
}

export async function fetchAppearance(): Promise<Appearance | null> {
  const res = await fetch('/api/appearance')
  if (!res.ok) return null
  return (await res.json()) as Appearance
}

export async function saveTheme(themeId: string): Promise<{ ok: boolean; error?: string }> {
  const res = await fetch('/api/admin/settings', {
    method: 'PUT',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ theme_id: themeId }),
  })
  if (!res.ok) {
    const text = await res.text()
    return { ok: false, error: text || 'Save failed' }
  }
  return { ok: true }
}

export async function enrollWebAuthn(token: string): Promise<{ ok: boolean; error?: string }> {
  const begin = await fetch('/api/auth/webauthn/register/begin', {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token }),
  })
  if (!begin.ok) return { ok: false, error: 'Invalid or expired enrollment link' }
  // go-webauthn's CredentialCreation serializes as { publicKey: {...} }.
  const options = await begin.json()
  const cred = await navigator.credentials.create({
    publicKey: PublicKeyCredential.parseCreationOptionsFromJSON(options.publicKey),
  })
  if (!cred) return { ok: false, error: 'Registration cancelled' }
  const finish = await fetch(
    `/api/auth/webauthn/register/finish?token=${encodeURIComponent(token)}`,
    {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      // PublicKeyCredential's binary fields (rawId, response.*) aren't
      // JSON-serializable directly — toJSON() base64url-encodes them into
      // the shape go-webauthn expects.
      body: JSON.stringify((cred as PublicKeyCredential).toJSON()),
    },
  )
  if (finish.status === 204) return { ok: true }
  return { ok: false, error: 'Registration failed' }
}
