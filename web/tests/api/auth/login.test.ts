import { beforeEach, describe, expect, it, vi } from 'vitest'

import { refreshAccessCredential, request } from '@/utils/request'
import { getCurrentUser, login, logout, refresh } from '@/api/auth/login'

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

  it('logs in with only email and password', async () => {
    requestMock.mockResolvedValue({ accessToken: 'jwt', expiresIn: 900 })
    const input = { email: 'admin@example.com', password: 'password' }
    await expect(login(input)).resolves.toEqual({ accessToken: 'jwt', expiresIn: 900 })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'POST',
      url: '/api/v1/auth/login',
      data: input,
    })
  })

  it('refreshes and logs out without a JSON body', async () => {
    refreshAccessCredentialMock.mockResolvedValue({ accessToken: 'jwt', expiresIn: 900 })
    requestMock.mockResolvedValueOnce({})
    await expect(refresh()).resolves.toEqual({ accessToken: 'jwt', expiresIn: 900 })
    await expect(logout()).resolves.toBeUndefined()
    expect(refreshAccessCredentialMock).toHaveBeenCalledOnce()
    expect(requestMock).toHaveBeenCalledOnce()
    expect(requestMock).toHaveBeenCalledWith({ method: 'POST', url: '/api/v1/auth/logout' })
  })

  it('loads and validates the current user', async () => {
    requestMock.mockResolvedValue({
      userId: 1,
      username: 'admin',
      email: 'admin@example.com',
      phone: null,
      avatar: 'avatar/profile.png',
    })
    await expect(getCurrentUser()).resolves.toEqual({
      userId: 1,
      username: 'admin',
      email: 'admin@example.com',
      phone: null,
      avatar: 'avatar/profile.png',
    })
    expect(requestMock).toHaveBeenCalledWith({ method: 'GET', url: '/api/v1/auth/me' })
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
