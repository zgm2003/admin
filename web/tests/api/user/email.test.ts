import { beforeEach, describe, expect, it, vi } from 'vitest'

import { bindEmail, sendEmailCode } from '@/api/user/email'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({ request: vi.fn() }))
const requestMock = vi.mocked(request)

describe('user email identity API', () => {
  beforeEach(() => requestMock.mockReset())

  it('sends exact current, next, and bind request shapes', async () => {
    requestMock.mockResolvedValueOnce({
      challengeId: 'current-challenge',
      expiresAt: '2026-09-11T08:00:00Z',
      resendAfterSeconds: 60,
    })
    await sendEmailCode({ target: 'current' })
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'POST',
      url: '/api/admin/v1/user/email/send-code',
      data: { target: 'current' },
    })

    requestMock.mockResolvedValueOnce({
      challengeId: 'next-challenge',
      expiresAt: '2026-09-11T08:00:00Z',
      resendAfterSeconds: 0,
    })
    await sendEmailCode({ target: 'next', email: 'next@example.com' })
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'POST',
      url: '/api/admin/v1/user/email/send-code',
      data: { target: 'next', email: 'next@example.com' },
    })

    const input = {
      currentChallengeId: 'current-challenge',
      currentCode: '111111',
      nextEmail: 'next@example.com',
      nextChallengeId: 'next-challenge',
      nextCode: '222222',
    }
    requestMock.mockResolvedValueOnce({ email: 'next@example.com' })
    await expect(bindEmail(input)).resolves.toEqual({ email: 'next@example.com' })
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'PUT',
      url: '/api/admin/v1/user/email',
      data: input,
    })
  })

  it.each([
    { challengeId: '', expiresAt: '2026-09-11T08:00:00Z', resendAfterSeconds: 60 },
    { challengeId: 'challenge', expiresAt: 'invalid', resendAfterSeconds: 60 },
    { challengeId: 'challenge', expiresAt: '2026-09-11T08:00:00Z', resendAfterSeconds: -1 },
    { challengeId: 'challenge', expiresAt: '2026-09-11T08:00:00Z', resendAfterSeconds: 86401 },
    {
      challengeId: 'challenge',
      expiresAt: '2026-09-11T08:00:00Z',
      resendAfterSeconds: 60,
      extra: true,
    },
  ])('rejects malformed send-code responses', async (response) => {
    requestMock.mockResolvedValue(response)
    await expect(sendEmailCode({ target: 'current' })).rejects.toThrow()
  })

  it.each([{ email: 'missing-at' }, { email: 'UPPER@example.com' }, { email: '' }, { email: 'a@b.com', extra: true }])(
    'rejects malformed email identity responses',
    async (response) => {
      requestMock.mockResolvedValue(response)
      await expect(
        bindEmail({ nextEmail: 'a@b.com', nextChallengeId: 'challenge', nextCode: '123456' }),
      ).rejects.toThrow()
    },
  )
})
