import ElementPlus from 'element-plus'
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
    readLocale: vi.fn(() => 'en-US'),
    elementPlusLocaleFor: vi.fn(() => ({ name: 'english-locale' })),
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
  readLocale: appHarness.readLocale,
  elementPlusLocaleFor: appHarness.elementPlusLocaleFor,
}))
vi.mock('@src/router', () => ({ router: { name: 'router' } }))
vi.mock('@src/store', () => ({ pinia: { name: 'pinia' } }))
vi.mock('@src/permission', () => ({ installPermissionGuard: appHarness.installPermissionGuard }))

describe('application bootstrap', () => {
  it('installs Element Plus with the persisted startup locale', async () => {
    await import('@src/main')

    expect(appHarness.readLocale).toHaveBeenCalledOnce()
    expect(appHarness.elementPlusLocaleFor).toHaveBeenCalledWith('en-US')
    expect(appHarness.app.use).toHaveBeenCalledWith(ElementPlus, {
      locale: { name: 'english-locale' },
    })
  })
})
