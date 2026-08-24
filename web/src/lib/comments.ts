export type CommentSession = {
  instance: string
  handle: string
  display_name?: string
  avatar_url?: string
}

export async function fetchCommentSession(): Promise<CommentSession | null> {
  const res = await fetch('/api/comments/session', { credentials: 'include' })
  if (res.status === 204) return null
  if (!res.ok) throw new Error('Failed to load comment session')
  return res.json()
}

export async function logoutComment(): Promise<void> {
  const res = await fetch('/api/comments/logout', { method: 'POST', credentials: 'include' })
  if (!res.ok) throw new Error('Failed to log out')
}

// startCommentAuth begins the OAuth flow (KTD7: JSON POST, not a bare GET,
// so a cross-site page can't trigger it blind). Usually navigates the
// browser to the returned authorize URL — a real full-page redirect (R2),
// not something fetch() itself can follow across origins — but returns
// 'posted' instead of navigating when the server had a still-valid cached
// token (TokenCookie, up to 24h) and posted the comment directly with no
// OAuth round-trip at all.
export async function startCommentAuth(
  postSlug: string,
  instance: string,
  draftText: string,
): Promise<'posted' | 'redirected'> {
  const res = await fetch('/api/comments/oauth/start', {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ post_slug: postSlug, instance, draft_text: draftText }),
  })
  if (!res.ok) {
    throw new Error(res.status === 404 ? 'Post not found.' : "Couldn't reach that instance.")
  }
  const data = (await res.json()) as { redirect_url?: string; posted?: boolean }
  if (data.posted) return 'posted'
  window.location.href = data.redirect_url!
  return 'redirected'
}

export type CommentReturnStatus =
  | 'success'
  | 'signed_in'
  | 'error_auth'
  | 'error_instance'
  | 'error_expired'
  | null

// readCommentReturnState reads the ?comment=...&draft=... query params
// CommentAuthCallback appends on redirect back, without disturbing the
// rest of the URL/hash (e.g. #replies).
export function readCommentReturnState(): { status: CommentReturnStatus; draft: string } {
  const params = new URLSearchParams(window.location.search)
  const status = params.get('comment') as CommentReturnStatus
  const draft = params.get('draft') ?? ''
  return { status, draft }
}

// clearCommentReturnState strips the comment/draft query params from the
// URL bar after they've been read, so a page refresh doesn't replay them.
export function clearCommentReturnState(): void {
  const url = new URL(window.location.href)
  url.searchParams.delete('comment')
  url.searchParams.delete('draft')
  window.history.replaceState(null, '', url.pathname + url.search + url.hash)
}
