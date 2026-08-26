// Build-time fetch of GitHub Releases for /changelog — same approach as
// www/scripts/fetch-releases.mjs, but as a VitePress data loader so the
// docs site owns its own copy instead of coupling to www's build output.
interface Release {
  tag: string
  name: string
  published_at: string
  html_url: string
  body: string
  prerelease: boolean
}

export default {
  async load(): Promise<Release[]> {
    const headers = {
      Accept: 'application/vnd.github+json',
      'User-Agent': 'nitpub-docs-changelog-fetch/0.1 (+https://github.com/newtosh/nitpub)',
      'X-GitHub-Api-Version': '2022-11-28',
    }

    try {
      const res = await fetch('https://api.github.com/repos/newtosh/nitpub/releases?per_page=30', { headers })
      if (!res.ok) throw new Error(`GitHub Releases API ${res.status}`)
      const raw = await res.json()
      return (Array.isArray(raw) ? raw : [])
        .filter((r: any) => !r.draft)
        .map((r: any) => ({
          tag: r.tag_name,
          name: r.name || r.tag_name,
          published_at: r.published_at,
          html_url: r.html_url,
          body: r.body || '',
          prerelease: Boolean(r.prerelease),
        }))
    } catch (err) {
      // Build-time only: log and ship an empty list rather than fail the
      // docs build over a flaky GitHub API call.
      console.warn('changelog.data: fetch failed —', (err as Error).message)
      return []
    }
  },
}
