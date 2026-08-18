export type ThemeMode = 'light' | 'dark'

const themeStorageKey = 'admin:theme'

export function readTheme(): ThemeMode {
  return window.localStorage.getItem(themeStorageKey) === 'dark' ? 'dark' : 'light'
}

export function applyTheme(theme: ThemeMode): void {
  document.documentElement.classList.toggle('dark', theme === 'dark')
  document.documentElement.style.colorScheme = theme
}

export function initializeTheme(): ThemeMode {
  const theme = readTheme()
  applyTheme(theme)
  return theme
}

export function toggleTheme(current: ThemeMode): ThemeMode {
  const nextTheme: ThemeMode = current === 'light' ? 'dark' : 'light'
  window.localStorage.setItem(themeStorageKey, nextTheme)
  applyTheme(nextTheme)
  return nextTheme
}
