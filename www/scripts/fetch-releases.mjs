#!/usr/bin/env node
/**
 * Build-time fetch of GitHub Releases for /changelog.
 * KTD4: User-Agent required; GITHUB_TOKEN when present; --require fails CI on error.
 */
import { mkdir, writeFile, readFile } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const root = join(__dirname, '..')
const outPath = join(root, 'data', 'releases.json')
const requireOk = process.argv.includes('--require') || process.env.CI === 'true'

const headers = {
  Accept: 'application/vnd.github+json',
  'User-Agent': 'nitpub-www-changelog-fetch/0.1 (+https://github.com/newtosh/nitpub)',
  'X-GitHub-Api-Version': '2022-11-28',
}
if (process.env.GITHUB_TOKEN) {
  headers.Authorization = `Bearer ${process.env.GITHUB_TOKEN}`
}

async function main() {
  await mkdir(dirname(outPath), { recursive: true })
  try {
    const res = await fetch('https://api.github.com/repos/newtosh/nitpub/releases?per_page=30', {
      headers,
    })
    if (!res.ok) {
      throw new Error(`GitHub Releases API ${res.status}: ${await res.text()}`)
    }
    const raw = await res.json()
    const releases = (Array.isArray(raw) ? raw : [])
      .filter((r) => !r.draft)
      .map((r) => ({
        tag: r.tag_name,
        name: r.name || r.tag_name,
        published_at: r.published_at,
        html_url: r.html_url,
        body: r.body || '',
        prerelease: Boolean(r.prerelease),
      }))
    await writeFile(outPath, JSON.stringify({ fetched_at: new Date().toISOString(), releases }, null, 2) + '\n')
    console.log(`wrote ${releases.length} releases → data/releases.json`)
  } catch (err) {
    console.error('fetch-releases:', err.message || err)
    if (requireOk) {
      process.exit(1)
    }
    try {
      await readFile(outPath, 'utf8')
      console.warn('using existing data/releases.json')
    } catch {
      await writeFile(
        outPath,
        JSON.stringify({ fetched_at: null, releases: [], error: String(err.message || err) }, null, 2) + '\n',
      )
      console.warn('wrote empty data/releases.json')
    }
  }
}

main()
