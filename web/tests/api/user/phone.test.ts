import { beforeEach, describe, expect, it, vi } from 'vitest'

import { bindPhone, getPhoneChangeLogs, sendPhoneCode } from '@/api/user/phone'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({ request: vi.fn() }))
const requestMock = vi.mocked(request)

describe('user phone identity API', () => {
  beforeEach(() => requestMock.mockReset())

  it('sends exact current, next, and bind request shapes', async () => {
    requestMock.mockResolvedValueOnce({
      challengeId: 'current-challenge',
      expiresAt: '2026-09-11T08:00:00Z',
      resendAfterSeconds: 60,
    })
    await sendPhoneCode({ target: 'current' })
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'POST',
      url: '/api/admin/v1/user/phone/send-code',
      data: { target: 'current' },
    })

    requestMock.mockResolvedValueOnce({
      challengeId: 'next-challenge',
      expiresAt: '2026-09-11T08:00:00Z',
      resendAfterSeconds: 0,
    })
    await sendPhoneCode({ target: 'next', phone: '+8615671628271' })
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'POST',
      url: '/api/admin/v1/user/phone/send-code',
      data: { target: 'next', phone: '+8615671628271' },
    })

    const input = {
      currentChallengeId: 'current-challenge',
      currentCode: '111111',
      nextPhone: '+8615671628271',
      nextChallengeId: 'next-challenge',
      nextCode: '222222',
    }
    requestMock.mockResolvedValueOnce({ phone: '+8615671628271' })
    await expect(bindPhone(input)).resolves.toEqual({ phone: '+8615671628271' })
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'PUT',
      url: '/api/admin/v1/user/phone',
      data: input,
    })
  })

  it.each([
    { challengeId: '', expiresAt: '2026-09-11T08:00:00Z', resendAfterSeconds: 60 },
    { challengeId: 'challenge', expiresAt: 'invalid', resendAfterSeconds: 60 },
    { challengeId: 'challenge', expiresAt: '2026-09-11T08:00:00Z', resendAfterSeconds: -1 },
    {
      challengeId: 'challenge',
      expiresAt: '2026-09-11T08:00:00Z',
      resendAfterSeconds: 60,
      extra: true,
    },
  ])('rejects malformed send-code responses', async (response) => {
    requestMock.mockResolvedValue(response)
    await expect(sendPhoneCode({ target: 'current' })).rejects.toThrow()
  })

  it.each([
    { phone: '15671628271' },
    { phone: '+8612671628271' },
    { phone: '' },
    { phone: '+8615671628271', extra: true },
  ])('rejects non-canonical phone identity responses', async (response) => {
    requestMock.mockResolvedValue(response)
    await expect(
      bindPhone({ nextPhone: '+8615671628271', nextChallengeId: 'challenge', nextCode: '123456' }),
    ).rejects.toThrow()
  })

  it('loads numeric phone change actions and plaintext history', async () => {
    requestMock.mockResolvedValue({
      list: [
        {
          id: 1,
          action: 1,
          oldPhone: '+8615671628271',
          newPhone: '+8613800000000',
          platform: 'admin',
          createdAt: '2026-09-29T05:00:00Z',
        },
        {
          id: 2,
          action: 2,
          oldPhone: null,
          newPhone: '+8613900000000',
          platform: 'admin',
          createdAt: '2026-09-28T05:00:00Z',
        },
      ],
      total: 2,
      page: 1,
      pageSize: 20,
    })
    await expect(getPhoneChangeLogs(7, { page: 1, pageSize: 20 })).resolves.toEqual({
      list: expect.arrayContaining([
        expect.objectContaining({ action: 1, oldPhone: '+8615671628271' }),
        expect.objectContaining({ action: 2, oldPhone: null }),
      ]),
      total: 2,
      page: 1,
      pageSize: 20,
    })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'GET',
      url: '/api/admin/v1/user/account/7/phone-change-log',
      params: { page: 1, pageSize: 20 },
    })
  })
})
