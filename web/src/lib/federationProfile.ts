/** What Mastodon / ActivityPub reliably renders for federated Notes. */

export type EditorProfile = 'note' | 'article' | 'quote'

export const FEDERATION_NOTE = {
  format: 'HTML (converted from Markdown on publish)',
  supports: ['bold', 'italic', 'strikethrough', 'inline code', 'links', 'quotes', 'lists'],
  unsupported: ['callouts', 'images', 'embeds', 'tables', 'task lists'],
} as const

export const FEDERATION_ARTICLE = {
  format: 'Summary + permalink (not full body)',
  supports: ['full markdown on your blog'],
  federates: ['plain-text summary (≤280 chars)', 'link to article on nitpub'],
} as const

export function federationHint(profile: EditorProfile): string {
  if (profile === 'note') {
    return `Federates as HTML: ${FEDERATION_NOTE.supports.join(', ')}. ${FEDERATION_NOTE.unsupported.join(', ')} stay on your blog only — use an article for those.`
  }
  if (profile === 'quote') {
    return `Federates in full: the linked source, excerpt, commentary, and via line all go out — not a summary like article.`
  }
  return `Federates as a short summary + link to nitpub. Callouts, images, and embeds render on your site only.`
}
