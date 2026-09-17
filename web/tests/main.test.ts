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
    installRouteLoadRecovery: vi.fn(),
  }
})

const elementPlusStyleHarness = vi.hoisted(() => ({
  message: vi.fn(),
  messageBox: vi.fn(),
  notification: vi.fn(),
}))

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
vi.mock('@/router/routeLoadRecovery', () => ({
  installRouteLoadRecovery: appHarness.installRouteLoadRecovery,
}))
vi.mock('element-plus/es/components/message/style/css', () => {
  elementPlusStyleHarness.message()
  return {}
})
vi.mock('element-plus/es/components/message-box/style/css', () => {
  elementPlusStyleHarness.messageBox()
  return {}
})
vi.mock('element-plus/es/components/notification/style/css', () => {
  elementPlusStyleHarness.notification()
  return {}
})

describe('application bootstrap', () => {
  it('bootstraps the app with programmatic Element Plus styles but without the full bundle', async () => {
    await import('@/main')

    expect(elementPlusStyleHarness.message).toHaveBeenCalledOnce()
    expect(elementPlusStyleHarness.messageBox).toHaveBeenCalledOnce()
    expect(elementPlusStyleHarness.notification).toHaveBeenCalledOnce()
    expect(appHarness.initializeLocale).toHaveBeenCalledOnce()
    expect(appHarness.installRouteLoadRecovery).toHaveBeenCalledWith({ name: 'router' })
    expect(appHarness.app.use).toHaveBeenCalledWith({ name: 'pinia' })
    expect(appHarness.app.use).toHaveBeenCalledWith({ name: 'router' })
    expect(appHarness.app.use).toHaveBeenCalledWith({ name: 'i18n' })
  })
})
