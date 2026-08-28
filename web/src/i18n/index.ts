import { createI18n } from 'vue-i18n'
import type { Language } from 'element-plus/es/locale'
import enUs from 'element-plus/es/locale/lang/en'
import zhCn from 'element-plus/es/locale/lang/zh-cn'

import { enUS } from './messages/en-US'
import { zhCN, type AppMessageKey } from './messages/zh-CN'

export type AppLocale = 'zh-CN' | 'en-US'
export type { AppMessageKey }
export const localeStorageKey = 'admin:locale'
const appMessageKeys: ReadonlySet<string> = new Set(Object.keys(zhCN))

export function isAppMessageKey(value: string): value is AppMessageKey {
  return appMessageKeys.has(value)
}

function isAppLocale(value: string | null): value is AppLocale {
  return value === 'zh-CN' || value === 'en-US'
}

export function readLocale(): AppLocale {
  const stored = window.localStorage.getItem(localeStorageKey)
  return isAppLocale(stored) ? stored : 'zh-CN'
}

export const appI18n = createI18n({
  legacy: false,
  locale: readLocale(),
  fallbackLocale: false,
  flatJson: true,
  messages: { 'zh-CN': zhCN, 'en-US': enUS },
})

export function setLocale(locale: AppLocale): void {
  appI18n.global.locale.value = locale
  window.localStorage.setItem(localeStorageKey, locale)
  document.documentElement.lang = locale
}

export function initializeLocale(): AppLocale {
  const locale = readLocale()
  setLocale(locale)
  return locale
}

export function elementPlusLocaleFor(locale: AppLocale): Language {
  return locale === 'en-US' ? enUs : zhCn
}
