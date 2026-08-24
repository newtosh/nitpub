export type IconCatalogEntry = {
  name: string
  tags: string[]
}

let cached: Promise<IconCatalogEntry[]> | null = null

/** Fetches the full icon catalog once per page load and reuses it after. */
export function fetchIconCatalog(): Promise<IconCatalogEntry[]> {
  if (!cached) {
    cached = fetch('/api/icons/catalog', { credentials: 'include' })
      .then((res) => (res.ok ? (res.json() as Promise<IconCatalogEntry[]>) : []))
      .catch(() => [])
  }
  return cached
}

/**
 * Ranks catalog entries against a query: exact name match first, then
 * name-starts-with, then a tag/name substring hit anywhere. Empty query
 * returns the first `limit` entries as-is (alphabetical, from the catalog).
 */
export function searchIconCatalog(
  entries: IconCatalogEntry[],
  query: string,
  limit = 8,
): IconCatalogEntry[] {
  const q = query.trim().toLowerCase()
  if (!q) return entries.slice(0, limit)

  const scored: { entry: IconCatalogEntry; score: number }[] = []
  for (const entry of entries) {
    const name = entry.name.toLowerCase()
    let score = -1
    if (name === q) score = 0
    else if (name.startsWith(q)) score = 1
    else if (name.includes(q)) score = 2
    else if (entry.tags.some((t) => t.toLowerCase() === q)) score = 3
    else if (entry.tags.some((t) => t.toLowerCase().startsWith(q))) score = 4
    else if (entry.tags.some((t) => t.toLowerCase().includes(q))) score = 5
    if (score >= 0) scored.push({ entry, score })
  }
  scored.sort((a, b) => a.score - b.score || a.entry.name.localeCompare(b.entry.name))
  return scored.slice(0, limit).map((s) => s.entry)
}
