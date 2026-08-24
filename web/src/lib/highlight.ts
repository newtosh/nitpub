import hljs from 'highlight.js/lib/core'
import bash from 'highlight.js/lib/languages/bash'
import css from 'highlight.js/lib/languages/css'
import go from 'highlight.js/lib/languages/go'
import java from 'highlight.js/lib/languages/java'
import javascript from 'highlight.js/lib/languages/javascript'
import json from 'highlight.js/lib/languages/json'
import python from 'highlight.js/lib/languages/python'
import rust from 'highlight.js/lib/languages/rust'
import sql from 'highlight.js/lib/languages/sql'
import typescript from 'highlight.js/lib/languages/typescript'
import xml from 'highlight.js/lib/languages/xml'
import yaml from 'highlight.js/lib/languages/yaml'

const languages: Array<[string, Parameters<typeof hljs.registerLanguage>[1]]> = [
  ['go', go],
  ['javascript', javascript],
  ['js', javascript],
  ['typescript', typescript],
  ['ts', typescript],
  ['bash', bash],
  ['sh', bash],
  ['shell', bash],
  ['json', json],
  ['yaml', yaml],
  ['yml', yaml],
  ['sql', sql],
  ['python', python],
  ['py', python],
  ['rust', rust],
  ['java', java],
  ['html', xml],
  ['xml', xml],
  ['css', css],
]

for (const [name, lang] of languages) {
  hljs.registerLanguage(name, lang)
}

export function highlightCode(source: string, lang: string): string {
  const name = lang.trim().toLowerCase().split(/\s+/)[0] ?? ''
  if (name && hljs.getLanguage(name)) {
    return hljs.highlight(source, { language: name, ignoreIllegals: true }).value
  }
  return hljs.highlightAuto(source).value
}

export { hljs }
