import type { App } from 'vue'
import { provideGlobalConfig } from 'element-plus/es/components/config-provider/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { createI18n } from 'vue-i18n'
import type { Language } from 'element-plus/es/locale'
import enUs from 'element-plus/es/locale/lang/en'
import zhCn from 'element-plus/es/locale/lang/zh-cn'

import { enUS } from './messages/en-US'
import { zhCN, type AppMessageKey } from './messages/zh-CN'
import { enUS as aiAgentEnUS, zhCN as aiAgentZhCN } from './messages/aiAgent'
import { enUS as aiToolEnUS, zhCN as aiToolZhCN } from './messages/aiTool'
import { enUS as aiProviderEnUS, zhCN as aiProviderZhCN } from './messages/aiProvider'
import { enUS as aiOfficialModelEnUS, zhCN as aiOfficialModelZhCN } from './messages/aiOfficialModel'
import { enUS as aiChatEnUS, zhCN as aiChatZhCN } from './messages/aiChat'
import { enUS as aiRunEnUS, zhCN as aiRunZhCN } from './messages/aiRun'

const appZhCN = { ...zhCN, ...aiAgentZhCN, ...aiToolZhCN, ...aiProviderZhCN, ...aiOfficialModelZhCN, ...aiChatZhCN, ...aiRunZhCN } as const
const appEnUS = { ...enUS, ...aiAgentEnUS, ...aiToolEnUS, ...aiProviderEnUS, ...aiOfficialModelEnUS, ...aiChatEnUS, ...aiRunEnUS } satisfies Record<keyof typeof appZhCN, string>

export type AppLocale = 'zh-CN' | 'en-US'
export type { AppMessageKey }
export const localeStorageKey = 'admin:locale'
const appMessageKeys: ReadonlySet<string> = new Set(Object.keys(appZhCN))

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
  messages: { 'zh-CN': appZhCN, 'en-US': appEnUS },
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

// Install outside component setup: both service dialogs and component providers
// inherit one reactive locale from the application's context.
export function installElementPlusLocale(app: App): void {
  provideGlobalConfig(
    {
      get locale() {
        return elementPlusLocaleFor(appI18n.global.locale.value === 'en-US' ? 'en-US' : 'zh-CN')
      },
    },
    app,
    true,
  )
  app.use(ElMessageBox)
}
