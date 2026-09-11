import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  forgotPassword,
  getCurrentUser,
  getLoginConfig,
  login,
  logout,
  refresh,
  resetPassword,
  sendLoginCode,
} from '@/api/auth/login'
import { refreshAccessCredential, request } from '@/utils/request'

vi.mock('@/utils/request', () => ({
  request: vi.fn(),
  refreshAccessCredential: vi.fn(),
  ProtocolError: class ProtocolError extends Error {},
}))

const requestMock = vi.mocked(request)
const refreshAccessCredentialMock = vi.mocked(refreshAccessCredential)

describe('auth API', () => {
  beforeEach(() => {
    requestMock.mockReset()
    refreshAccessCredentialMock.mockReset()
  })

  it('logs in with password and parses isNewUser', async () => {
    requestMock.mockResolvedValue({
      accessToken: 'jwt',
      expiresIn: 900,
      isNewUser: false,
      passwordSetRequired: false,
    })
    const input = {
      loginType: 'password' as const,
      loginAccount: 'admin@example.com',
      password: 'password',
    }
    await expect(login(input)).resolves.toEqual({
      accessToken: 'jwt',
      expiresIn: 900,
      isNewUser: false,
      passwordSetRequired: false,
    })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'POST',
      url: '/api/v1/auth/login',
      data: input,
    })
  })

  it('logs in with an email verification code', async () => {
    requestMock.mockResolvedValue({
      accessToken: 'jwt',
      expiresIn: 900,
      isNewUser: true,
      passwordSetRequired: true,
    })
    await expect(
      login({
        loginType: 'email',
        loginAccount: 'admin@example.com',
        challengeId: 'challenge-1',
        code: '123456',
      }),
    ).resolves.toEqual({
      accessToken: 'jwt',
      expiresIn: 900,
      isNewUser: true,
      passwordSetRequired: true,
    })
  })

  it('loads the effective login config', async () => {
    requestMock.mockResolvedValue({
      loginTypes: [
        { value: 'password', label: '密码' },
        { value: 'email', label: '邮箱验证码' },
      ],
      allowRegister: false,
    })
    await expect(getLoginConfig()).resolves.toEqual({
      loginTypes: [
        { value: 'password', label: '密码' },
        { value: 'email', label: '邮箱验证码' },
      ],
      allowRegister: false,
    })
    expect(requestMock).toHaveBeenCalledWith({ method: 'GET', url: '/api/v1/auth/login-config' })
  })

  it('sends a login verification code', async () => {
    requestMock.mockResolvedValue({
      challengeId: 'challenge-1',
      expiresAt: '2026-09-07T10:00:00Z',
      resendAfterSeconds: 60,
    })
    await expect(
      sendLoginCode('admin@example.com', 'email', 'login', 'challenge-1'),
    ).resolves.toEqual({
      challengeId: 'challenge-1',
      expiresAt: '2026-09-07T10:00:00Z',
      resendAfterSeconds: 60,
    })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'POST',
      url: '/api/v1/auth/send-code',
      data: {
        account: 'admin@example.com',
        loginType: 'email',
        scene: 'login',
        challengeId: 'challenge-1',
      },
    })
  })

  it('rejects an invalid login type in the config', async () => {
    requestMock.mockResolvedValue({
      loginTypes: [{ value: 'wechat', label: '微信' }],
      allowRegister: false,
    })
    await expect(getLoginConfig()).rejects.toThrow('login config option value is invalid')
  })

  it('rejects non-boolean credential and config flags', async () => {
    requestMock.mockResolvedValueOnce({
      accessToken: 'jwt',
      expiresIn: 900,
      isNewUser: 1,
      passwordSetRequired: false,
    })
    await expect(
      login({ loginType: 'password', loginAccount: 'admin@example.com', password: 'password' }),
    ).rejects.toThrow('access credential.isNewUser must be a boolean')

    requestMock.mockResolvedValueOnce({
      loginTypes: [{ value: 'password', label: 'Password' }],
      allowRegister: 1,
    })
    await expect(getLoginConfig()).rejects.toThrow('login config.allowRegister must be a boolean')
  })

  it('accepts phone in the effective config when the backend reports it ready', async () => {
    requestMock.mockResolvedValue({
      loginTypes: [{ value: 'phone', label: 'Phone code' }],
      allowRegister: false,
    })
    await expect(getLoginConfig()).resolves.toEqual({
      loginTypes: [{ value: 'phone', label: 'Phone code' }],
      allowRegister: false,
    })
  })

  it('refreshes and logs out without a JSON body', async () => {
    refreshAccessCredentialMock.mockResolvedValue({
      accessToken: 'jwt',
      expiresIn: 900,
      isNewUser: false,
      passwordSetRequired: false,
    })
    requestMock.mockResolvedValueOnce({})
    await expect(refresh()).resolves.toEqual({
      accessToken: 'jwt',
      expiresIn: 900,
      isNewUser: false,
      passwordSetRequired: false,
    })
    await expect(logout()).resolves.toBeUndefined()
    expect(refreshAccessCredentialMock).toHaveBeenCalledOnce()
    expect(requestMock).toHaveBeenCalledOnce()
    expect(requestMock).toHaveBeenCalledWith({ method: 'POST', url: '/api/v1/auth/logout' })
  })

  it('rejects empty, duplicate, and malformed effective login config', async () => {
    requestMock.mockResolvedValueOnce({ loginTypes: [], allowRegister: false })
    await expect(getLoginConfig()).rejects.toThrow('login config.loginTypes must not be empty')

    requestMock.mockResolvedValueOnce({
      loginTypes: [
        { value: 'password', label: 'Password' },
        { value: 'password', label: 'Password' },
      ],
      allowRegister: false,
    })
    await expect(getLoginConfig()).rejects.toThrow('login config.loginTypes contains duplicates')
  })

  it('rejects a malformed verification-code expiry', async () => {
    requestMock.mockResolvedValue({
      challengeId: 'challenge-1',
      expiresAt: 'not-a-timestamp',
      resendAfterSeconds: 60,
    })
    await expect(
      sendLoginCode('admin@example.com', 'email', 'login', 'challenge-1'),
    ).rejects.toThrow('send code.expiresAt must be a timestamp')
  })

  it('accepts zero resend wait when shared mail quota remains', async () => {
    requestMock.mockResolvedValue({
      challengeId: 'challenge-1',
      expiresAt: '2026-09-08T10:00:00Z',
      resendAfterSeconds: 0,
    })
    const result = await sendLoginCode('admin@example.com', 'email', 'login', 'challenge-1')
    expect(result.resendAfterSeconds).toBe(0)
  })

  it('rejects a missing or invalid resend window', async () => {
    requestMock.mockResolvedValue({
      challengeId: 'challenge-1',
      expiresAt: '2026-09-07T10:00:00Z',
    })
    await expect(
      sendLoginCode('admin@example.com', 'email', 'login', 'challenge-1'),
    ).rejects.toThrow('send code response has invalid fields')

    requestMock.mockResolvedValue({
      challengeId: 'challenge-1',
      expiresAt: '2026-09-07T10:00:00Z',
      resendAfterSeconds: -1,
    })
    await expect(
      sendLoginCode('admin@example.com', 'email', 'login', 'challenge-1'),
    ).rejects.toThrow('send code.resendAfterSeconds must be between 0 and 86400')

    requestMock.mockResolvedValue({
      challengeId: 'challenge-1',
      expiresAt: '2026-09-07T10:00:00Z',
      resendAfterSeconds: 90000,
    })
    await expect(
      sendLoginCode('admin@example.com', 'email', 'login', 'challenge-1'),
    ).rejects.toThrow('send code.resendAfterSeconds must be between 0 and 86400')

    requestMock.mockResolvedValue({
      challengeId: 'challenge-1',
      expiresAt: '2026-09-07T10:00:00Z',
      resendAfterSeconds: 1.5,
    })
    await expect(
      sendLoginCode('admin@example.com', 'email', 'login', 'challenge-1'),
    ).rejects.toThrow('send code.resendAfterSeconds must be an integer')
  })

  it('loads and validates the current user', async () => {
    requestMock.mockResolvedValue({
      userId: 1,
      username: 'admin',
      email: 'admin@example.com',
      phone: null,
      avatar: 'avatar/profile.png',
      passwordSetRequired: false,
    })
    await expect(getCurrentUser()).resolves.toEqual({
      userId: 1,
      username: 'admin',
      email: 'admin@example.com',
      phone: null,
      avatar: 'avatar/profile.png',
      passwordSetRequired: false,
    })
    expect(requestMock).toHaveBeenCalledWith({ method: 'GET', url: '/api/v1/auth/me' })
  })

  it('requests a forgot-password code and parses the resend window', async () => {
    requestMock.mockResolvedValue({
      challengeId: 'challenge-9',
      expiresAt: '2026-09-07T10:00:00Z',
      resendAfterSeconds: 60,
    })
    await expect(forgotPassword('admin@example.com', 'email')).resolves.toEqual({
      challengeId: 'challenge-9',
      expiresAt: '2026-09-07T10:00:00Z',
      resendAfterSeconds: 60,
    })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'POST',
      url: '/api/v1/auth/password/forgot',
      data: { account: 'admin@example.com', loginType: 'email' },
    })
  })

  it('resets the password and requires an empty object result', async () => {
    const input = {
      code: '123456',
      newPassword: 'NewPassw0rd!',
      confirmPassword: 'NewPassw0rd!',
    }
    requestMock.mockResolvedValueOnce({})
    await expect(resetPassword({ account: 'admin@example.com', loginType: 'email', challengeId: 'challenge-1', ...input })).resolves.toBeUndefined()
    expect(requestMock).toHaveBeenCalledWith({
      method: 'POST',
      url: '/api/v1/auth/password/reset',
      data: { account: 'admin@example.com', loginType: 'email', challengeId: 'challenge-1', ...input },
    })

    requestMock.mockResolvedValueOnce({ unexpected: true })
    await expect(resetPassword({ account: 'admin@example.com', loginType: 'email', challengeId: 'challenge-1', ...input })).rejects.toThrow('reset password result')
  })

  it('rejects a current user response without the required phone field', async () => {
    requestMock.mockResolvedValue({ userId: 1, username: 'admin', email: 'admin@example.com' })
    await expect(getCurrentUser()).rejects.toThrow('current user response')
  })

  it('rejects a current user response with a non-string phone', async () => {
    requestMock.mockResolvedValue({
      userId: 1,
      username: 'admin',
      email: 'admin@example.com',
      phone: 1,
    })
    await expect(getCurrentUser()).rejects.toThrow('current user response')
  })
})
