export type Post = {
  id: string
  kind: string
  title?: string | null
  content: string
  status?: 'draft' | 'published'
  created_at: string
  updated_at?: string
  federation?: {
    shared: boolean
    shared_at?: string
    error?: string
    remote_url?: string
  }
  reply_count?: number
  // Raw structured input for a kind:"quote" post (internal/outbox.QuoteFields
  // mirror) — undefined for every other kind.
  quote?: {
    source_url: string
    title?: string
    excerpt: string
    commentary?: string
    via?: string
  }
}

import { plainTextFromMarkdown, stripTitleMarker } from './markdown'
import { hasLinkCardLine, LINK_CARD_LINE } from './linkCard'

/** First markdown image in source, e.g. `![alt](url)`. */
export const IMAGE_MARKDOWN = /!\[[^\]]*\]\([^)]+\)/

export function hasPreviewImage(markdown: string): boolean {
  return IMAGE_MARKDOWN.test(markdown)
}

/** Remove GitHub alert callouts (`> [!NOTE]` blocks) from feed preview markdown. */
export function stripCallouts(markdown: string): string {
  const lines = markdown.split('\n')
  const out: string[] = []
  let i = 0
  while (i < lines.length) {
    const line = lines[i]
    if (/^>\s*\[!\w+\]/i.test(line)) {
      i++
      while (i < lines.length && /^>/.test(lines[i])) i++
      continue
    }
    out.push(line)
    i++
  }
  return out.join('\n').replace(/\n{3,}/g, '\n\n').trim()
}

/** Trim feed previews before code fences, rich blocks, and length limits. */
export function truncateForFeed(markdown: string, maxLen: number): string {
  const trimmed = markdown.trim()
  if (!trimmed) return ''

  let slice = trimmed
  const richBoundary = findRichBlockBoundary(slice)
  if (richBoundary >= 0) {
    slice = slice.slice(0, richBoundary).trimEnd()
  }

  if (slice.length <= maxLen) return slice

  let cut = slice.slice(0, maxLen)
  const lastPara = cut.lastIndexOf('\n\n')
  if (lastPara > maxLen * 0.45) {
    cut = cut.slice(0, lastPara)
  } else {
    const lastSpace = cut.lastIndexOf(' ')
    if (lastSpace > maxLen * 0.6) cut = cut.slice(0, lastSpace)
  }
  return cut.trimEnd() + '…'
}

