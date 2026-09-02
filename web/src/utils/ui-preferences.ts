import { isSixDigitHexColor, type ThemeMode } from './theme'

export type PageTransitionName = 'fade' | 'slide-left' | 'zoom'
export type UIPreferencesOperation = 'read' | 'write'

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

interface PersistedUIPreferencesV2 {
  primaryColor: string
  showBreadcrumb: boolean
  showMenuToggle: boolean
  showRouteTabs: boolean
  uniqueOpened: boolean
  showFooter: boolean
  pageTransition: boolean
  transitionName: PageTransitionName
}

interface StoredUIPreferencesV2 {
  version: 2
  preferences: PersistedUIPreferencesV2
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

const runtimePreferenceKeys = [
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
const persistedPreferenceKeys = runtimePreferenceKeys.filter((key) => key !== 'theme')
const booleanPreferenceKeys = [
  'showBreadcrumb',
  'showMenuToggle',
  'showRouteTabs',
  'uniqueOpened',
  'showFooter',
  'pageTransition',
] as const
const transitionNames: readonly PageTransitionName[] = ['fade', 'slide-left', 'zoom']

export class UIPreferencesError extends Error {
  public readonly operation: UIPreferencesOperation

  constructor(operation: UIPreferencesOperation, message: string, options?: ErrorOptions) {
    super(message, options)
    this.operation = operation
    this.name = 'UIPreferencesError'
  }
}

export function parseStoredUIPreferences(value: unknown): UIPreferences {
  const stored = closedRecord(value, ['preferences', 'version'], 'stored UI preferences', 'read')
  if (stored.version !== 2) {
    throw new UIPreferencesError('read', 'stored UI preferences version must be 2')
  }

  const preferences = closedRecord(
    stored.preferences,
    persistedPreferenceKeys,
    'stored UI preferences preferences',
    'read',
  )
  return { theme: 'light', ...parsePersistedPreferenceFields(preferences, 'read') }
}

export function readUIPreferences(): UIPreferences {
  let storedValue: string | null
  try {
    storedValue = window.localStorage.getItem(uiPreferencesStorageKey)
  } catch (error: unknown) {
    throw new UIPreferencesError('read', 'failed to read UI preferences', { cause: error })
  }
  if (storedValue === null) return { ...defaultUIPreferences }

  let parsed: unknown
  try {
    parsed = JSON.parse(storedValue) as unknown
  } catch (error: unknown) {
    throw new UIPreferencesError('read', 'stored UI preferences JSON is invalid', { cause: error })
  }

  const stored = closedRecord(parsed, ['preferences', 'version'], 'stored UI preferences', 'read')
  if (stored.version === 1) {
    const migrated = parseStoredUIPreferencesV1(parsed)
    writeUIPreferences(migrated)
    return migrated
  }
  return parseStoredUIPreferences(parsed)
}

export function writeUIPreferences(preferences: UIPreferences): void {
  const value: StoredUIPreferencesV2 = {
    version: 2,
    preferences: toPersistedUIPreferences(preferences),
  }
  try {
    window.localStorage.setItem(uiPreferencesStorageKey, JSON.stringify(value))
  } catch (error: unknown) {
    throw new UIPreferencesError('write', 'failed to write UI preferences', { cause: error })
  }
}

function parseStoredUIPreferencesV1(value: unknown): UIPreferences {
  const stored = closedRecord(value, ['preferences', 'version'], 'stored UI preferences', 'read')
  if (stored.version !== 1) {
    throw new UIPreferencesError('read', 'stored UI preferences version must be 1')
  }

  const preferences = closedRecord(
    stored.preferences,
    runtimePreferenceKeys,
    'stored UI preferences preferences',
    'read',
  )
  const theme = preferences.theme
  if (theme !== 'light' && theme !== 'dark') {
    throw new UIPreferencesError('read', 'stored UI preferences theme is invalid')
  }
  return { theme: 'light', ...parsePersistedPreferenceFields(preferences, 'read') }
}

function toPersistedUIPreferences(preferences: UIPreferences): PersistedUIPreferencesV2 {
  const validated = parseRuntimeUIPreferences(preferences, 'write')
  return {
    primaryColor: validated.primaryColor,
    showBreadcrumb: validated.showBreadcrumb,
    showMenuToggle: validated.showMenuToggle,
    showRouteTabs: validated.showRouteTabs,
    uniqueOpened: validated.uniqueOpened,
    showFooter: validated.showFooter,
    pageTransition: validated.pageTransition,
    transitionName: validated.transitionName,
  }
}

function parseRuntimeUIPreferences(
  value: unknown,
  operation: UIPreferencesOperation,
): UIPreferences {
  const preferences = closedRecord(value, runtimePreferenceKeys, 'UI preferences', operation)
  const theme = preferences.theme
  if (theme !== 'light' && theme !== 'dark') {
    throw new UIPreferencesError(operation, 'UI preferences theme is invalid')
  }
  return { theme, ...parsePersistedPreferenceFields(preferences, operation) }
}

function parsePersistedPreferenceFields(
  preferences: Record<string, unknown>,
  operation: UIPreferencesOperation,
): PersistedUIPreferencesV2 {
  const primaryColor = preferences.primaryColor
  if (!isSixDigitHexColor(primaryColor)) {
    throw new UIPreferencesError(operation, 'UI preferences primaryColor is invalid')
  }

  for (const key of booleanPreferenceKeys) {
    if (typeof preferences[key] !== 'boolean') {
      throw new UIPreferencesError(operation, `UI preferences ${key} must be boolean`)
    }
  }

  const transitionName = preferences.transitionName
  if (
    typeof transitionName !== 'string' ||
    !transitionNames.includes(transitionName as PageTransitionName)
  ) {
    throw new UIPreferencesError(operation, 'UI preferences transitionName is invalid')
  }

  return {
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

function closedRecord(
  value: unknown,
  expectedKeys: readonly string[],
  label: string,
  operation: UIPreferencesOperation,
): Record<string, unknown> {
  if (!isRecord(value)) {
    throw new UIPreferencesError(operation, `${label} must be an object`)
  }
  const actualKeys = Object.keys(value).sort()
  const sortedExpectedKeys = [...expectedKeys].sort()
  if (
    actualKeys.length !== sortedExpectedKeys.length ||
    actualKeys.some((key, index) => key !== sortedExpectedKeys[index])
  ) {
    throw new UIPreferencesError(operation, `${label} contains unexpected or missing fields`)
  }
  return value
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
