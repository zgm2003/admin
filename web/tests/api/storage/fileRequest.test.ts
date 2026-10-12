import type { AxiosAdapter, AxiosRequestConfig, InternalAxiosRequestConfig } from 'axios'
import { afterEach, expect, it, vi } from 'vitest'

import { completeFileUpload } from '@/api/storage/file'
import { request } from '@/utils/request'

afterEach(() => vi.restoreAllMocks())

it.each([true, false])(
  'sends completion with no serialized body through the real request layer (signal=%s)',
  async (withSignal) => {
    let received: InternalAxiosRequestConfig | undefined
    const confirmed = { fileId: 8, objectKey: 'final/key' }
    const adapter: AxiosAdapter = async (config) => {
      received = config
      return {
        config,
        status: 200,
        statusText: 'OK',
        headers: {},
        data: { code: 0, data: confirmed, message: 'ok' },
      }
    }
    const originalPost = request.post
    vi.spyOn(request, 'post').mockImplementation(
      <T = unknown, D = unknown>(
        url: string,
        data?: D,
        config?: AxiosRequestConfig<D>,
      ): Promise<T> => originalPost<T, D>(url, data, { ...config, adapter }),
    )
    const signal = withSignal ? new AbortController().signal : undefined
    expect(await completeFileUpload(8, signal)).toBe(confirmed)
    expect(received?.url).toBe('/api/v1/storage/file/8/complete')
    expect(received?.method).toBe('post')
    expect(received?.data).toBeUndefined()
    expect(received?.signal).toBe(signal)
  },
)
