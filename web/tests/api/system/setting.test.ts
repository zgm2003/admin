import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  getBrandSettings,
  getLegalDocument,
  getPublicLegalDocument,
  updateBrandSettings,
  updateLegalDocument,
} from '@/api/system/setting'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({ request: vi.fn() }))

describe('system setting API', () => {
  beforeEach(() => vi.clearAllMocks())

  it('strictly parses and updates the brand settings contract', async () => {
    vi.mocked(request)
      .mockResolvedValueOnce({
        titleZhCN: '智澜',
        titleEnUS: 'ZHILAN',
        defaultAvatar: 'avatar/2026/default.png',
      })
      .mockResolvedValueOnce({})

    await expect(getBrandSettings()).resolves.toEqual({
      titleZhCN: '智澜',
      titleEnUS: 'ZHILAN',
      defaultAvatar: 'avatar/2026/default.png',
    })
    await expect(
      updateBrandSettings({ titleZhCN: '新标题', titleEnUS: 'New title', defaultAvatar: '' }),
    ).resolves.toBeUndefined()
    expect(request).toHaveBeenNthCalledWith(1, {
      method: 'GET',
      url: '/api/admin/v1/system/setting/brand',
    })
    expect(request).toHaveBeenNthCalledWith(2, {
      method: 'PUT',
      url: '/api/admin/v1/system/setting/brand',
      data: { titleZhCN: '新标题', titleEnUS: 'New title', defaultAvatar: '' },
    })
  })

  it('rejects malformed brand settings responses', async () => {
    vi.mocked(request).mockResolvedValue({ titleZhCN: '智澜', titleEnUS: 'ZHILAN' })
    await expect(getBrandSettings()).rejects.toThrow()
  })

  it('strictly parses public and management legal document contracts', async () => {
    vi.mocked(request)
      .mockResolvedValueOnce({ kind: 'privacyPolicy', contentHtml: '<p>Privacy</p>' })
      .mockResolvedValueOnce({ kind: 'userAgreement', contentHtml: '<p>Terms</p>' })
      .mockResolvedValueOnce({})

    await expect(getPublicLegalDocument('privacyPolicy')).resolves.toEqual({
      kind: 'privacyPolicy',
      contentHtml: '<p>Privacy</p>',
    })
    await expect(getLegalDocument('userAgreement')).resolves.toEqual({
      kind: 'userAgreement',
      contentHtml: '<p>Terms</p>',
    })
    await expect(updateLegalDocument('userAgreement', '<p>Updated</p>')).resolves.toBeUndefined()

    expect(request).toHaveBeenNthCalledWith(1, {
      method: 'GET',
      url: '/api/v1/system/setting/legal/privacyPolicy',
    })
    expect(request).toHaveBeenNthCalledWith(2, {
      method: 'GET',
      url: '/api/admin/v1/system/setting/legal/userAgreement',
    })
    expect(request).toHaveBeenNthCalledWith(3, {
      method: 'PUT',
      url: '/api/admin/v1/system/setting/legal/userAgreement',
      data: { contentHtml: '<p>Updated</p>' },
    })
  })

  it('rejects malformed and mismatched legal document responses', async () => {
    vi.mocked(request)
      .mockResolvedValueOnce({
        kind: 'privacyPolicy',
        contentHtml: '<p>Privacy</p>',
        locale: 'zh-CN',
      })
      .mockResolvedValueOnce({ kind: 'privacyPolicy', contentHtml: '<p>Privacy</p>' })
    await expect(getPublicLegalDocument('privacyPolicy')).rejects.toThrow()
    await expect(getLegalDocument('userAgreement')).rejects.toThrow()
  })
})
