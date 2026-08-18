import { beforeEach, describe, expect, it } from 'vitest'

import { applyTheme, initializeTheme, readTheme, toggleTheme } from './theme'

describe('theme', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    document.documentElement.style.removeProperty('color-scheme')
  })

  it('defaults to the light Element Plus theme', () => {
    expect(initializeTheme()).toBe('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(document.documentElement.style.colorScheme).toBe('light')
  })

  it('restores and applies a stored dark theme', () => {
    localStorage.setItem('admin:theme', 'dark')

    expect(initializeTheme()).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(document.documentElement.style.colorScheme).toBe('dark')
  })

  it('persists an explicit toggle', () => {
    expect(toggleTheme('light')).toBe('dark')
    expect(readTheme()).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)

    applyTheme('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })
})