/** Index of the first rich block after introductory text, or -1. */
function findRichBlockBoundary(markdown: string): number {
  const patterns = [
    /\n```/, // code fence
    /\n!\[/, // image
    /(?:^|\n)> ?\[!\w+]/i, // GitHub alert callout
    /\n>/, // blockquote
    /\n#{1,6}\s+\S/, // heading after first line
  ]
  let boundary = -1
  for (const pattern of patterns) {
    const idx = markdown.search(pattern)
    if (idx > 0) {
      boundary = boundary < 0 ? idx : Math.min(boundary, idx)
    }
  }
  return boundary
}

export function noteHasTitle(content: string): boolean {
  const first = content.split('\n')[0]?.trim() ?? ''
  return /^#{1,6}\s+\S/.test(first)
}

export function noteTitle(content: string): string {
  const first = content.split('\n')[0]?.trim() ?? ''
  if (!/^#{1,6}\s+\S/.test(first)) return ''
  return stripTitleMarker(first)
}

export function noteBody(content: string): string {
  const lines = content.split('\n')
  const first = lines[0]?.trim() ?? ''
  if (/^#{1,6}\s+\S/.test(first)) {
    return lines.length > 1 ? lines.slice(1).join('\n').trimStart() : ''
  }
  return content
}

export function postSlug(id: string): string {
  const i = id.lastIndexOf('/')
  return i >= 0 ? id.slice(i + 1) : id
}

export function articleTitle(content: string): string {
  const line = content.split('\n')[0]?.trim() ?? ''
  const stripped = stripTitleMarker(line)
  return stripped || 'Article'
}

export function articleBody(content: string): string {
  const lines = content.split('\n')
  if (lines.length <= 1) return ''
  return lines.slice(1).join('\n').trim()
}

export function postExcerpt(post: Post, maxLen = 160): string {
  const source =
    post.kind === 'article'
      ? articleBody(post.content) || articleTitle(post.content)
      : post.content
  return plainTextFromMarkdown(source, maxLen)
}

/** Feed preview that keeps link cards and hero images visible on the index. */
function feedPreviewMarkdown(markdown: string, maxLen: number): string {
  const trimmed = stripCallouts(markdown).trim()
  if (!trimmed) return ''

  const cardMatch = trimmed.match(LINK_CARD_LINE)
  if (cardMatch && cardMatch.index !== undefined) {
    const cardLine = cardMatch[0].trim()
    const beforeCard = trimmed.slice(0, cardMatch.index).trimEnd()
    const intro = beforeCard ? truncateForFeed(beforeCard, maxLen) : ''
    return intro ? `${intro}\n\n${cardLine}` : cardLine
  }

  const imageMatch = trimmed.match(IMAGE_MARKDOWN)
  if (imageMatch && imageMatch.index !== undefined) {
    const imageMd = imageMatch[0]
    const beforeImage = trimmed.slice(0, imageMatch.index).trimEnd()
    const intro = beforeImage ? truncateForFeed(beforeImage, maxLen) : ''
    return intro ? `${intro}\n\n${imageMd}` : imageMd
  }

  return truncateForFeed(trimmed, maxLen)
}

/** Markdown preview for note cards on the index (text-first; stops before rich blocks). */
export function notePreviewMarkdown(post: Post, maxLen = 280): string {
  const source = noteBody(post.content).trim() || post.content.trim()
  return feedPreviewMarkdown(source, maxLen)
}

/** Markdown preview for article cards — intro only, stops before code blocks. */
export function articlePreviewMarkdown(post: Post, maxLen = 480): string {
  const body = articleBody(post.content) || articleTitle(post.content)
  return feedPreviewMarkdown(body, maxLen)
}

export function noteIsTruncatedInPreview(post: Post, maxLen = 280): boolean {
  const source = (noteBody(post.content) || post.content).trim()
  if (hasLinkCardLine(source)) {
    const preview = notePreviewMarkdown(post, maxLen)
    return preview.endsWith('…') || preview.replace(/\s*https?:\/\/\S+\s*/g, '').trim().length < source.replace(/\s*https?:\/\/\S+\s*/g, '').trim().length
  }
  const preview = notePreviewMarkdown(post, maxLen)
  return preview.endsWith('…') || preview.length < source.length
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  })
}

/** "just now" / "Xm ago" / "Xh ago" / "Xd ago" / "Xmo ago" / "Xy ago" — compact
 * relative format (Hyvor Talk style) all the way back; full date/time is
 * available via a title tooltip on the caller's <time> element. */
export function relativeTime(iso: string): string {
  const diffMs = Date.now() - new Date(iso).getTime()
  const minutes = Math.floor(diffMs / 60_000)
  if (minutes < 1) return 'just now'
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  if (days < 30) return `${days}d ago`
  const months = Math.floor(days / 30)
  if (months < 12) return `${months}mo ago`
  const years = Math.floor(months / 12)
  return `${years}y ago`
}

export async function fetchPosts(): Promise<Post[]> {
  const res = await fetch('/api/posts')
  if (!res.ok) throw new Error('Failed to load posts')
  return res.json()
}

export type PostsPage = {
  posts: Post[]
  total: number
}

export async function fetchPostsPage(limit: number, offset: number): Promise<PostsPage> {
  const res = await fetch(`/api/posts?limit=${limit}&offset=${offset}`)
  if (!res.ok) throw new Error('Failed to load posts')
  return res.json()
}

export async function fetchPost(slug: string): Promise<Post> {
  const res = await fetch(`/api/posts/${slug}`)
  if (res.status === 404) throw new Error('not-found')
  if (!res.ok) throw new Error('Failed to load post')
  return res.json()
}

/** Timestamp-based placeholder for a draft with no derivable title (KTD6). */
export function draftPlaceholderTitle(createdAt: string): string {
  const d = new Date(createdAt)
  const datePart = d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
  const timePart = d.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })
  return `Draft — ${datePart}, ${timePart}`
}

export function postDisplayTitle(post: Post): string {
  // A draft's separately-saved title (KTD7) takes precedence — its Content
  // is not yet title-embedded the way a published article's is.
  if (post.title) return post.title
  // A draft with no title goes straight to the timestamp placeholder (R6) —
  // no content-derived fallback. For a draft article, Content is body-only,
  // so falling back to its first line would show body text as if it were a
  // title; for a draft note it would surface in-progress body content where
  // the UI implies "title".
  if (post.status === 'draft') return draftPlaceholderTitle(post.created_at)
  if (post.kind === 'article') {
    const firstLine = stripTitleMarker(post.content.split('\n')[0]?.trim() ?? '')
    if (firstLine) return firstLine
    return 'Untitled article'
  }
  const excerpt = plainTextFromMarkdown(post.content, 60)
  return excerpt || 'Note'
}

export async function updatePost(
  slug: string,
  payload: {
    kind: string
    content: string
    source_url?: string
    title?: string
    excerpt?: string
    commentary?: string
    via?: string
  },
): Promise<Post> {
  const res = await fetch(`/api/posts/${slug}`, {
    method: 'PUT',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  if (res.status === 404) throw new Error('not-found')
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || 'Update failed')
  }
  return res.json()
}

export async function saveDraft(payload: {
  kind: string
  title?: string
  content: string
  slug?: string
}): Promise<Post> {
  const res = await fetch('/api/posts/drafts', {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || 'Autosave failed')
  }
  return res.json()
}

export async function publishDraft(
  slug: string,
  payload?: { kind?: string; title?: string; content?: string; federate?: boolean },
): Promise<Post> {
  const res = await fetch(`/api/posts/${slug}/publish`, {
    method: 'POST',
    credentials: 'include',
    headers: payload ? { 'Content-Type': 'application/json' } : undefined,
    body: payload ? JSON.stringify(payload) : undefined,
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || 'Publish failed')
  }
  return res.json()
}

export async function deletePost(slug: string): Promise<void> {
  const res = await fetch(`/api/posts/${slug}`, {
    method: 'DELETE',
    credentials: 'include',
  })
  if (res.status === 404) throw new Error('not-found')
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || 'Delete failed')
  }
}
