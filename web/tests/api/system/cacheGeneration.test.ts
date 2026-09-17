import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCacheGenerations, type CacheGeneration } from '@/api/system/cacheGeneration'
import { ProtocolError } from '@/types/http'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({ request: vi.fn() }))

const requestMock = vi.mocked(request)

function generationRow(overrides: Partial<Record<string, unknown>> = {}): Record<string, unknown> {
  return {
    namespace: 'system.setting',
    scopeKey: 'global',
    generation: 12,
    status: 'ready',
    pendingCount: 0,
    oldestPendingAt: null,
    latestAttempts: 1,
    lastError: '',
    latestPublishedGeneration: 12,
    latestPublishedAt: '2026-09-16T08:00:00Z',
    updatedAt: '2026-09-16T08:00:00Z',
    ...overrides,
  }
}

describe('cache generation API', () => {
  beforeEach(() => requestMock.mockReset())

  it('requests the frozen endpoint and parses the strict page DTO', async () => {
    requestMock.mockResolvedValueOnce({
      list: [
        generationRow(),
        generationRow({
          status: 'retrying',
          pendingCount: 2,
          latestAttempts: 3,
          oldestPendingAt: '2026-09-16T07:00:00Z',
        }),
      ],
      total: 2,
      page: 1,
      pageSize: 20,
    })

    const page = await getCacheGenerations({
      page: 1,
      pageSize: 20,
      keyword: 'system',
      publishState: 'retrying',
    })
    expect(page.total).toBe(2)
    expect(page.list[0]?.status).toBe('ready')
    expect(page.list[1]?.oldestPendingAt).toBe('2026-09-16T07:00:00Z')
    expect(requestMock.mock.calls[0]?.[0]).toEqual({
      method: 'GET',
      url: '/api/admin/v1/system/cachegeneration',
      params: { page: 1, pageSize: 20, keyword: 'system', publishState: 'retrying' },
    })
  })

  it('rejects unknown or missing fields, invalid enums and invalid counters', async () => {
    const invalidRows: Array<Record<string, unknown>> = [
      generationRow({ ignored: true }),
      generationRow({ status: 'stale' }),
      generationRow({ generation: 0 }),
      generationRow({ pendingCount: -1 }),
      generationRow({ latestAttempts: -2 }),
      generationRow({ latestPublishedGeneration: 0 }),
      generationRow({ latestPublishedGeneration: '12' }),
    ]
    const missingField = generationRow()
    delete missingField.lastError
    invalidRows.push(missingField)

    for (const row of invalidRows) {
      requestMock.mockResolvedValueOnce({ list: [row], total: 1, page: 1, pageSize: 20 })
      await expect(getCacheGenerations({ page: 1, pageSize: 20 })).rejects.toBeInstanceOf(
        ProtocolError,
      )
    }
  })

  it('rejects invalid and malformed timestamps but accepts nullable ones', async () => {
    for (const row of [
      generationRow({ updatedAt: '2026-09-16 08:00:00' }),
      generationRow({ updatedAt: 'not-a-time' }),
      generationRow({ latestPublishedAt: '2026-09-16T08:00:00+08:00' }),
      generationRow({ oldestPendingAt: 12 }),
    ]) {
      requestMock.mockResolvedValueOnce({ list: [row], total: 1, page: 1, pageSize: 20 })
      await expect(getCacheGenerations({ page: 1, pageSize: 20 })).rejects.toBeInstanceOf(
        ProtocolError,
      )
    }

    requestMock.mockResolvedValueOnce({
      list: [
        generationRow({
          oldestPendingAt: null,
          latestPublishedAt: null,
          latestPublishedGeneration: null,
        }),
      ],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    const page = await getCacheGenerations({ page: 1, pageSize: 20 })
    const row: CacheGeneration | undefined = page.list[0]
    expect(row?.oldestPendingAt).toBeNull()
    expect(row?.latestPublishedGeneration).toBeNull()
  })

  it('rejects a page envelope with unknown fields', async () => {
    requestMock.mockResolvedValueOnce({ list: [], total: 0, page: 1, pageSize: 20, ignored: 1 })
    await expect(getCacheGenerations({ page: 1, pageSize: 20 })).rejects.toBeInstanceOf(
      ProtocolError,
    )
  })
})
