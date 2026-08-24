import { stripTitleMarker } from './markdown'

/** Mastodon default; ActivityPub has no protocol-level cap. */
export const NOTE_MAX_CHARS = 500

export function noteCharCount(text: string): number {
  return [...text].length
}

export function splitArticleContent(content: string): { title: string; body: string } {
  const lines = content.split('\n')
  const first = lines[0]?.trim() ?? ''
  const title = stripTitleMarker(first)
  const body = lines.length > 1 ? lines.slice(1).join('\n').trimStart() : ''
  return { title, body }
}

export function combineArticleContent(title: string, body: string): string {
  const t = title.trim()
  const b = body.trim()
  if (!t && !b) return ''
  if (!t) return b
  if (!b) return t
  return `${t}\n\n${b}`
}

export function shouldAutoConvertToArticle(text: string): boolean {
  return noteCharCount(text.trim()) > NOTE_MAX_CHARS
}

export function noteLengthLabel(count: number): string {
  if (count > NOTE_MAX_CHARS) return `${count} / ${NOTE_MAX_CHARS} — too long for a note`
  if (count > NOTE_MAX_CHARS * 0.9) return `${count} / ${NOTE_MAX_CHARS} — almost at the limit`
  return `${count} / ${NOTE_MAX_CHARS}`
}
