import { beforeEach, describe, expect, it, vi } from 'vitest'
import { request } from '@src/utils/request'
import { changePassword, getAccountProfile, updateAccountProfile } from '@src/api/account'

vi.mock('@src/utils/request', () => ({ request: vi.fn(), ProtocolError: class ProtocolError extends Error {} }))
const requestMock = vi.mocked(request)

describe('account API', () => {
  beforeEach(() => requestMock.mockReset())

  it('uses the admin account endpoints and preserves profile fields', async () => {
    const profile = { userId: 7, username: 'alice', email: 'alice@example.com', phone: null, birthday: '2000-01-02', gender: 2 }
    requestMock.mockResolvedValueOnce(profile)
    await expect(getAccountProfile()).resolves.toEqual(profile)
    expect(requestMock).toHaveBeenLastCalledWith({ method: 'GET', url: '/api/admin/v1/account/profile' })

    requestMock.mockResolvedValueOnce({ ...profile, updatedAt: '2026-08-28T00:00:00Z' })
    await updateAccountProfile({ username: 'alice', phone: null, birthday: '2000-01-02', gender: 2 })
    expect(requestMock).toHaveBeenLastCalledWith({ method: 'PUT', url: '/api/admin/v1/account/profile', data: { username: 'alice', phone: null, birthday: '2000-01-02', gender: 2 } })

    requestMock.mockResolvedValueOnce({})
    await changePassword({ currentPassword: 'old-pass', newPassword: 'new-pass', confirmPassword: 'new-pass' })
    expect(requestMock).toHaveBeenLastCalledWith({ method: 'POST', url: '/api/admin/v1/account/password', data: { currentPassword: 'old-pass', newPassword: 'new-pass', confirmPassword: 'new-pass' } })
  })
})
