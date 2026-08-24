import { isSixDigitHexColor, type ThemeMode } from './theme'

export type PageTransitionName = 'fade' | 'slide-left' | 'zoom'

export interface UIPreferences {
  theme: ThemeMode
  primaryColor: string
  showBreadcrumb: boolean
  showMenuToggle: boolean
  showRouteTabs: boolean
  uniqueOpened: boolean
  showFooter: boolean
  pageTransition: boolean
  transitionName: PageTransitionName
}

interface StoredUIPreferences {
  version: 1
  preferences: UIPreferences
}

export const uiPreferencesStorageKey = 'admin:ui-preferences'
export const defaultUIPreferences: Readonly<UIPreferences> = Object.freeze({
  theme: 'light',
  primaryColor: '#409EFF',
  showBreadcrumb: true,
  showMenuToggle: true,
  showRouteTabs: true,
  uniqueOpened: true,
  showFooter: true,
  pageTransition: true,
  transitionName: 'fade',
})

const preferenceKeys = [
  'pageTransition',
  'primaryColor',
  'showBreadcrumb',
  'showFooter',
  'showMenuToggle',
  'showRouteTabs',
  'theme',
  'transitionName',
  'uniqueOpened',
] as const

const transitionNames: readonly PageTransitionName[] = ['fade', 'slide-left', 'zoom']
export class UIPreferencesError extends Error {
  constructor(message: string, options?: ErrorOptions) {
    super(message, options)
    this.name = 'UIPreferencesError'
  }
}

export function parseStoredUIPreferences(value: unknown): UIPreferences {
  const stored = closedRecord(value, ['preferences', 'version'], 'stored UI preferences')
  if (stored.version !== 1) {
    throw new UIPreferencesError('stored UI preferences version must be 1')
  }

  const preferences = closedRecord(stored.preferences, [...preferenceKeys], 'stored UI preferences preferences')
  const theme = preferences.theme
  if (theme !== 'light' && theme !== 'dark') {
    throw new UIPreferencesError('stored UI preferences theme is invalid')
  }

  const primaryColor = preferences.primaryColor
  if (!isSixDigitHexColor(primaryColor)) {
    throw new UIPreferencesError('stored UI preferences primaryColor is invalid')
  }

  const booleanKeys = ['showBreadcrumb', 'showMenuToggle', 'showRouteTabs', 'uniqueOpened', 'showFooter', 'pageTransition'] as const
  for (const key of booleanKeys) {
    if (typeof preferences[key] !== 'boolean') {
      throw new UIPreferencesError(`stored UI preferences ${key} must be boolean`)
    }
  }

  const transitionName = preferences.transitionName
  if (typeof transitionName !== 'string' || !transitionNames.includes(transitionName as PageTransitionName)) {
    throw new UIPreferencesError('stored UI preferences transitionName is invalid')
  }

  return {
    theme,
    primaryColor: primaryColor.toUpperCase(),
    showBreadcrumb: preferences.showBreadcrumb as boolean,
    showMenuToggle: preferences.showMenuToggle as boolean,
    showRouteTabs: preferences.showRouteTabs as boolean,
    uniqueOpened: preferences.uniqueOpened as boolean,
    showFooter: preferences.showFooter as boolean,
    pageTransition: preferences.pageTransition as boolean,
    transitionName: transitionName as PageTransitionName,
  }
}

export function readUIPreferences(): UIPreferences {
  let storedValue: string | null
  try {
    storedValue = window.localStorage.getItem(uiPreferencesStorageKey)
  } catch (error: unknown) {
    throw new UIPreferencesError('failed to read UI preferences', { cause: error })
  }
  if (storedValue === null) return { ...defaultUIPreferences }

  let parsed: unknown
  try {
    parsed = JSON.parse(storedValue) as unknown
  } catch (error: unknown) {
    throw new UIPreferencesError('stored UI preferences JSON is invalid', { cause: error })
  }
  return parseStoredUIPreferences(parsed)
}

export function writeUIPreferences(preferences: UIPreferences): void {
  const value: StoredUIPreferences = {
    version: 1,
    preferences: parseStoredUIPreferences({ version: 1, preferences }),
  }
  try {
    window.localStorage.setItem(uiPreferencesStorageKey, JSON.stringify(value))
  } catch (error: unknown) {
    throw new UIPreferencesError('failed to write UI preferences', { cause: error })
  }
}

function closedRecord(value: unknown, expectedKeys: readonly string[], label: string): Record<string, unknown> {
  if (!isRecord(value)) {
    throw new UIPreferencesError(`${label} must be an object`)
  }
  const actualKeys = Object.keys(value).sort()
  const sortedExpectedKeys = [...expectedKeys].sort()
  if (actualKeys.length !== sortedExpectedKeys.length || actualKeys.some((key, index) => key !== sortedExpectedKeys[index])) {
    throw new UIPreferencesError(`${label} contains unexpected or missing fields`)
  }
  return value
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
