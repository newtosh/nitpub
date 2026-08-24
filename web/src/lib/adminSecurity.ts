async function postSecurity<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`/api/admin/security/${path}`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (res.status === 429) {
    throw new Error('Too many attempts, try again shortly.')
  }
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || 'Request failed')
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

export async function changePassword(currentPassword: string, newPassword: string): Promise<void> {
  await postSecurity('password', { current_password: currentPassword, new_password: newPassword })
}

export type TOTPEnableResult = { secret: string; url: string }

export async function enableTOTP(currentPassword: string): Promise<TOTPEnableResult> {
  return postSecurity('totp/enable', { current_password: currentPassword })
}

export async function confirmTOTP(currentPassword: string, code: string): Promise<void> {
  await postSecurity('totp/confirm', { current_password: currentPassword, code })
}

export async function disableTOTP(currentPassword: string): Promise<void> {
  await postSecurity('totp/disable', { current_password: currentPassword })
}

export async function cleanupTOTP(secret: string): Promise<void> {
  await postSecurity('totp/cleanup', { secret })
}

export async function regenerateBackupCodes(currentPassword: string): Promise<string[]> {
  const res = await postSecurity<{ codes: string[] }>('backup-codes/regenerate', {
    current_password: currentPassword,
  })
  return res.codes
}

export async function passkeyEnrollLink(currentPassword: string): Promise<string> {
  const res = await postSecurity<{ url: string }>('passkey/enroll-link', {
    current_password: currentPassword,
  })
  return res.url
}

export async function disablePasskey(currentPassword: string): Promise<void> {
  await postSecurity('passkey/disable', { current_password: currentPassword })
}
