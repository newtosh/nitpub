export type LinkPreview = {
  url: string
  title: string
  description?: string
  image?: string
  site_name?: string
}

const previewCache = new Map<string, LinkPreview>()

export function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

export function linkCardPlaceholder(url: string): string {
  const esc = escapeHtml(url)
  return `<div class="embed link-card link-card--pending" data-unfurl-url="${esc}"><a class="link-card-fallback" href="${esc}" rel="noopener noreferrer" target="_blank">${esc}</a></div>`
}

export function renderLinkCard(preview: LinkPreview): string {
  const title = escapeHtml(preview.title || preview.url)
  const url = escapeHtml(preview.url)
  const site = escapeHtml(preview.site_name || new URL(preview.url).hostname)
  const desc = preview.description ? escapeHtml(preview.description) : ''
  const image = preview.image
    ? `<img class="link-card-image" src="${escapeHtml(preview.image)}" alt="" loading="lazy" decoding="async" />`
    : ''
  const descHtml = desc
    ? `<span class="link-card-desc">${desc}</span>`
    : ''
  return `<div class="embed link-card">
  <a class="link-card-inner" href="${url}" rel="noopener noreferrer" target="_blank">
    ${image}
    <span class="link-card-body">
      <span class="link-card-site">${site}</span>
      <span class="link-card-title">${title}</span>
      ${descHtml}
    </span>
  </a>
</div>`
}

export async function fetchLinkPreview(url: string): Promise<LinkPreview> {
  const cached = previewCache.get(url)
  if (cached) return cached
  const res = await fetch(`/api/unfurl?url=${encodeURIComponent(url)}`)
  if (!res.ok) throw new Error('unfurl failed')
  const data = (await res.json()) as LinkPreview
  previewCache.set(url, data)
  return data
}

export async function hydrateLinkCards(root: HTMLElement | null): Promise<void> {
  if (!root) return
  const pending = [...root.querySelectorAll<HTMLElement>('.link-card--pending[data-unfurl-url]')]
  await Promise.all(
    pending.map(async (el) => {
      const url = el.getAttribute('data-unfurl-url')
      if (!url) return
      try {
        const preview = await fetchLinkPreview(url)
        el.outerHTML = renderLinkCard(preview)
      } catch {
        el.classList.remove('link-card--pending')
      }
    }),
  )
}

/** Standalone markdown URL line that becomes a link card embed. */
export const LINK_CARD_LINE = /^\s*https?:\/\/\S+\s*$/m

export function hasLinkCardLine(markdown: string): boolean {
  return LINK_CARD_LINE.test(markdown)
}
