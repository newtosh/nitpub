export type VersionCheck = {
  current: string
  latest?: string
  latest_url?: string
  update_available: boolean
  check_error?: string
}

export async function fetchAdminVersion(): Promise<VersionCheck> {
  const res = await fetch('/api/admin/version', { credentials: 'include' })
  if (!res.ok) throw new Error('Failed to check version')
  return res.json()
}
