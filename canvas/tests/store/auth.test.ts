import { describe, expect, it, vi } from 'vitest'

import type { CurrentUser, LoginConfig } from '@/api/auth/login'
import { AuthStore } from '@/store/auth'
import type { RequestClient } from '@/utils/request'

const user: CurrentUser = {
  userId: 7,
  username: 'canvas-user',
  email: 'user@example.com',
  phone: null,
  avatar: '',
  passwordSetRequired: false,
}

const config: LoginConfig = {
  loginTypes: [{ value: 'password', label: '密码' }],
  allowRegister: false,
}

function fakeClient(overrides: Partial<RequestClient> = {}): RequestClient {
  return {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    setAccessToken: vi.fn(),
    getAccessToken: vi.fn(() => ''),
    clearAccessToken: vi.fn(),
    ...overrides,
  }
}

describe('AuthStore', () => {
  it('loads login policy and restores an authenticated user after refresh', async () => {
    const client = fakeClient({
      get: vi.fn().mockResolvedValueOnce(config).mockResolvedValueOnce(user),
      post: vi.fn().mockResolvedValue({ accessToken: 'restored-token', expiresIn: 300, isNewUser: false, passwordSetRequired: false }),
    })
    const store = new AuthStore(client)

    await expect(store.bootstrap()).resolves.toBe('authenticated')
    expect(store.state.status).toBe('authenticated')
    expect(store.state.user).toEqual(user)
    expect(client.setAccessToken).toHaveBeenCalledWith('restored-token')
  })

  it('deduplicates concurrent bootstrap calls from development effect replays', async () => {
    let resolveConfig: ((value: LoginConfig) => void) | undefined
    const client = fakeClient({
      get: vi.fn()
        .mockImplementationOnce(() => new Promise<LoginConfig>((resolve) => { resolveConfig = resolve }))
        .mockResolvedValueOnce(user),
      post: vi.fn().mockResolvedValue({ accessToken: 'restored-token', expiresIn: 300, isNewUser: false, passwordSetRequired: false }),
    })
    const store = new AuthStore(client)
    const first = store.bootstrap()
    const second = store.bootstrap()
    resolveConfig?.(config)

    await expect(Promise.all([first, second])).resolves.toEqual(['authenticated', 'authenticated'])
    expect(client.get).toHaveBeenCalledTimes(2)
    expect(client.post).toHaveBeenCalledTimes(1)
  })

  it('deduplicates concurrent login config loads from the bootstrap and login page', async () => {
    let resolveConfig: ((value: LoginConfig) => void) | undefined
    const client = fakeClient({
      get: vi.fn().mockImplementation(() => new Promise<LoginConfig>((resolve) => { resolveConfig = resolve })),
    })
    const store = new AuthStore(client)
    const first = store.loadLoginConfig()
    const second = store.loadLoginConfig()
    resolveConfig?.(config)

    await expect(Promise.all([first, second])).resolves.toEqual([config, config])
    expect(client.get).toHaveBeenCalledOnce()
  })

  it('keeps login business failures in state without replacing them with a generic error', async () => {
    const client = fakeClient({
      get: vi.fn().mockResolvedValue(config),
      post: vi.fn().mockRejectedValue(new Error('凭据错误')),
    })
    const store = new AuthStore(client)

    await expect(store.login({ loginType: 'password', loginAccount: 'user@example.com', password: 'bad' })).rejects.toThrow('凭据错误')
    expect(store.state.status).toBe('error')
    expect(store.state.error).toBe('凭据错误')
  })

  it('clears the credential when loading the user after login fails', async () => {
    const client = fakeClient({
      post: vi.fn().mockResolvedValue({ accessToken: 'short-lived-token', expiresIn: 300, isNewUser: false, passwordSetRequired: false }),
      get: vi.fn().mockRejectedValue(new Error('用户信息读取失败')),
    })
    const store = new AuthStore(client)

    await expect(store.login({ loginType: 'password', loginAccount: 'user@example.com', password: 'secret' })).rejects.toThrow('用户信息读取失败')
    expect(client.clearAccessToken).toHaveBeenCalledOnce()
    expect(store.state.accessToken).toBe('')
  })

  it('clears session state when logout succeeds', async () => {
    const client = fakeClient({
      post: vi.fn().mockResolvedValue(undefined),
    })
    const store = new AuthStore(client)
    store.setAuthenticated({ accessToken: 'token', expiresIn: 300, isNewUser: false, passwordSetRequired: false }, user)

    await store.logout()

    expect(store.state.status).toBe('anonymous')
    expect(store.state.user).toBeNull()
    expect(client.clearAccessToken).toHaveBeenCalledOnce()
  })
})
