export type ThemeDefinition = {
  id: string
  name: string
  description: string
}

export type ColorScheme = 'auto' | 'light' | 'dark'

export const COLOR_SCHEMES: { id: ColorScheme; name: string; description: string }[] = [
  { id: 'auto', name: 'Auto', description: 'Follow system light/dark preference' },
  { id: 'light', name: 'Light', description: 'Always use the palette light variant' },
  { id: 'dark', name: 'Dark', description: 'Always use the palette dark variant' },
]

export const THEMES: ThemeDefinition[] = [
  { id: 'github', name: 'GitHub', description: 'Primer-inspired neutral UI.' },
  { id: 'nord', name: 'Nord', description: 'Arctic, calm blues and snow tones.' },
  { id: 'ayu', name: 'Ayu', description: 'Warm editor palette with orange accents.' },
  { id: 'tokyo-night', name: 'Tokyo Night', description: 'Indigo night sky and daybreak light.' },
  { id: 'catppuccin', name: 'Catppuccin', description: 'Pastel Latte and cozy Mocha.' },
  { id: 'dracula', name: 'Dracula', description: 'Purple accents on soft light or classic dark.' },
  { id: 'monokai', name: 'Monokai', description: 'Editor classic with green highlights.' },
]

export const DEFAULT_THEME_ID = 'github'
export const DEFAULT_COLOR_SCHEME: ColorScheme = 'auto'

const LEGACY_THEME_ALIASES: Record<string, string> = {
  warm: 'github',
  paper: 'github',
  ocean: 'tokyo-night',
  midnight: 'tokyo-night',
}

export function isValidTheme(id: string): boolean {
  return THEMES.some((t) => t.id === id)
}

export function normalizeThemeId(id: string): string {
  const aliased = LEGACY_THEME_ALIASES[id] ?? id
  return isValidTheme(aliased) ? aliased : DEFAULT_THEME_ID
}

export function normalizeColorScheme(scheme: string | undefined): ColorScheme {
  if (scheme === 'light' || scheme === 'dark' || scheme === 'auto') {
    return scheme
  }
  return DEFAULT_COLOR_SCHEME
}
