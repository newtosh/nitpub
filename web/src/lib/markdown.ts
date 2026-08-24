import MarkdownIt from 'markdown-it'
import type { Options } from 'markdown-it'
import markdownItGitHubAlerts from 'markdown-it-github-alerts'
import markdownItMark from 'markdown-it-mark'
import { markdownItStrikethrough } from './markdownItStrikethrough'
import markdownItMultimdTable from 'markdown-it-multimd-table'
import markdownItTaskLists from 'markdown-it-task-lists'
import DOMPurify from 'dompurify'
import { highlightCode } from './highlight'
import { linkCardPlaceholder } from './linkCard'
import { iconPlaceholder } from './phosphorIcons'
import { youtubeFacadeHtml } from './youtubeFacade'
import type { StateInline } from 'markdown-it'

function highlight(str: string, lang: string): string {
  return highlightCode(str, lang)
}

const md = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: true,
  typographer: true,
  highlight,
} as Options)
  .use(markdownItGitHubAlerts)
  .use(markdownItMark)
  .use(markdownItStrikethrough)
  .use(markdownItTaskLists, { enabled: false })
  .use(markdownItMultimdTable, { multiline: true, rowspan: true })

// External links open in a new tab by default (admin-configurable,
// ContentConfig.ExternalLinksNewTab) — same-site/relative links always stay
// in the current tab regardless. Reads the per-render env object markdown-it
// passes through rather than a module-level flag, so concurrent renders with
// different settings (e.g. an admin preview) can't stomp on each other.
type LinkEnv = { externalLinksNewTab?: boolean }

function isExternalHref(href: string): boolean {
  try {
    const base = typeof window !== 'undefined' ? window.location.origin : 'http://localhost'
    const url = new URL(href, base)
    if (url.protocol !== 'http:' && url.protocol !== 'https:') return false
    if (typeof window === 'undefined') return true
    return url.hostname !== window.location.hostname
  } catch {
    return false
  }
}

const defaultLinkOpen =
  md.renderer.rules.link_open ??
  ((tokens, idx, opts, _env, self) => self.renderToken(tokens, idx, opts))
md.renderer.rules.link_open = (tokens, idx, opts, env: LinkEnv, self) => {
  const token = tokens[idx]
  const hrefIdx = token.attrIndex('href')
  const href = hrefIdx >= 0 ? token.attrs![hrefIdx][1] : ''
  if ((env?.externalLinksNewTab ?? true) && isExternalHref(href)) {
    token.attrSet('target', '_blank')
    token.attrSet('rel', 'noopener noreferrer')
  }
  return defaultLinkOpen(tokens, idx, opts, env, self)
}

// ":icon-name:" shortcode (Slack-emoji-style), backed by Phosphor icons —
// see internal/icons and web/src/lib/phosphorIcons.ts. A proper inline-rule
// hook (not a post-render regex over the HTML string, like the embed/link
// card rules below) so it only fires on actual text, never inside code
// spans, fenced blocks, or link hrefs the way a naive string replace could.
// Requiring the name to start with a letter (matching every real Phosphor
// name) is what keeps something like "12:30:45" from ever being treated as
// a candidate shortcode in the first place — the two candidate names there
// ("30", "45") are digit-led, so this rule never touches them.
const iconShortcode = /^:([a-z][a-z0-9]*(?:-[a-z0-9]+)*):/
md.inline.ruler.after('emphasis', 'phosphor_icon', (state: StateInline, silent: boolean) => {
  if (state.src.charCodeAt(state.pos) !== 0x3a /* ':' */) return false
  const match = iconShortcode.exec(state.src.slice(state.pos))
  if (!match) return false
  if (!silent) {
    const token = state.push('phosphor_icon', '', 0)
    token.content = match[1]
  }
  state.pos += match[0].length
  return true
})
md.renderer.rules.phosphor_icon = (tokens, idx) => iconPlaceholder(tokens[idx].content)

const defaultFence = md.renderer.rules.fence!
md.renderer.rules.fence = (tokens, idx, options, _env, slf) => {
  const token = tokens[idx]
  const lang = token.info.trim().split(/\s+/g)[0] ?? ''
  const label = lang
    ? `<span class="code-lang">${md.utils.escapeHtml(lang)}</span>`
    : ''
  if (options.highlight) {
    const inner = options.highlight(token.content, lang, '')
    if (inner) {
      const cls = lang ? `hljs language-${md.utils.escapeHtml(lang)}` : 'hljs'
      return `<div class="code-block">${label}<pre><code class="${cls}">${inner}</code></pre></div>\n`
    }
  }
  return defaultFence(tokens, idx, options, _env, slf)
}

const embedAllowedHosts = new Set([
  'www.youtube-nocookie.com',
  'www.youtube.com',
  'player.vimeo.com',
  'open.spotify.com',
])

let sanitizeHooksReady = false

function ensureSanitizeHooks() {
  if (sanitizeHooksReady) return
  DOMPurify.addHook('uponSanitizeElement', (node, data) => {
    if (data.tagName !== 'iframe' || !(node instanceof Element)) return
    const src = node.getAttribute('src') ?? ''
    try {
      const host = new URL(src, window.location.origin).hostname
      if (!embedAllowedHosts.has(host)) {
        node.remove()
      }
    } catch {
      node.remove()
    }
  })
  sanitizeHooksReady = true
}

