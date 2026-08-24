import type { ColorScheme } from './theme-catalog'
import { DEFAULT_COLOR_SCHEME, normalizeColorScheme } from './theme-catalog'

const STORAGE_KEY = 'nitpub-color-scheme'

export function loadColorSchemePreference(): ColorScheme {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    return normalizeColorScheme(raw ?? undefined)
  } catch {
    return DEFAULT_COLOR_SCHEME
  }
}

export function saveColorSchemePreference(scheme: ColorScheme): void {
  try {
    localStorage.setItem(STORAGE_KEY, scheme)
  } catch {
    // private browsing or blocked storage — session-only is fine
  }
}
