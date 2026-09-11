import { beforeEach, describe, expect, it, vi } from 'vitest'
import { request } from '@/utils/request'
import {
  changePassword,
  changePasswordByCode,
  getAccountProfile,
  sendPasswordCode,
  updateAccountProfile,
} from '@/api/user/profile'

vi.mock('@/utils/request', () => ({
  request: vi.fn(),
  ProtocolError: class ProtocolError extends Error {},
}))
const requestMock = vi.mocked(request)

describe('account API', () => {
  beforeEach(() => requestMock.mockReset())

  it('uses the admin account endpoints and preserves profile fields', async () => {
    const profile = {
      userId: 7,
      username: 'alice',
      email: 'alice@example.com',
      phone: null,
      avatar: 'avatar/a.png',
      birthday: '2000-01-02',
      gender: 2,
    }
    requestMock.mockResolvedValueOnce(profile)
    await expect(getAccountProfile()).resolves.toEqual(profile)
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'GET',
      url: '/api/admin/v1/user/profile',
    })

    requestMock.mockResolvedValueOnce({ ...profile, updatedAt: '2026-08-28T00:00:00Z' })
    await updateAccountProfile({
      username: 'alice',
      avatar: 'avatar/a.png',
      birthday: '2000-01-02',
      gender: 2,
    })
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'PUT',
      url: '/api/admin/v1/user/profile',
      data: {
        username: 'alice',
        avatar: 'avatar/a.png',
        birthday: '2000-01-02',
        gender: 2,
      },
    })

    requestMock.mockResolvedValueOnce({})
    await changePassword({
      currentPassword: 'old-pass',
      newPassword: 'new-pass',
      confirmPassword: 'new-pass',
    })
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'POST',
      url: '/api/admin/v1/user/password',
      data: { currentPassword: 'old-pass', newPassword: 'new-pass', confirmPassword: 'new-pass' },
    })
  })

  it('uses symmetric email and phone password-code endpoints', async () => {
    requestMock.mockResolvedValueOnce({
      challengeId: 'password-challenge',
      expiresAt: '2026-09-11T08:00:00Z',
      resendAfterSeconds: 60,
    })
    await expect(sendPasswordCode('email')).resolves.toEqual({
      challengeId: 'password-challenge',
      expiresAt: '2026-09-11T08:00:00Z',
      resendAfterSeconds: 60,
    })
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'POST',
      url: '/api/admin/v1/user/password/send-code',
      data: { loginType: 'email' },
    })

    const input = {
      loginType: 'phone' as const,
      challengeId: 'password-challenge',
      code: '123456',
      newPassword: 'NewPassw0rd!',
      confirmPassword: 'NewPassw0rd!',
    }
    requestMock.mockResolvedValueOnce({})
    await expect(changePasswordByCode(input)).resolves.toBeUndefined()
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'PUT',
      url: '/api/admin/v1/user/password/by-code',
      data: input,
    })
  })

  it('rejects malformed password-code responses', async () => {
    requestMock.mockResolvedValueOnce({
      challengeId: '',
      expiresAt: 'invalid',
      resendAfterSeconds: -1,
    })
    await expect(sendPasswordCode('phone')).rejects.toThrow()

    requestMock.mockResolvedValueOnce({ ignored: true })
    await expect(
      changePasswordByCode({
        loginType: 'email',
        challengeId: 'challenge',
        code: '123456',
        newPassword: 'NewPassw0rd!',
        confirmPassword: 'NewPassw0rd!',
      }),
    ).rejects.toThrow('password code result')
  })
})
