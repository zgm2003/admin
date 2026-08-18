import { beforeEach, describe, expect, it } from 'vitest'

import { appI18n, initializeLocale, localeStorageKey, readLocale, setLocale } from './index'
import { enUS } from './messages/en-US'
import { zhCN } from './messages/zh-CN'

describe('frontend i18n', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.lang = ''
    setLocale('zh-CN')
  })

  it('defaults to Chinese and updates the root language', () => {
    expect(initializeLocale()).toBe('zh-CN')
    expect(readLocale()).toBe('zh-CN')
    expect(document.documentElement.lang).toBe('zh-CN')
    expect(appI18n.global.t('navigation.dashboard')).toBe('工作台')
  })

  it('normalizes an invalid stored locale to Chinese', () => {
    localStorage.setItem(localeStorageKey, 'fr-FR')
    expect(initializeLocale()).toBe('zh-CN')
    expect(localStorage.getItem(localeStorageKey)).toBe('zh-CN')
  })

  it('persists English and keeps both catalogs exactly shaped', () => {
    setLocale('en-US')
    expect(localStorage.getItem(localeStorageKey)).toBe('en-US')
    expect(document.documentElement.lang).toBe('en-US')
    expect(appI18n.global.t('navigation.dashboard')).toBe('Dashboard')
    expect(Object.keys(enUS).sort()).toEqual(Object.keys(zhCN).sort())
  })
})
