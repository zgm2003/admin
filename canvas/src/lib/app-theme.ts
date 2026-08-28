import type { ThemeConfig } from 'antd'
import { theme } from 'antd'

export function getAntThemeConfig(dark: boolean): ThemeConfig {
  const primary = dark ? '#fafafa' : '#171717'
  const primaryHover = dark ? '#ffffff' : '#000000'
  const primaryText = dark ? '#171717' : '#ffffff'
  return {
    algorithm: dark ? theme.darkAlgorithm : theme.defaultAlgorithm,
    cssVar: { key: dark ? 'infinite-canvas-dark' : 'infinite-canvas-light' },
    token: {
      colorPrimary: primary,
      colorInfo: primary,
      colorLink: primary,
      colorLinkHover: primaryHover,
      colorLinkActive: primary,
      colorTextLightSolid: primaryText,
      colorBgElevated: dark ? '#1c1917' : '#ffffff',
    },
    components: { Button: { primaryShadow: 'none' } },
  }
}
