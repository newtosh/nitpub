---
title: Changelog
---

<script setup>
import { data as releases } from './changelog.data.ts'
import MarkdownIt from 'markdown-it'

const md = new MarkdownIt({ html: false, linkify: true, breaks: true })
function renderBody(body) {
  return body ? md.render(body) : '<p class="muted">No notes for this release.</p>'
}
function formatDate(iso) {
  if (!iso) return ''
  return new Intl.DateTimeFormat('en', { year: 'numeric', month: 'short', day: 'numeric' }).format(new Date(iso))
}
</script>

# Changelog

Release notes from [GitHub Releases](https://github.com/newtosh/nitpub/releases).

<p v-if="!releases.length" class="muted">No releases listed yet. Check <a href="https://github.com/newtosh/nitpub/releases">GitHub Releases</a>.</p>

<article v-for="r in releases" :key="r.tag" class="changelog-release">
  <h2><a :href="r.html_url">{{ r.name }}</a> <span v-if="r.prerelease" class="changelog-pill">pre</span></h2>
  <p class="changelog-meta"><code>{{ r.tag }}</code> · {{ formatDate(r.published_at) }}</p>
  <div v-html="renderBody(r.body)"></div>
</article>

<style>
.changelog-release {
  margin: 2rem 0;
  padding-top: 1.5rem;
  border-top: 1px solid var(--vp-c-divider);
}
.changelog-release:first-of-type {
  border-top: none;
}
.changelog-meta {
  color: var(--vp-c-text-2);
  font-size: 0.9em;
}
.changelog-pill {
  display: inline-block;
  padding: 0.1em 0.6em;
  border-radius: 999px;
  background: var(--vp-c-default-soft);
  color: var(--vp-c-text-2);
  font-size: 0.7em;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  vertical-align: middle;
}
</style>
