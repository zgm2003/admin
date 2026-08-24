export type ThemeMode = 'light' | 'dark'

const sixDigitHex = /^#[0-9a-fA-F]{6}$/

export function applyTheme(theme: ThemeMode): void {
  document.documentElement.classList.toggle('dark', theme === 'dark')
  document.documentElement.style.colorScheme = theme
}

export function mixHexColor(base: string, target: string, weight: number): string {
  const baseChannels = parseHex(base)
  const targetChannels = parseHex(target)
  if (!Number.isFinite(weight) || weight < 0 || weight > 1) {
    throw new Error('color mix weight must be between 0 and 1')
  }
  return `#${baseChannels.map((channel, index) => (
    Math.round(channel + (targetChannels[index] - channel) * weight)
      .toString(16)
      .padStart(2, '0')
  )).join('').toUpperCase()}`
}

export function applyPrimaryColor(color: string): void {
  if (!sixDigitHex.test(color)) {
    throw new Error('primary color must be a six-digit hex color')
  }
  const normalized = color.toUpperCase()
  const channels = parseHex(normalized)
  const style = document.documentElement.style
  style.setProperty('--el-color-primary', normalized)
  style.setProperty('--el-color-primary-rgb', channels.join(', '))
  for (const [suffix, weight] of [['3', 0.3], ['5', 0.5], ['7', 0.7], ['8', 0.8], ['9', 0.9]] as const) {
    style.setProperty(`--el-color-primary-light-${suffix}`, mixHexColor(normalized, '#FFFFFF', weight))
  }
  style.setProperty('--el-color-primary-dark-2', mixHexColor(normalized, '#000000', 0.2))
}

function parseHex(value: string): [number, number, number] {
  if (!sixDigitHex.test(value)) {
    throw new Error('color must be a six-digit hex color')
  }
  return [
    Number.parseInt(value.slice(1, 3), 16),
    Number.parseInt(value.slice(3, 5), 16),
    Number.parseInt(value.slice(5, 7), 16),
  ]
}
