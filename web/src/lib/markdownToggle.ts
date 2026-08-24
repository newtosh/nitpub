export type WrapSpec = {
  before: string
  after: string
  placeholder?: string
}

export type ToggleResult = {
  text: string
  selStart: number
  selEnd: number
}

/** Wrap or unwrap inline markdown markers around the current selection (GitHub-style). */
export function toggleWrap(
  text: string,
  start: number,
  end: number,
  spec: WrapSpec,
): ToggleResult {
  const { before, after, placeholder = 'text' } = spec
  const hasSelection = start !== end
  const selected = hasSelection ? text.slice(start, end) : ''

  const beforeStart = start - before.length
  const afterEnd = end + after.length
  if (
    beforeStart >= 0 &&
    text.slice(beforeStart, start) === before &&
    text.slice(end, afterEnd) === after
  ) {
    const inner = hasSelection ? selected : placeholder
    return {
      text: text.slice(0, beforeStart) + inner + text.slice(afterEnd),
      selStart: beforeStart,
      selEnd: beforeStart + inner.length,
    }
  }

  if (
    hasSelection &&
    selected.startsWith(before) &&
    selected.endsWith(after) &&
    selected.length >= before.length + after.length
  ) {
    const inner = selected.slice(before.length, selected.length - after.length)
    return {
      text: text.slice(0, start) + inner + text.slice(end),
      selStart: start,
      selEnd: start + inner.length,
    }
  }

  const inner = hasSelection ? selected : placeholder
  const wrapped = before + inner + after
  return {
    text: text.slice(0, start) + wrapped + text.slice(end),
    selStart: start + before.length,
    selEnd: start + before.length + inner.length,
  }
}

const linkPattern = /^\[([^\]]*)\]\(([^)]*)\)$/

/** Toggle a markdown link around the selection. */
export function toggleLink(
  text: string,
  start: number,
  end: number,
  defaultURL = 'https://',
): ToggleResult {
  const hasSelection = start !== end
  const selected = hasSelection ? text.slice(start, end) : ''

  const match = selected.match(linkPattern)
  if (match) {
    const label = match[1]
    return {
      text: text.slice(0, start) + label + text.slice(end),
      selStart: start,
      selEnd: start + label.length,
    }
  }

  const label = hasSelection ? selected : 'label'
  const wrapped = `[${label}](${defaultURL})`
  return {
    text: text.slice(0, start) + wrapped + text.slice(end),
    selStart: start + 1,
    selEnd: start + 1 + label.length,
  }
}

/** Toggle bold using ** or __ when the selection is already marked. */
export function toggleBold(text: string, start: number, end: number): ToggleResult {
  for (const spec of [
    { before: '**', after: '**', placeholder: 'bold' },
    { before: '__', after: '__', placeholder: 'bold' },
  ]) {
    const beforeStart = start - spec.before.length
    const afterEnd = end + spec.after.length
    if (
      beforeStart >= 0 &&
      text.slice(beforeStart, start) === spec.before &&
      text.slice(end, afterEnd) === spec.after
    ) {
      return toggleWrap(text, start, end, spec)
    }
    if (start !== end) {
      const selected = text.slice(start, end)
      if (
        selected.startsWith(spec.before) &&
        selected.endsWith(spec.after) &&
        selected.length >= spec.before.length + spec.after.length
      ) {
        return toggleWrap(text, start, end, spec)
      }
    }
  }
  return toggleWrap(text, start, end, { before: '**', after: '**', placeholder: 'bold' })
}

/** Toggle italic using * or _ markers. */
export function toggleItalic(text: string, start: number, end: number): ToggleResult {
  for (const spec of [
    { before: '*', after: '*', placeholder: 'italic' },
    { before: '_', after: '_', placeholder: 'italic' },
  ]) {
    const beforeStart = start - spec.before.length
    const afterEnd = end + spec.after.length
    if (
      beforeStart >= 0 &&
      text.slice(beforeStart, start) === spec.before &&
      text.slice(end, afterEnd) === spec.after &&
      !text.slice(beforeStart, afterEnd).startsWith('**')
    ) {
      return toggleWrap(text, start, end, spec)
    }
    if (start !== end) {
      const selected = text.slice(start, end)
      if (
        selected.startsWith(spec.before) &&
        selected.endsWith(spec.after) &&
        selected.length >= 2 &&
        !selected.startsWith('**')
      ) {
        return toggleWrap(text, start, end, spec)
      }
    }
  }
  return toggleWrap(text, start, end, { before: '*', after: '*', placeholder: 'italic' })
}
