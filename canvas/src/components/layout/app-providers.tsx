import type { ReactNode } from 'react'
import { App, ConfigProvider } from 'antd'
import { I18nextProvider } from 'react-i18next'
import { useEffect } from 'react'

import i18n from '@/i18n'
import { useThemeStore } from '@/store/theme'
import { getAntThemeConfig } from '@/lib/app-theme'

export function AppProviders({ children }: { children: ReactNode }) {
  const currentTheme = useThemeStore((state) => state.theme)
  useEffect(() => { document.documentElement.classList.toggle('dark', currentTheme === 'dark'); document.documentElement.style.colorScheme = currentTheme }, [currentTheme])
  return <I18nextProvider i18n={i18n}><ConfigProvider theme={getAntThemeConfig(currentTheme === 'dark')}><App>{children}</App></ConfigProvider></I18nextProvider>
}
