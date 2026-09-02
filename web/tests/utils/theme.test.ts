import { beforeEach, describe, expect, it } from 'vitest'

import { applyPrimaryColor, applyTheme, isSixDigitHexColor, mixHexColor } from '@/utils/theme'

describe('theme', () => {
  beforeEach(() => {
    document.documentElement.classList.remove('dark')
    document.documentElement.style.removeProperty('color-scheme')
  })

  it('applies the light Element Plus theme', () => {
    applyTheme('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(document.documentElement.style.colorScheme).toBe('light')
  })

  it('applies the dark Element Plus theme', () => {
    applyTheme('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(document.documentElement.style.colorScheme).toBe('dark')
  })

  it('derives the Element Plus primary palette from one six-digit hex color', () => {
    applyPrimaryColor('#409EFF')

    const style = document.documentElement.style
    expect(style.getPropertyValue('--el-color-primary')).toBe('#409EFF')
    expect(style.getPropertyValue('--el-color-primary-rgb')).toBe('64, 158, 255')
    expect(style.getPropertyValue('--el-color-primary-light-3')).toBe('#79BBFF')
    expect(style.getPropertyValue('--el-color-primary-light-5')).toBe('#A0CFFF')
    expect(style.getPropertyValue('--el-color-primary-dark-2')).toBe('#337ECC')
  })

  it('rejects malformed colors instead of applying a partial palette', () => {
    expect(() => applyPrimaryColor('blue')).toThrow('primary color must be a six-digit hex color')
  })

  it('recognizes only six-digit hex colors', () => {
    expect(isSixDigitHexColor('#FFFFFF')).toBe(true)
    expect(isSixDigitHexColor('#1a2B3c')).toBe(true)
    expect(isSixDigitHexColor('#fff')).toBe(false)
    expect(isSixDigitHexColor('#12345678')).toBe(false)
    expect(isSixDigitHexColor(null)).toBe(false)
  })

  it('mixes channels deterministically', () => {
    expect(mixHexColor('#000000', '#FFFFFF', 0.5)).toBe('#808080')
  })
})
