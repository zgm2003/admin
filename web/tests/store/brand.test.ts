import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getBrandSettings } from '@/api/system/setting'
import type { BrandSettings } from '@/api/system/setting'
import { useBrandStore } from '@/store/brand'

vi.mock('@/api/system/setting', () => ({ getBrandSettings: vi.fn() }))

const getBrandSettingsMock = vi.mocked(getBrandSettings)

describe('brand store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getBrandSettingsMock.mockReset()
  })

  it('shares one in-flight request and exposes the loaded brand', async () => {
    const request = deferred<BrandSettings>()
    getBrandSettingsMock.mockReturnValue(request.promise)
    const store = useBrandStore()

    const first = store.load()
    const second = store.load()

    expect(getBrandSettingsMock).toHaveBeenCalledOnce()
    request.resolve({ titleZhCN: '智澜', titleEnUS: 'ZHILAN', defaultAvatar: 'avatar/default.png' })
    await Promise.all([first, second])

    expect(store.status).toBe('ready')
    expect(store.settings).toEqual({
      titleZhCN: '智澜',
      titleEnUS: 'ZHILAN',
      defaultAvatar: 'avatar/default.png',
    })
  })

  it('does not restore a response that resolves after reset', async () => {
    const request = deferred<BrandSettings>()
    getBrandSettingsMock.mockReturnValue(request.promise)
    const store = useBrandStore()

    const pending = store.load()
    store.reset()
    request.resolve({ titleZhCN: '旧标题', titleEnUS: 'OLD', defaultAvatar: 'avatar/old.png' })
    await pending

    expect(store.status).toBe('idle')
    expect(store.settings).toEqual({ titleZhCN: '', titleEnUS: '', defaultAvatar: '' })
  })
})

function deferred<T>(): { promise: Promise<T>; resolve: (value: T) => void } {
  let resolvePromise: ((value: T) => void) | undefined
  const promise = new Promise<T>((resolve) => {
    resolvePromise = resolve
  })
  return {
    promise,
    resolve: (value: T) => {
      if (resolvePromise === undefined) throw new Error('deferred promise was not initialized')
      resolvePromise(value)
    },
  }
}
