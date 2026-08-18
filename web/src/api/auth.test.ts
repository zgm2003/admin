import { beforeEach, describe, expect, it, vi } from 'vitest'

import { refreshAccessCredential, request } from '../utils/request'
import { getCurrentUser, login, logout, refresh, register } from './auth'

vi.mock('../utils/request', () => ({ request: vi.fn(), refreshAccessCredential: vi.fn() }))

const requestMock = vi.mocked(request)
const refreshAccessCredentialMock = vi.mocked(refreshAccessCredential)

describe('auth API', () => {
  beforeEach(() => {
    requestMock.mockReset()
    refreshAccessCredentialMock.mockReset()
  })

  it('registers and validates the response', async () => {
    requestMock.mockResolvedValue({ userId: 1, username: 'admin', email: 'admin@example.com' })
    const input = { username: 'admin', email: 'admin@example.com', password: 'password', confirmPassword: 'password' }
    await expect(register(input)).resolves.toEqual({ userId: 1, username: 'admin', email: 'admin@example.com' })
    expect(requestMock).toHaveBeenCalledWith({ method: 'POST', url: '/api/v1/auth/register', data: input })
  })

  it('logs in and validates the credential', async () => {
    requestMock.mockResolvedValue({ accessToken: 'jwt', expiresIn: 900 })
    const input = { username: 'admin', password: 'password' }
    await expect(login(input)).resolves.toEqual({ accessToken: 'jwt', expiresIn: 900 })
    expect(requestMock).toHaveBeenCalledWith({ method: 'POST', url: '/api/v1/auth/login', data: input })
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
    requestMock.mockResolvedValue({ userId: 1, username: 'admin', email: 'admin@example.com' })
    await expect(getCurrentUser()).resolves.toEqual({ userId: 1, username: 'admin', email: 'admin@example.com' })
    expect(requestMock).toHaveBeenCalledWith({ method: 'GET', url: '/api/v1/auth/me' })
  })
})
