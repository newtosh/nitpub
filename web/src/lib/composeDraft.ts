export type ComposeDraft = {
  kind: 'note' | 'article'
  content: string
  savedAt: string
}

const PREFIX = 'nitpub:draft:'

function draftKey(slug: string): string {
  return `${PREFIX}${slug}`
}

export function loadComposeDraft(slug: string): ComposeDraft | null {
  try {
    const raw = localStorage.getItem(draftKey(slug))
    if (!raw) return null
    const parsed = JSON.parse(raw) as ComposeDraft
    if (parsed.kind !== 'note' && parsed.kind !== 'article') return null
    if (typeof parsed.content !== 'string') return null
    return parsed
  } catch {
    return null
  }
}

export function saveComposeDraft(slug: string, draft: Omit<ComposeDraft, 'savedAt'>): void {
  const payload: ComposeDraft = { ...draft, savedAt: new Date().toISOString() }
  try {
    localStorage.setItem(draftKey(slug), JSON.stringify(payload))
  } catch {
    // ignore quota errors
  }
}

export function clearComposeDraft(slug: string): void {
  try {
    localStorage.removeItem(draftKey(slug))
  } catch {
    // ignore
  }
}
