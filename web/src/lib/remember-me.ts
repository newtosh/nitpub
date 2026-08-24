const STORAGE_KEY = 'nitpub-remember-me'

/** Whether the login form should default the "Remember me" checkbox to checked. */
export function loadRememberMePreference(): boolean {
  try {
    return localStorage.getItem(STORAGE_KEY) === '1'
  } catch {
    return false
  }
}

export function saveRememberMePreference(remember: boolean): void {
  try {
    localStorage.setItem(STORAGE_KEY, remember ? '1' : '0')
  } catch {
    // ignore blocked storage
  }
}
