import DOMPurify from 'dompurify'
import { escapeHtml } from './linkCard'

const iconCache = new Map<string, string>()

export function iconPlaceholder(name: string): string {
  const esc = escapeHtml(name)
  return `<span class="icon-phosphor icon-phosphor--pending" data-icon-name="${esc}">:${esc}:</span>`
}

export async function fetchIconSVG(name: string): Promise<string> {
  const cached = iconCache.get(name)
  if (cached) return cached
  const res = await fetch(`/icons/${encodeURIComponent(name)}`)
  if (!res.ok) throw new Error('icon fetch failed')
  const raw = await res.text()
  // Trust boundary: /icons proxies Phosphor's own GitHub repo, fetched and
  // cached server-side (internal/icons) — not visitor-supplied content. Still
  // sanitized here as defense in depth against a compromised upstream.
  const svg = DOMPurify.sanitize(raw, { USE_PROFILES: { svg: true, svgFilters: true } })
  iconCache.set(name, svg)
  return svg
}

/** Replaces every pending icon placeholder with its fetched SVG, or falls
 * back to the literal ":name:" text if the name isn't a real Phosphor icon —
 * matches Slack's behavior for an unrecognized emoji shortcode, and quietly
 * absorbs false-positive matches like a "12:30:45"-shaped timestamp that the
 * markdown rule can't distinguish from a real shortcode without a fetch. */
export async function hydrateIcons(root: HTMLElement | null): Promise<void> {
  if (!root) return
  const pending = [...root.querySelectorAll<HTMLElement>('.icon-phosphor--pending[data-icon-name]')]
  await Promise.all(
    pending.map(async (el) => {
      const name = el.getAttribute('data-icon-name')
      if (!name) return
      try {
        const svg = await fetchIconSVG(name)
        el.outerHTML = `<span class="icon-phosphor" aria-hidden="true">${svg}</span>`
      } catch {
        el.outerHTML = `:${escapeHtml(name)}:`
      }
    }),
  )
}
