// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { request } from '@/utils/request'
import {
  exportMailRuleXlsx,
  getMailRuleImportTemplate,
  importMailRuleXlsx,
  previewMailRuleXlsx,
} from '@/api/message/mail'
import { decodeBase64Bytes } from '@/utils/browserFile'

vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
}))
beforeEach(() => vi.resetAllMocks())

const input = { fileName: 'rules.xlsx', contentBase64: 'UEsDBAA=' }
const preview = {
  rows: [
    {
      line: 2,
      rawValues: ['后端类型', ' RAW VALUE ', '后端动作', '', '', '后端状态'],
      data: {
        scope: 99,
        pattern: ' BACKEND-PATTERN ',
        action: 99,
        name: '',
        remark: ' BACKEND-REMARK ',
        isEnabled: 1,
      },
      errors: ['server_added_code'],
      serverAdded: true,
    },
  ],
  errors: [],
}

describe('mail XLSX thin API', () => {
  it('preserves numeric values, untranslated cells and backend-added errors without interpreting them', async () => {
    vi.mocked(request.post).mockResolvedValue(preview)
    await expect(previewMailRuleXlsx(input)).resolves.toBe(preview)
    expect(request.post).toHaveBeenCalledExactlyOnceWith(
      '/api/admin/v1/message/mail/recipient-rule/import/preview',
      input,
      {},
    )
  })
  it('preserves empty previews and failed-row data from the backend', async () => {
    const failed = {
      rows: [{ line: 2, rawValues: [], data: null, errors: ['invalid_scope'] }],
      errors: ['empty'],
    }
    for (const result of [{ rows: [], errors: [] }, failed]) {
      vi.mocked(request.post).mockResolvedValue(result)
      await expect(previewMailRuleXlsx(input)).resolves.toBe(result)
    }
  })
  it('preserves Base64 DTO and leaves byte conversion to the browser utility', async () => {
    const result = {
      fileName: 'server-defined.xlsx',
      contentBase64: input.contentBase64,
      serverAdded: true,
    }
    vi.mocked(request.get).mockResolvedValue(result)
    const file = await exportMailRuleXlsx()
    expect(file).toBe(result)
    expect(Array.from(new Uint8Array(decodeBase64Bytes(file.contentBase64)))).toEqual([
      80, 75, 3, 4, 0,
    ])
  })
  it('forwards cancellation signals to all four file requests', async () => {
    const signal = new AbortController().signal
    vi.mocked(request.get).mockResolvedValue({})
    vi.mocked(request.post).mockResolvedValue({})
    await getMailRuleImportTemplate(signal)
    await previewMailRuleXlsx(input, signal)
    await importMailRuleXlsx(input, signal)
    await exportMailRuleXlsx(signal)
    expect(vi.mocked(request.get).mock.calls.map(([, config]) => config?.signal)).toEqual([
      signal,
      signal,
    ])
    expect(vi.mocked(request.post).mock.calls.map(([, , config]) => config?.signal)).toEqual([
      signal,
      signal,
    ])
  })
  it('does not rewrite, validate or suppress an input that must be checked by Go', async () => {
    const invalidInput = { fileName: '', contentBase64: 'not-base64' }
    const error = new Error('Go rejected the input')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(importMailRuleXlsx(invalidInput)).rejects.toBe(error)
    expect(request.post).toHaveBeenCalledExactlyOnceWith(
      '/api/admin/v1/message/mail/recipient-rule/import',
      invalidInput,
      {},
    )
  })
})
