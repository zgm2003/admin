import { beforeEach, describe, expect, it, vi } from 'vitest'
import { request } from '@/utils/request'
import {
  exportMailRuleXlsx,
  getMailRuleImportTemplate,
  importMailRuleXlsx,
  previewMailRuleXlsx,
  type MailRuleXlsxImportInput,
} from '@/api/message/mail'
import { MailRuleAction, MailRuleScope } from '@/enums/mailRecipientRule'
import { YesNo } from '@/enums/yesNo'

vi.mock('@/utils/request', () => ({ request: vi.fn() }))
const input = { fileName: 'rules.xlsx', contentBase64: 'UEsDBAA=' }
const key = 'setting/.admin-storage/v2/p1/r1/c1/v1/2026/10/08/0123456789abcdef0123456789abcdef.xlsx'
const rowData = {
  scope: MailRuleScope.Email,
  pattern: 'a@example.com',
  action: MailRuleAction.Deny,
  name: '名称',
  remark: '',
  isEnabled: YesNo.Yes,
}
const row = {
  line: 2,
  rawValues: ['邮箱', 'a@example.com', '拒绝', '名称', '', '启用'],
  data: rowData,
  errors: [],
}
const preview = { rows: [row], errors: [] }

describe('mail XLSX protocol', () => {
  beforeEach(() => vi.mocked(request).mockReset())

  it('accepts an XLSX template object key and an explicitly unconfigured template', async () => {
    for (const objectKey of [key, '']) {
      vi.mocked(request).mockResolvedValue({ objectKey })
      await expect(getMailRuleImportTemplate()).resolves.toEqual({ objectKey })
    }
  })

  it.each([
    null,
    {},
    { objectKey: null },
    { objectKey: 1 },
    { objectKey: [key] },
    { objectKey: key.replace('.xlsx', '.csv') },
    { objectKey: key.replace('.xlsx', '.xlsm') },
    { url: 'https://example.com/t.xlsx' },
    { objectKey: 'file/template.xlsx' },
    { objectKey: key + '?x=1' },
    { objectKey: key.replace('/p1/', '/p0/') },
    { objectKey: '', extra: true },
  ])('rejects a malformed template response: %j', async (value) => {
    vi.mocked(request).mockResolvedValue(value)
    await expect(getMailRuleImportTemplate()).rejects.toThrow()
  })

  it('sends the same binary payload to preview and confirmation without changing the URLs', async () => {
    vi.mocked(request).mockResolvedValueOnce(preview).mockResolvedValueOnce({ imported: 1 })
    await expect(previewMailRuleXlsx(input)).resolves.toEqual(preview)
    await expect(importMailRuleXlsx(input)).resolves.toEqual({ imported: 1 })
    expect(request).toHaveBeenNthCalledWith(1, {
      method: 'POST',
      url: '/api/admin/v1/message/mail/recipient-rule/import/preview',
      data: input,
    })
    expect(request).toHaveBeenNthCalledWith(2, {
      method: 'POST',
      url: '/api/admin/v1/message/mail/recipient-rule/import',
      data: input,
    })
  })

  it('parses backend numeric data while preserving raw Chinese cells without interpreting them', async () => {
    vi.mocked(request).mockResolvedValue({ rows: [row], errors: [] })
    await expect(previewMailRuleXlsx(input)).resolves.toEqual({ rows: [row], errors: [] })
    const duplicate = { ...row, errors: ['duplicate_file'] }
    vi.mocked(request).mockResolvedValue({ rows: [duplicate], errors: [] })
    await expect(previewMailRuleXlsx(input)).resolves.toEqual({ rows: [duplicate], errors: [] })
  })

  it('does not infer or normalize business fields from raw cells', async () => {
    const untouched = {
      ...row,
      rawValues: [
        'uninterpreted type',
        ' RAW VALUE ',
        'uninterpreted action',
        '',
        '',
        'uninterpreted status',
      ],
      data: { ...rowData, pattern: ' BACKEND-PATTERN ', name: '', remark: ' BACKEND-REMARK ' },
    }
    vi.mocked(request).mockResolvedValue({ rows: [untouched], errors: [] })
    await expect(previewMailRuleXlsx(input)).resolves.toEqual({ rows: [untouched], errors: [] })
  })

  it('accepts a domain allow rule with disabled status and duplicate errors', async () => {
    const domain = {
      ...row,
      data: {
        ...rowData,
        scope: MailRuleScope.Domain,
        action: MailRuleAction.Allow,
        isEnabled: YesNo.No,
      },
      errors: ['duplicate_existing', 'duplicate_file'],
    }
    vi.mocked(request).mockResolvedValue({ rows: [domain], errors: [] })
    await expect(previewMailRuleXlsx(input)).resolves.toEqual({ rows: [domain], errors: [] })
  })

  it.each([
    { ...row, rawValues: ['邮箱', 'a@example.com'], data: rowData },
    {
      ...row,
      rawValues: ['邮箱', 'a@example.com', '拒绝', '名称', '', '启用'],
      data: null,
      errors: [],
    },
    { ...row, data: { ...rowData, scope: 2 } },
    { ...row, data: { ...rowData, action: 'deny' } },
    { ...row, data: { ...rowData, isEnabled: null } },
    { ...row, data: { ...rowData, extra: true } },
    { ...row, data: { ...rowData, pattern: null } },
    { ...row, data: { ...rowData, name: false } },
    { ...row, data: { ...rowData, remark: 1 } },
    { ...row, data: {} },
    { ...row, data: [] },
    { ...row, data: undefined },
    { ...row, rawValues: [...row.rawValues, 'extra'] },
    { ...row, data: null, errors: ['invalid_columns'] },
    { ...row, data: null, errors: ['unsupported_formula'] },
    { ...row, data: null, errors: ['invalid_scope', 'duplicate_file'] },
    { line: 2, values: row.rawValues, errors: [] },
    {
      ...row,
      rawValues: ['邮箱', 'a@example.com', '拒绝', '名称', '', '启用'],
      data: null,
      errors: ['duplicate_file'],
    },
    {
      ...row,
      rawValues: ['邮箱', 'a@example.com', '拒绝', '名称', '', '启用'],
      data: rowData,
      errors: ['invalid_scope'],
    },
  ])(
    'rejects a row that violates the rawValues/data/error relationship: %j',
    async (invalidRow) => {
      vi.mocked(request).mockResolvedValue({ rows: [invalidRow], errors: [] })
      await expect(previewMailRuleXlsx(input)).rejects.toThrow()
    },
  )

  it.each([
    null,
    {},
    { ...input, extra: true },
    { fileName: 'rules.xlsx' },
    { ...input, fileName: null },
    { ...input, fileName: '' },
    { ...input, fileName: 1 },
    ...[
      'rules.csv',
      'rules.xls',
      'rules.xlsm',
      '../rules.xlsx',
      'C:\\rules.xlsx',
      '.xlsx',
      ' rules.xlsx',
      'rules.xlsx ',
      'rules?.xlsx',
      'rules\u0000.xlsx',
    ].map((fileName) => ({ ...input, fileName })),
    ...[
      null,
      1,
      '',
      '@@@',
      'UEsDBAB=',
      'UEsDBAA',
      'UEsDBAA=\n',
      'Y3N2',
      'data:application/zip;base64,UEsDBAA=',
    ].map((contentBase64) => ({ ...input, contentBase64 })),
  ])('rejects malformed input before either request: %j', async (value) => {
    vi.mocked(request).mockResolvedValue(preview)
    await expect(previewMailRuleXlsx(value as MailRuleXlsxImportInput)).rejects.toThrow()
    await expect(importMailRuleXlsx(value as MailRuleXlsxImportInput)).rejects.toThrow()
    expect(request).not.toHaveBeenCalled()
  })

  it.each([
    null,
    {},
    { rows: [], errors: null },
    { rows: null, errors: [] },
    { ...preview, extra: true },
    { ...preview, errors: ['unknown'] },
    { ...preview, errors: ['empty', 'empty'] },
    { ...preview, errors: ['invalid_scope'] },
    { ...preview, errors: ['duplicate_existing'] },
    ...[
      {},
      null,
      { ...row, extra: 1 },
      { ...row, line: null },
      { ...row, line: 1 },
      { ...row, line: 2.1 },
      { ...row, line: 1002 },
      { ...row, values: null },
      { ...row, values: ['邮箱', 'a@example.com', '拒绝', '名称', null, '启用'] },
      { ...row, errors: null },
      { ...row, errors: ['invalid_xlsx', 'invalid_xlsx'] },
      { ...row, values: ['email', 'a@example.com', 'deny', '名称', '', '1'] },
      { ...row, values: ['邮箱', 'a@example.com', '拒绝', '名称', '', '1'] },
      { ...row, values: ['邮箱', '', '拒绝', '名称', '', '启用'] },
    ].map((value) => ({ rows: [value], errors: [] })),
    { rows: [row, row], errors: [] },
  ])('rejects malformed preview DTOs: %j', async (value) => {
    vi.mocked(request).mockResolvedValue(value)
    await expect(previewMailRuleXlsx(input)).rejects.toThrow()
  })

  it('preserves raw values for invalid fields and file-level validation errors', async () => {
    const invalid = {
      rows: [
        { line: 3, rawValues: ['错误', '', '', '', '', ''], data: null, errors: ['invalid_scope'] },
      ],
      errors: [],
    }
    vi.mocked(request).mockResolvedValue(invalid)
    await expect(previewMailRuleXlsx(input)).resolves.toEqual(invalid)
    for (const error of [
      'invalid_xlsx',
      'missing_sheet',
      'unsupported_formula',
      'invalid_columns',
      'too_many_rows',
      'empty',
    ]) {
      vi.mocked(request).mockResolvedValue({ rows: [], errors: [error] })
      await expect(previewMailRuleXlsx(input)).resolves.toEqual({ rows: [], errors: [error] })
    }
  })

  it.each([
    null,
    {},
    { imported: null },
    { imported: 0 },
    { imported: 1.2 },
    { imported: 1001 },
    { imported: '2' },
    { imported: 1, extra: true },
  ])('rejects an impossible import result: %j', async (value) => {
    vi.mocked(request).mockResolvedValue(value)
    await expect(importMailRuleXlsx(input)).rejects.toThrow()
  })

  it('decodes bounded XLSX export bytes for a native Blob download', async () => {
    vi.mocked(request).mockResolvedValue({
      fileName: 'mail-recipient-rule.xlsx',
      contentBase64: input.contentBase64,
    })
    const file = await exportMailRuleXlsx()
    expect(file.fileName).toBe('mail-recipient-rule.xlsx')
    expect(new Uint8Array(file.content)).toEqual(new Uint8Array([80, 75, 3, 4, 0]))
  })

  it.each([
    null,
    {},
    { fileName: null, contentBase64: input.contentBase64 },
    { fileName: 'mail-recipient-rule.csv', contentBase64: input.contentBase64 },
    { fileName: '../a.xlsx', contentBase64: input.contentBase64 },
    { fileName: 'mail-recipient-rule.xlsx', content: 'old protocol' },
    { fileName: 'mail-recipient-rule.xlsx', contentBase64: input.contentBase64, extra: true },
    ...[null, 1, '', '@@@', 'UEsDBAB=', 'UEsDBAA', 'Y3N2'].map((contentBase64) => ({
      fileName: 'mail-recipient-rule.xlsx',
      contentBase64,
    })),
  ])('rejects malformed export DTOs: %j', async (value) => {
    vi.mocked(request).mockResolvedValue(value)
    await expect(exportMailRuleXlsx()).rejects.toThrow()
  })

  it('forwards cancellation to template, preview, confirmation, and export requests', async () => {
    const signal = new AbortController().signal
    vi.mocked(request)
      .mockResolvedValueOnce({ objectKey: key })
      .mockResolvedValueOnce(preview)
      .mockResolvedValueOnce({ imported: 1 })
      .mockResolvedValueOnce({
        fileName: 'mail-recipient-rule.xlsx',
        contentBase64: input.contentBase64,
      })
    await getMailRuleImportTemplate(signal)
    await previewMailRuleXlsx(input, signal)
    await importMailRuleXlsx(input, signal)
    await exportMailRuleXlsx(signal)
    expect(vi.mocked(request).mock.calls.map(([config]) => config.signal)).toEqual([
      signal,
      signal,
      signal,
      signal,
    ])
  })

  it('rejects input and export bytes exceeding the 2 MiB boundary', async () => {
    const contentBase64 = btoa('PK\x03\x04' + '\0'.repeat(2 * 1024 * 1024 - 3))
    vi.mocked(request).mockResolvedValue(preview)
    await expect(previewMailRuleXlsx({ ...input, contentBase64 })).rejects.toThrow()
    expect(request).not.toHaveBeenCalled()
    vi.mocked(request).mockResolvedValue({ fileName: 'mail-recipient-rule.xlsx', contentBase64 })
    await expect(exportMailRuleXlsx()).rejects.toThrow()
  })
})
