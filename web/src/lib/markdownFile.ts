export type MarkdownFileKind = 'note' | 'article'

export type ParsedMarkdownFile = {
  kind: MarkdownFileKind
  content: string
  filename: string
}

function inferKind(content: string): MarkdownFileKind {
  const first = content.split('\n')[0]?.trim() ?? ''
  if (first.startsWith('# ')) return 'note'
  if (content.includes('\n')) return 'article'
  return 'note'
}

function stripFrontmatter(text: string): { kind?: MarkdownFileKind; content: string } {
  if (!text.startsWith('---\n')) {
    return { content: text }
  }
  const parts = text.split('\n---\n', 2)
  if (parts.length !== 2) {
    return { content: text }
  }
  let kind: MarkdownFileKind | undefined
  for (const line of parts[0].split('\n')) {
    const trimmed = line.trim().replace(/^---/, '').trim()
    if (trimmed.startsWith('kind:')) {
      const value = trimmed.slice('kind:'.length).trim().toLowerCase()
      if (value === 'note' || value === 'article') kind = value
    }
  }
  return { kind, content: parts[1] }
}

export function parseMarkdownText(text: string, filename: string, defaultKind: MarkdownFileKind = 'article'): ParsedMarkdownFile {
  const { kind: fromMeta, content: raw } = stripFrontmatter(text)
  const content = raw.trim()
  if (!content) {
    throw new Error('File is empty')
  }
  const kind = fromMeta ?? (defaultKind || inferKind(content))
  return { kind, content, filename }
}

export async function readMarkdownFile(file: File): Promise<ParsedMarkdownFile> {
  if (!file.name.toLowerCase().endsWith('.md') && file.type !== 'text/markdown') {
    throw new Error('Choose a .md markdown file')
  }
  const text = await file.text()
  return parseMarkdownText(text, file.name)
}
