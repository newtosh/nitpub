export type PendingReply = {
  key: string
  activity_id: string
  post_slug: string
  actor: string
  content: string
  author_name?: string
  url?: string
  avatar_url?: string
  // True when this reply targets another reply rather than the post itself
  // (a nested reply-to-a-reply).
  nested?: boolean
  // The immediate parent reply's identity, set only when nested — who this
  // reply is actually addressing.
  parent_actor?: string
  parent_author_name?: string
  received_at?: string
  status: 'pending' | 'approved' | 'rejected' | 'skipped'
  // false for entries migrated by the one-time backfill, whose raw stored
  // activity never carried an HTTP-signature-verified actor identity.
  verified: boolean
}

async function unwrap<T>(res: Response, failMessage: string): Promise<T> {
  if (!res.ok) throw new Error(failMessage)
  return res.json()
}

export async function fetchPendingReplies(): Promise<PendingReply[]> {
  const res = await fetch('/api/admin/replies', { credentials: 'include' })
  return unwrap(res, 'Failed to load pending replies')
}

/** Every already-actioned reply (approved, rejected, or skipped) — the
 * "Reviewed" queue view, from which a decision can be reverted. */
export async function fetchReviewedReplies(): Promise<PendingReply[]> {
  const res = await fetch('/api/admin/replies/reviewed', { credentials: 'include' })
  return unwrap(res, 'Failed to load reviewed replies')
}

async function postReplyAction(key: string, action: string, failMessage: string): Promise<void> {
  const res = await fetch(`/api/admin/replies/${encodeURIComponent(key)}/${action}`, {
    method: 'POST',
    credentials: 'include',
  })
  if (!res.ok) throw new Error(failMessage)
}

export const approveReply = (key: string) => postReplyAction(key, 'approve', 'Failed to approve reply')
export const rejectReply = (key: string) => postReplyAction(key, 'reject', 'Failed to reject reply')
export const skipReply = (key: string) => postReplyAction(key, 'skip', 'Failed to skip reply')
export const revertReply = (key: string) => postReplyAction(key, 'revert', 'Failed to revert reply')

async function fetchActorList(list: 'trusted' | 'blocked'): Promise<string[]> {
  const res = await fetch(`/api/admin/moderation/${list}`, { credentials: 'include' })
  return unwrap(res, `Failed to load ${list} actors`)
}

async function addActor(list: 'trusted' | 'blocked', actor: string): Promise<void> {
  const res = await fetch(`/api/admin/moderation/${list}`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ actor }),
  })
  if (!res.ok) throw new Error(`Failed to add ${list} actor`)
}

async function removeActor(list: 'trusted' | 'blocked', actor: string): Promise<void> {
  const res = await fetch(`/api/admin/moderation/${list}/${encodeURIComponent(actor)}`, {
    method: 'DELETE',
    credentials: 'include',
  })
  if (!res.ok) throw new Error(`Failed to remove ${list} actor`)
}

export const fetchTrustedActors = () => fetchActorList('trusted')
export const addTrustedActor = (actor: string) => addActor('trusted', actor)
export const removeTrustedActor = (actor: string) => removeActor('trusted', actor)

export const fetchBlockedActors = () => fetchActorList('blocked')
export const addBlockedActor = (actor: string) => addActor('blocked', actor)
export const removeBlockedActor = (actor: string) => removeActor('blocked', actor)
