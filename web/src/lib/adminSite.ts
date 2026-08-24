import type { SiteConfig } from './site'

export type PageRef = {
  path: string
  type: 'markdown' | 'links'
  file: string
}

export type AdminManifest = SiteConfig & {
  pages?: PageRef[]
}

export type AdminSiteFile = {
  path: string
  content: string
}

export type AdminSiteResponse = {
  manifest: AdminManifest
  files: AdminSiteFile[]
  manifest_exists: boolean
}

export async function fetchAdminSite(): Promise<AdminSiteResponse> {
  const res = await fetch('/api/admin/site', { credentials: 'include' })
  if (!res.ok) throw new Error('Failed to load site admin data')
  return res.json()
}

export async function saveManifest(manifest: AdminManifest): Promise<void> {
  const res = await fetch('/api/admin/site/manifest', {
    method: 'PUT',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(manifest),
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || 'Save failed')
  }
}

export async function saveSiteFile(path: string, content: string): Promise<void> {
  const rel = path.replace(/^\//, '')
  const res = await fetch(`/api/admin/site/files/${rel}`, {
    method: 'PUT',
    credentials: 'include',
    headers: { 'Content-Type': 'text/plain' },
    body: content,
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || 'Save failed')
  }
}

export async function importPosts(files: FileList | File[], kind = 'article'): Promise<{ imported: number; errors: string[] }> {
  const form = new FormData()
  form.set('kind', kind)
  for (const file of files) {
    form.append('files', file)
  }
  const res = await fetch('/api/admin/import/posts', {
    method: 'POST',
    credentials: 'include',
    body: form,
  })
  if (!res.ok) throw new Error('Import failed')
  return res.json()
}

export type { LinkEntry } from './site'
