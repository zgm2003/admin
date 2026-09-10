import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getDictionaryOptions } from '@/api/system/dictionary'
import { setLocale } from '@/i18n'
import { useSystemDictionaryStore } from '@/store/systemDictionary'

vi.mock('@/api/system/dictionary', () => ({ getDictionaryOptions: vi.fn() }))

const getDictionaryOptionsMock = vi.mocked(getDictionaryOptions)

describe('system dictionary store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    setLocale('zh-CN')
    getDictionaryOptionsMock.mockReset()
  })

  it('does not present an unloaded or failed dictionary as an empty option list', async () => {
    const failure = new Error('dictionary unavailable')
    getDictionaryOptionsMock.mockRejectedValueOnce(failure)
    const store = useSystemDictionaryStore()

    expect(store.state('user.gender').value.status).toBe('idle')
    expect(store.options('user.gender').value).toBeUndefined()
    await expect(store.load(['user.gender'])).rejects.toBe(failure)
    expect(store.state('user.gender').value.status).toBe('error')
    expect(store.options('user.gender').value).toBeUndefined()
  })

  it('loads different codes requested concurrently without dropping either request', async () => {
    const gender = deferred<Record<string, Array<{ label: string; value: string }>>>()
    const status = deferred<Record<string, Array<{ label: string; value: string }>>>()
    getDictionaryOptionsMock.mockReturnValueOnce(gender.promise).mockReturnValueOnce(status.promise)
    const store = useSystemDictionaryStore()

    const first = store.load(['user.gender'])
    const second = store.load(['user.status'])
    expect(getDictionaryOptionsMock).toHaveBeenNthCalledWith(1, ['user.gender'])
    expect(getDictionaryOptionsMock).toHaveBeenNthCalledWith(2, ['user.status'])

    gender.resolve({ 'user.gender': [{ label: '女', value: 'female' }] })
    status.resolve({ 'user.status': [{ label: '启用', value: 'enabled' }] })
    await Promise.all([first, second])

    expect(store.options('user.gender').value).toEqual([{ label: '女', value: 'female' }])
    expect(store.options('user.status').value).toEqual([{ label: '启用', value: 'enabled' }])
  })

  it('isolates option labels by active language', async () => {
    getDictionaryOptionsMock
      .mockResolvedValueOnce({ 'user.gender': [{ label: '女', value: 'female' }] })
      .mockResolvedValueOnce({ 'user.gender': [{ label: 'Female', value: 'female' }] })
    const store = useSystemDictionaryStore()

    await store.load(['user.gender'])
    setLocale('en-US')
    expect(store.options('user.gender').value).toBeUndefined()
    await store.load(['user.gender'])

    expect(getDictionaryOptionsMock).toHaveBeenCalledTimes(2)
    expect(store.options('user.gender').value).toEqual([{ label: 'Female', value: 'female' }])
    setLocale('zh-CN')
    expect(store.options('user.gender').value).toEqual([{ label: '女', value: 'female' }])
  })

  it('reset prevents a late request from restoring stale options', async () => {
    const response = deferred<Record<string, Array<{ label: string; value: string }>>>()
    getDictionaryOptionsMock.mockReturnValueOnce(response.promise)
    const store = useSystemDictionaryStore()

    const pending = store.load(['user.gender'])
    store.reset()
    response.resolve({ 'user.gender': [{ label: '女', value: 'female' }] })
    await pending

    expect(store.state('user.gender').value.status).toBe('idle')
    expect(store.options('user.gender').value).toBeUndefined()
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
