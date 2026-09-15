import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getBrandSettings, updateBrandSettings } from '@/api/system/setting'
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
})
