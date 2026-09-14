import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

import enUS from '@/i18n/locales/en-US'
import zhCN from '@/i18n/locales/zh-CN'

export type AppLocale = 'zh-CN' | 'en-US'

export const localeStorageKey = 'admin:canvas-locale'

const resources = {
  'zh-CN': { translation: zhCN },
  'en-US': { translation: enUS },
}

function initialLocale(): AppLocale {
  const value = window.localStorage.getItem(localeStorageKey)
  return value === 'en-US' ? 'en-US' : 'zh-CN'
}

void i18n.use(initReactI18next).init({
  resources,
  lng: initialLocale(),
  fallbackLng: 'zh-CN',
  supportedLngs: ['zh-CN', 'en-US'],
  initAsync: false,
  interpolation: { escapeValue: false },
  react: { useSuspense: false },
})

export function setLocale(locale: AppLocale) {
  window.localStorage.setItem(localeStorageKey, locale)
  document.documentElement.lang = locale
  return i18n.changeLanguage(locale)
}

export function changeAppLocale(locale: AppLocale) {
  return setLocale(locale)
}

export default i18n
