import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import { describe, expect, it, vi } from 'vitest'

const appHarness = vi.hoisted(() => {
  const app = {
    use: vi.fn(),
    mount: vi.fn(),
  }
  app.use.mockReturnValue(app)
  return {
    app,
    createApp: vi.fn(() => app),
    initializeLocale: vi.fn(),
    installPermissionGuard: vi.fn(),
  }
})

vi.mock('vue', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue')>(),
  createApp: appHarness.createApp,
}))
vi.mock('@src/App.vue', () => ({ default: {} }))
vi.mock('@src/i18n', () => ({
  appI18n: { name: 'i18n' },
  initializeLocale: appHarness.initializeLocale,
}))
vi.mock('@src/router', () => ({ router: { name: 'router' } }))
vi.mock('@src/store', () => ({ pinia: { name: 'pinia' } }))
vi.mock('@src/permission', () => ({ installPermissionGuard: appHarness.installPermissionGuard }))

describe('application bootstrap', () => {
  it('installs Element Plus with the Chinese locale', async () => {
    await import('@src/main')

    expect(appHarness.app.use).toHaveBeenCalledWith(ElementPlus, { locale: zhCn })
  })
})