function sanitize(html: string): string {
  ensureSanitizeHooks()
  return DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true, svg: true, svgFilters: true },
    ADD_TAGS: ['iframe', 'span', 'button', 'img', 'figure', 'figcaption', 'input', 'label'],
    ADD_ATTR: [
      'allow',
      'allowfullscreen',
      'frameborder',
      'loading',
      'referrerpolicy',
      'class',
      'title',
      // DOMPurify's default "html" profile excludes target (historical
      // reverse-tabnabbing hardening) — rel="noopener noreferrer" is what
      // actually closes that hole, and we always set both together in the
      // link_open rule above, so allowing target here is safe.
      'target',
      'src',
      'alt',
      'decoding',
      'type',
      'aria-label',
      'aria-hidden',
      'data-yt-id',
      'data-yt-mode',
      'data-bound',
      'data-unfurl-url',
      'data-icon-name',
      'disabled',
      'checked',
    ],
  })
}

type EmbedMatch = { html: string } | null

function youtubeEmbed(url: string): EmbedMatch {
  const watch = url.match(
    /(?:youtube\.com\/watch\?v=|youtube\.com\/shorts\/|youtu\.be\/)([A-Za-z0-9_-]{6,})/i,
  )
  if (!watch) return null
  return { html: youtubeFacadeHtml(watch[1]) }
}

function vimeoEmbed(url: string): EmbedMatch {
  const m = url.match(/vimeo\.com\/(\d+)/i)
  if (!m) return null
  return {
    html: `<div class="embed embed-video"><iframe src="https://player.vimeo.com/video/${m[1]}" title="Vimeo video" loading="lazy" allow="autoplay; fullscreen; picture-in-picture" allowfullscreen></iframe></div>`,
  }
}

function spotifyEmbed(url: string): EmbedMatch {
  const m = url.match(/open\.spotify\.com\/(track|album|playlist|episode)\/([A-Za-z0-9]+)/i)
  if (!m) return null
  return {
    html: `<div class="embed embed-audio"><iframe src="https://open.spotify.com/embed/${m[1]}/${m[2]}" title="Spotify embed" loading="lazy" allow="autoplay; clipboard-write; encrypted-media; fullscreen; picture-in-picture"></iframe></div>`,
  }
}

function linkCardEmbed(url: string): EmbedMatch {
  try {
    const parsed = new URL(url)
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return null
    if (youtubeEmbed(url) || vimeoEmbed(url) || spotifyEmbed(url)) return null
    return { html: linkCardPlaceholder(url) }
  } catch {
    return null
  }
}

function embedForURL(url: string): EmbedMatch {
  return youtubeEmbed(url) ?? vimeoEmbed(url) ?? spotifyEmbed(url) ?? linkCardEmbed(url)
}

function enhanceFigures(html: string): string {
  const withCaption = html.replace(
    /<p>\s*(<img\b[^>]*>)\s*<\/p>\s*<p>\s*<em>([\s\S]*?)<\/em>\s*<\/p>/gi,
    (_match, img: string, caption: string) =>
      `<figure class="figure">${img}<figcaption class="figure-caption">${caption}</figcaption></figure>`,
  )
  return withCaption.replace(
    /<p>\s*(<img\b[^>]*>)\s*<\/p>/gi,
    (_match, img: string) => `<figure class="figure">${img}</figure>`,
  )
}

const soloLinkParagraph = /<p>\s*<a href="([^"]+)"[^>]*>[^<]*<\/a>\s*<\/p>/gi
const singleLinkInParagraph = /<a href="([^"]+)"[^>]*>[^<]*<\/a>/gi

function enhanceEmbeds(html: string, options?: { inlineLinkCards?: boolean }): string {
  // A paragraph that's the URL and nothing else always becomes just the
  // card, for both notes and articles.
  const solo = html.replace(soloLinkParagraph, (full, href: string) => embedForURL(href)?.html ?? full)
  if (!options?.inlineLinkCards) return solo

  // Notes are short, conversational posts where "some text: URL" is the
  // natural way to share a link — unlike an article's prose, upgrading
  // these doesn't disrupt reading flow the same way a mid-paragraph card
  // would in long-form writing. Keep the original sentence (and its
  // inline link) intact and append the card below it, rather than
  // replacing the paragraph outright.
  return solo.replace(/<p>([\s\S]*?)<\/p>/gi, (full, inner: string) => {
    const links = [...inner.matchAll(singleLinkInParagraph)]
    if (links.length !== 1) return full
    const embed = embedForURL(links[0][1])
    return embed ? `${full}${embed.html}` : full
  })
}

export function renderMarkdown(
  source: string,
  options?: { inlineLinkCards?: boolean; externalLinksNewTab?: boolean },
): string {
  const raw = md.render(source, { externalLinksNewTab: options?.externalLinksNewTab } as LinkEnv)
  return sanitize(enhanceFigures(enhanceEmbeds(raw, options)))
}

function htmlToPlainText(html: string): string {
  const stripped = html
    .replace(/<[^>]+>/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
  if (typeof document === 'undefined') return stripped
  const el = document.createElement('div')
  el.innerHTML = stripped
  return (el.textContent ?? stripped).replace(/\s+/g, ' ').trim()
}

export function plainTextFromMarkdown(source: string, maxLen?: number): string {
  const text = htmlToPlainText(md.render(source))
  if (maxLen && text.length > maxLen) {
    return text.slice(0, maxLen - 3) + '...'
  }
  return text
}

export function stripTitleMarker(line: string): string {
  return line.replace(/^#+\s*/, '').trim()
}
