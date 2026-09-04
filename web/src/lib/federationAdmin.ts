export type FederationInfo = {
  actor: string
  domain: string
  acct: string
  actor_url: string
  follower_count: number
  follow_policy: 'open'
  followers_open: boolean
}

export async function fetchFederationInfo(): Promise<FederationInfo> {
  const res = await fetch('/api/admin/federation', { credentials: 'include' })
  if (!res.ok) throw new Error('Failed to load federation info')
  return res.json()
}

export type FederationDelivery = {
  slug: string
  kind: string
  created_at: string
  status: 'delivered' | 'pending' | 'error'
  error?: string
  shared_at?: string
}

export type BackfillResult = {
  sent: number
  skipped: number
  errors?: string[]
}

async function unwrap<T>(res: Response, failMessage: string): Promise<T> {
  if (!res.ok) throw new Error(failMessage)
  return res.json()
}

export async function fetchFederationDeliveries(
  limit?: number,
  offset?: number,
): Promise<{ deliveries: FederationDelivery[]; total: number }> {
  const params = new URLSearchParams()
  if (limit !== undefined) params.set('limit', String(limit))
  if (offset !== undefined) params.set('offset', String(offset))
  const qs = params.toString()
  const res = await fetch(`/api/admin/federation/deliveries${qs ? `?${qs}` : ''}`, {
    credentials: 'include',
  })
  return unwrap(res, 'Failed to load delivery log')
}

async function postFederationAction<T>(path: string, failMessage: string): Promise<T> {
  const res = await fetch(`/api/admin/federation/${path}`, {
    method: 'POST',
    credentials: 'include',
  })
  return unwrap(res, failMessage)
}

export const resendAccepts = () =>
  postFederationAction<{ sent: number }>('resend-accepts', 'Failed to resend accepts')
export const backfillFederation = () =>
  postFederationAction<BackfillResult>('backfill', 'Failed to backfill federation')
export const redeliverShared = () =>
  postFederationAction<BackfillResult>('redeliver-shared', 'Failed to redeliver shared posts')

export type ReferenceStatus = {
  connected: boolean
  instance?: string
}

export async function fetchReferenceStatus(): Promise<ReferenceStatus> {
  const res = await fetch('/api/admin/federation/reference/status', { credentials: 'include' })
  return unwrap(res, 'Failed to load reference instance status')
}

export const startReferenceConnect = () =>
  postFederationAction<{ redirect_url: string }>('reference/connect', 'Failed to start connect')
export const resolveReferencePermalinks = () =>
  postFederationAction<BackfillResult>('reference/resolve', 'Failed to resolve permalinks')

export async function disconnectReference(): Promise<void> {
  const res = await fetch('/api/admin/federation/reference/disconnect', {
    method: 'POST',
    credentials: 'include',
  })
  if (!res.ok) throw new Error('Failed to disconnect')
}

export type BlueskyStatus = {
  connected: boolean
  handle: string
  needs_reconnect: boolean
}

export async function fetchBlueskyStatus(): Promise<BlueskyStatus> {
  const res = await fetch('/api/admin/bluesky/status', { credentials: 'include' })
  return unwrap(res, 'Failed to load Bluesky status')
}

export async function connectBluesky(
  handle: string,
  appPassword: string,
): Promise<{ connected: true; handle: string }> {
  const res = await fetch('/api/admin/bluesky/connect', {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ handle, app_password: appPassword }),
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || 'Failed to connect — check the handle and app password')
  }
  return res.json()
}

export async function disconnectBluesky(): Promise<void> {
  const res = await fetch('/api/admin/bluesky/connect', {
    method: 'DELETE',
    credentials: 'include',
  })
  if (!res.ok) throw new Error('Failed to disconnect')
}
