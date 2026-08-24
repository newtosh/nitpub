import MarkdownIt from 'markdown-it'
import markdownItMark from 'markdown-it-mark'
import DOMPurify from 'dompurify'
import { markdownItStrikethrough } from './markdownItStrikethrough'

const md = new MarkdownIt({ html: false, linkify: true, breaks: true })
  .use(markdownItMark)
  .use(markdownItStrikethrough)

const policy = {
  ALLOWED_TAGS: [
    'p', 'br', 'strong', 'em', 'b', 'i', 'u', 'del',
    'a', 'blockquote', 'pre', 'code', 'ul', 'ol', 'li',
  ],
  ALLOWED_ATTR: ['href', 'rel', 'class'],
}

const alertLine = /^>\s*\[!([A-Za-z]+)\]\s*(.*)$/gm
const imagePattern = /!\[[^\]]*\]\([^)]+\)/g
const embedLine =
  /^\s*(https?:\/\/(?:www\.)?(?:youtube\.com|youtu\.be|vimeo\.com|open\.spotify\.com)\S+)\s*$/gm

/** Preview how a note will look after Mastodon-style federation. */
export function renderFederatedNotePreview(source: string): string {
  let text = source
    .replace(alertLine, '> **$1:** $2')
    .replace(imagePattern, '')
    .replace(embedLine, '$1')
    .trim()
  const html = md.render(text)
  return DOMPurify.sanitize(html, policy)
}
