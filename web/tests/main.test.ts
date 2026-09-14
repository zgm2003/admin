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
  ...(await importOriginal<typeof import('vue')>()),
  createApp: appHarness.createApp,
}))
vi.mock('@/App.vue', () => ({ default: {} }))
vi.mock('@/i18n', () => ({
  appI18n: { name: 'i18n' },
  initializeLocale: appHarness.initializeLocale,
}))
vi.mock('@/router', () => ({ router: { name: 'router' } }))
vi.mock('@/store', () => ({ pinia: { name: 'pinia' } }))
vi.mock('@/permission', () => ({ installPermissionGuard: appHarness.installPermissionGuard }))

describe('application bootstrap', () => {
  it('bootstraps the app without eagerly installing the full Element Plus bundle', async () => {
    await import('@/main')

    expect(appHarness.initializeLocale).toHaveBeenCalledOnce()
    expect(appHarness.app.use).toHaveBeenCalledWith({ name: 'pinia' })
    expect(appHarness.app.use).toHaveBeenCalledWith({ name: 'router' })
    expect(appHarness.app.use).toHaveBeenCalledWith({ name: 'i18n' })
  })
})
