import { ref } from 'vue'
import { loadColorSchemePreference, saveColorSchemePreference } from '../lib/color-scheme-preference'
import {
  DEFAULT_COLOR_SCHEME,
  DEFAULT_THEME_ID,
  type ColorScheme,
  normalizeColorScheme,
  normalizeThemeId,
} from '../lib/theme-catalog'

const activeTheme = ref(DEFAULT_THEME_ID)
const activeScheme = ref<ColorScheme>(loadColorSchemePreference())

// iOS Safari tints the status bar strip above the page (not part of the
// page's own rendering surface, so CSS/env(safe-area-inset-top) can't
// reach it) using this meta tag. Without it Safari picks its own
// mismatched default instead of following whichever theme/light-dark
// mode is actually active — read the real computed value rather than
// hardcoding one, since both are user-selectable and this needs to
// track them exactly, not just the default theme's colors.
function syncThemeColorMeta(target: HTMLElement) {
  const surface = getComputedStyle(target).getPropertyValue('--surface').trim()
  if (!surface) return
  let meta = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]')
  if (!meta) {
    meta = document.createElement('meta')
    meta.name = 'theme-color'
    document.head.appendChild(meta)
  }
  meta.content = surface
}

// Safari 26 samples the sticky header's background-color once at
// initial paint and explicitly does not re-sample on later JS-driven
// color changes (confirmed by multiple external writeups on its
// "Liquid Glass" toolbar tinting — intentional on Apple's part, not a
// bug). A plain color change on an already-sampled element does
// nothing; removing the element from the render tree and reinserting
// it is a different event that does trigger a fresh sample — confirmed
// on a real device toggling light/dark with no page reload.
function pokeStatusBarTint() {
  const el = document.querySelector<HTMLElement>('.site-chrome')
  if (!el) return
  requestAnimationFrame(() => {
    const prevDisplay = el.style.display
    el.style.display = 'none'
    void el.offsetHeight
    requestAnimationFrame(() => {
      el.style.display = prevDisplay
    })
  })
}

export function useTheme() {
  function applyTheme(id: string, target: HTMLElement = document.documentElement) {
    activeTheme.value = normalizeThemeId(id)
    target.dataset.theme = activeTheme.value
    syncThemeColorMeta(target)
    pokeStatusBarTint()
  }

  function applyColorScheme(
    scheme: ColorScheme,
    target: HTMLElement = document.documentElement,
    persist = true,
  ) {
    activeScheme.value = normalizeColorScheme(scheme)
    target.dataset.scheme = activeScheme.value
    if (persist) {
      saveColorSchemePreference(activeScheme.value)
    }
    syncThemeColorMeta(target)
    pokeStatusBarTint()
  }

  function applyAppearance(
    themeId: string,
    colorScheme: ColorScheme = DEFAULT_COLOR_SCHEME,
    target: HTMLElement = document.documentElement,
    persistScheme = false,
  ) {
    applyTheme(themeId, target)
    applyColorScheme(colorScheme, target, persistScheme)
  }

  function clearAppearance(target: HTMLElement) {
    delete target.dataset.theme
    delete target.dataset.scheme
  }

  return {
    activeTheme,
    activeScheme,
    applyAppearance,
    applyTheme,
    applyColorScheme,
    clearAppearance,
  }
}
