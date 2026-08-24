import type MarkdownIt from 'markdown-it'
import type StateInline from 'markdown-it/lib/rules_inline/state_inline.mjs'

/** GFM `~~strikethrough~~` → `<del>`. */
export function markdownItStrikethrough(md: MarkdownIt): void {
  function strikethrough(state: StateInline, silent: boolean): boolean {
    const start = state.pos
    if (state.src.charCodeAt(start) !== 0x7e /* ~ */) return false
    if (start + 2 >= state.posMax) return false
    if (state.src.charCodeAt(start + 1) !== 0x7e /* ~ */) return false
    if (silent) return false
    if (start > 0 && state.src.charCodeAt(start - 1) === 0x7e) return false

    state.pos = start + 2
    while (state.pos < state.posMax) {
      if (state.src.charCodeAt(state.pos) === 0x7e /* ~ */) {
        if (state.src.charCodeAt(state.pos + 1) !== 0x7e /* ~ */) break
        if (state.pos === start + 2) {
          state.pos = start
          return false
        }

        if (!silent) {
          const open = state.push('strikethrough_open', 'del', 1)
          open.markup = '~~'
          const text = state.push('text', '', 0)
          text.content = state.src.slice(start + 2, state.pos)
          const close = state.push('strikethrough_close', 'del', -1)
          close.markup = '~~'
        }

        state.pos += 2
        return true
      }
      state.md.inline.skipToken(state)
    }

    state.pos = start
    return false
  }

  md.inline.ruler.before('emphasis', 'strikethrough', strikethrough)
  md.renderer.rules.strikethrough_open = () => '<del>'
  md.renderer.rules.strikethrough_close = () => '</del>'
}
