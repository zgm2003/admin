// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/auth/login'
import { refreshAccessCredential, request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('delegates refresh to the shared single-flight authentication coordinator', async () => {
    const credential = {
      accessToken: 'token',
      expiresIn: 900,
      isNewUser: false,
      passwordSetRequired: false,
    }
    vi.mocked(refreshAccessCredential).mockResolvedValue(credential)
    await expect(api.refresh()).resolves.toBe(credential)
    expect(refreshAccessCredential).toHaveBeenCalledOnce()
    expect(request.post).not.toHaveBeenCalled()
  })
  it('getCaptcha preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getCaptcha()
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/v1/auth/captcha')
  })
  it('getCaptcha propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getCaptcha()).rejects.toBe(error)
  })
  it('login preserves the HTTP contract and backend data', async () => {
    const input: Parameters<typeof api.login>[0] = {
      loginType: 'password',
      loginAccount: 'sample',
      password: 'sample',
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.login(input)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/v1/auth/login', input)
  })
  it('login propagates request failures unchanged', async () => {
    const input: Parameters<typeof api.login>[0] = {
      loginType: 'password',
      loginAccount: 'sample',
      password: 'sample',
    }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.login(input)).rejects.toBe(error)
  })
  it('getLoginConfig preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getLoginConfig()
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/v1/auth/login-config')
  })
  it('getLoginConfig propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getLoginConfig()).rejects.toBe(error)
  })
  it('sendLoginCode preserves the HTTP contract and backend data', async () => {
    const account: Parameters<typeof api.sendLoginCode>[0] = 'sample'
    const loginType: Parameters<typeof api.sendLoginCode>[1] = 'email'
    const scene: Parameters<typeof api.sendLoginCode>[2] = 'login'
    const challengeId: Parameters<typeof api.sendLoginCode>[3] = 'sample'
    const captcha: Parameters<typeof api.sendLoginCode>[4] = {
      captchaId: 'sample',
      captchaAnswer: { x: 1, y: 1 },
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.sendLoginCode(account, loginType, scene, challengeId, captcha)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/v1/auth/send-code', {
      account,
      loginType,
      scene,
      challengeId,
      captchaId: captcha?.captchaId,
      captchaAnswer: captcha?.captchaAnswer,
    })
  })
  it('sendLoginCode propagates request failures unchanged', async () => {
    const account: Parameters<typeof api.sendLoginCode>[0] = 'sample'
    const loginType: Parameters<typeof api.sendLoginCode>[1] = 'email'
    const scene: Parameters<typeof api.sendLoginCode>[2] = 'login'
    const challengeId: Parameters<typeof api.sendLoginCode>[3] = 'sample'
    const captcha: Parameters<typeof api.sendLoginCode>[4] = {
      captchaId: 'sample',
      captchaAnswer: { x: 1, y: 1 },
    }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.sendLoginCode(account, loginType, scene, challengeId, captcha)).rejects.toBe(
      error,
    )
  })
  it('forgotPassword preserves the HTTP contract and backend data', async () => {
    const account: Parameters<typeof api.forgotPassword>[0] = 'sample'
    const loginType: Parameters<typeof api.forgotPassword>[1] = 'email'
    const captcha: Parameters<typeof api.forgotPassword>[2] = {
      captchaId: 'sample',
      captchaAnswer: { x: 1, y: 1 },
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.forgotPassword(account, loginType, captcha)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/v1/auth/password/forgot', {
      account,
      loginType,
      captchaId: captcha?.captchaId,
      captchaAnswer: captcha?.captchaAnswer,
    })
  })
  it('forgotPassword propagates request failures unchanged', async () => {
    const account: Parameters<typeof api.forgotPassword>[0] = 'sample'
    const loginType: Parameters<typeof api.forgotPassword>[1] = 'email'
    const captcha: Parameters<typeof api.forgotPassword>[2] = {
      captchaId: 'sample',
      captchaAnswer: { x: 1, y: 1 },
    }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.forgotPassword(account, loginType, captcha)).rejects.toBe(error)
  })
  it('resetPassword preserves the HTTP contract and backend data', async () => {
    const input: Parameters<typeof api.resetPassword>[0] = {
      account: 'sample',
      loginType: 'email',
      challengeId: 'sample',
      code: 'sample',
      newPassword: 'sample',
      confirmPassword: 'sample',
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.resetPassword(input)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/v1/auth/password/reset', input)
  })
  it('resetPassword propagates request failures unchanged', async () => {
    const input: Parameters<typeof api.resetPassword>[0] = {
      account: 'sample',
      loginType: 'email',
      challengeId: 'sample',
      code: 'sample',
      newPassword: 'sample',
      confirmPassword: 'sample',
    }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.resetPassword(input)).rejects.toBe(error)
  })
  it('logout preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.logout()
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/v1/auth/logout', undefined)
  })
  it('logout propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.logout()).rejects.toBe(error)
  })
  it('getCurrentUser preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getCurrentUser()
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/v1/auth/me')
  })
  it('getCurrentUser propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getCurrentUser()).rejects.toBe(error)
  })
})
