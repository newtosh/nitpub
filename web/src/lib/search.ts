export type SearchResult = {
  type: 'post' | 'page'
  title: string
  snippet: string
  url: string
}

export async function searchContent(query: string): Promise<SearchResult[]> {
  const q = query.trim()
  if (!q) return []
  const res = await fetch(`/api/search?q=${encodeURIComponent(q)}`)
  if (!res.ok) throw new Error('Search failed')
  const data = (await res.json()) as { results: SearchResult[] }
  return data.results ?? []
}
