export type ThemeMode = 'light' | 'dark'

export const themeColorPresets = [
  '#409EFF',
  '#3B82F6',
  '#4F46E5',
  '#7C3AED',
  '#DB2777',
  '#DC2626',
  '#EA580C',
  '#CA8A04',
  '#059669',
  '#0891B2',
  '#475569',
] as const

const sixDigitHex = /^#[0-9a-fA-F]{6}$/

export function isSixDigitHexColor(value: unknown): value is string {
  return typeof value === 'string' && sixDigitHex.test(value)
}

export function applyTheme(theme: ThemeMode): void {
  document.documentElement.classList.toggle('dark', theme === 'dark')
  document.documentElement.style.colorScheme = theme

  // Element Plus changes its semantic surface variables when the root mode
  // changes. Re-derive the primary palette so a previously selected accent
  // remains legible in the new surface instead of retaining light-mode tints.
  const primary = readCssColor('--el-color-primary')
  if (primary !== null) applyPrimaryColor(primary)
}

export function mixHexColor(base: string, target: string, weight: number): string {
  const baseChannels = parseHex(base)
  const targetChannels = parseHex(target)
  if (!Number.isFinite(weight) || weight < 0 || weight > 1) {
    throw new Error('color mix weight must be between 0 and 1')
  }
  return `#${baseChannels
    .map((channel, index) =>
      Math.round(channel + (targetChannels[index] - channel) * weight)
        .toString(16)
        .padStart(2, '0'),
    )
    .join('')
    .toUpperCase()}`
}

export function applyPrimaryColor(color: string): void {
  if (!isSixDigitHexColor(color)) {
    throw new Error('primary color must be a six-digit hex color')
  }
  const normalized = color.toUpperCase()
  const channels = parseHex(normalized)
  const dark = document.documentElement.classList.contains('dark')
  const surface = readCssColor('--el-bg-color') ?? (dark ? '#191F2A' : '#FFFFFF')
  const style = document.documentElement.style
  style.setProperty('--el-color-primary', normalized)
  style.setProperty('--el-color-primary-rgb', channels.join(', '))
  for (const [suffix, weight] of [
    ['3', 0.3],
    ['5', 0.5],
    ['7', 0.7],
    ['8', 0.8],
    ['9', 0.9],
  ] as const) {
    style.setProperty(
      `--el-color-primary-light-${suffix}`,
      mixHexColor(normalized, surface, weight),
    )
  }
  style.setProperty(
    '--el-color-primary-dark-2',
    mixHexColor(normalized, dark ? '#FFFFFF' : '#000000', 0.2),
  )
}

function readCssColor(name: string): string | null {
  const computed = window.getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  if (isSixDigitHexColor(computed)) return computed
  const inline = document.documentElement.style.getPropertyValue(name).trim()
  return isSixDigitHexColor(inline) ? inline : null
}

function parseHex(value: string): [number, number, number] {
  if (!isSixDigitHexColor(value)) {
    throw new Error('color must be a six-digit hex color')
  }
  return [
    Number.parseInt(value.slice(1, 3), 16),
    Number.parseInt(value.slice(3, 5), 16),
    Number.parseInt(value.slice(5, 7), 16),
  ]
}
