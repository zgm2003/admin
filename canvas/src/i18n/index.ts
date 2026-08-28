import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

export type AppLocale = 'zh-CN' | 'en-US'
export const localeStorageKey = 'admin:canvas-locale'

const resources = {
  'zh-CN': { translation: {
    'brand.name': 'Canvas',
    'brand.eyebrow': '视觉创作工作台',
    'brand.description': '把灵感、素材与智能工具放进同一张无限画布。',
    'locale.zh': '中文',
    'locale.en': 'English',
  } },
  'en-US': { translation: {
    'brand.name': 'Canvas',
    'brand.eyebrow': 'Visual creation workspace',
    'brand.description': 'Bring ideas, assets, and intelligent tools together on one infinite canvas.',
    'locale.zh': '中文',
    'locale.en': 'English',
  } },
} as const

function initialLocale(): AppLocale {
  const value = window.localStorage.getItem(localeStorageKey)
  return value === 'en-US' ? 'en-US' : 'zh-CN'
}

void i18n.use(initReactI18next).init({
  resources,
  lng: initialLocale(),
  fallbackLng: 'zh-CN',
  interpolation: { escapeValue: false },
})

export function setLocale(locale: AppLocale): void {
  window.localStorage.setItem(localeStorageKey, locale)
  document.documentElement.lang = locale
  void i18n.changeLanguage(locale)
}

export default i18n
