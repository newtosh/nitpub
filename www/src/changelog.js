import MarkdownIt from 'markdown-it'
import data from '../data/releases.json'

const md = new MarkdownIt({ html: false, linkify: true, breaks: true })
const root = document.getElementById('changelog-root')

function formatDate(iso) {
  if (!iso) return ''
  try {
    return new Intl.DateTimeFormat('en', { year: 'numeric', month: 'short', day: 'numeric' }).format(new Date(iso))
  } catch {
    return iso
  }
}

const releases = data.releases || []
if (!releases.length) {
  root.innerHTML = `<p class="muted">No releases listed yet. Check <a href="https://github.com/newtosh/nitpub/releases">GitHub Releases</a>.</p>`
} else {
  root.innerHTML = releases
    .map((r) => {
      const body = r.body ? md.render(r.body) : '<p class="muted">No notes for this release.</p>'
      const pre = r.prerelease ? ' <span class="pill">pre</span>' : ''
      return `<article class="release">
        <header>
          <h2><a href="${r.html_url}">${escapeHtml(r.name || r.tag)}</a>${pre}</h2>
          <p class="meta"><code>${escapeHtml(r.tag)}</code> · ${formatDate(r.published_at)}</p>
        </header>
        <div class="release-body">${body}</div>
      </article>`
    })
    .join('')
}

function escapeHtml(s) {
  return String(s)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
}
