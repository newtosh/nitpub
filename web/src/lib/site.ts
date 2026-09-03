export type NavItem = {
  label: string
  path: string
  icon?: string
}

export type SiteConfig = {
  title?: string
  nav: NavItem[]
  home: { recent_count: number }
  archive: { mode: 'pagination' | 'infinite'; page_size: number }
  search: { enabled: boolean }
  federation?: {
    cross_post_default?: boolean | null
    show_avatars_default?: boolean | null
    moderation_enabled?: boolean | null
    replies_collapsed_default?: boolean | null
    reference_instance?: string
  }
  content?: {
    external_links_new_tab?: boolean | null
    expand_notes_in_feed?: boolean
  }
  // Deploy-time config.toml flag (internal/config), not part of the
  // runtime-editable settings above — never wire an admin edit form for it.
  analytics_enabled?: boolean
  // CLI-flag-only (--enable-quote-posts), not config.toml/admin-editable —
  // same rationale as analytics_enabled above.
  quote_posts_enabled?: boolean
  footer?: {
    text?: string
    show_github_link?: boolean | null
    github_url?: string
  }
  version?: string
  branding?: {
    favicon_url?: string
    logo_url?: string
    tagline?: string
  }
}

export type SitePage =
  | { type: 'markdown'; path: string; title?: string; body: string; file?: string }
  | {
      type: 'links'
      path: string
      title?: string
      links: LinkEntry[]
      file?: string
    }

export type LinkEntry = {
  title: string
  url: string
  description?: string
  icon?: string
}

const STORAGE_KEY = 'nitpub-site-config-cache'

// A plain in-memory cache resets on every full page load, which is
// exactly when this mattered most: a hard refresh showed the default
// title ("nitpub") and an empty nav for a beat before the real fetch
// resolved, then visibly snapped to the actual config. Persisting to
// localStorage and reading it back synchronously (not awaited) lets
// the very first render already have last-known-good data instead of
// starting from nothing every time.
let cached: SiteConfig | null = readStoredConfig()

function readStoredConfig(): SiteConfig | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    return raw ? (JSON.parse(raw) as SiteConfig) : null
  } catch {
    return null
  }
}

function writeStoredConfig(config: SiteConfig) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(config))
  } catch {
    // Storage full/unavailable (private browsing) — cache just won't
    // survive a reload; not worth failing the actual config fetch over.
  }
}

// Synchronous — for seeding a ref's initial value so first paint isn't
// forced to show defaults/empty state even for a single frame. May be
// stale (or null, for a first-ever visit); fetchSiteConfig() below
// still runs to get the current value and correct it if it's changed.
export function getCachedSiteConfig(): SiteConfig | null {
  return cached
}

export async function fetchSiteConfig(): Promise<SiteConfig> {
  const res = await fetch('/api/site')
  if (!res.ok) throw new Error('Failed to load site config')
  cached = await res.json()
  writeStoredConfig(cached!)
  return cached!
}

export function clearSiteConfigCache() {
  cached = null
  try {
    localStorage.removeItem(STORAGE_KEY)
  } catch {
    // ignore
  }
}

export async function fetchSitePage(path: string): Promise<SitePage> {
  const rel = path.replace(/^\//, '')
  const res = await fetch(`/api/site/pages/${rel}`)
  if (res.status === 404) throw new Error('not-found')
  if (!res.ok) throw new Error('Failed to load page')
  return res.json()
}
