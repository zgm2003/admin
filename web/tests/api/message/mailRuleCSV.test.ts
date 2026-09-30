import { beforeEach, describe, expect, it, vi } from 'vitest'
import { request } from '@/utils/request'
import {
  exportMailRules,
  getMailRuleImportTemplate,
  importMailRules,
  previewMailRuleImport,
} from '@/api/message/mail'

vi.mock('@/utils/request', () => ({ request: vi.fn() }))

const content = '\ufeff类型,邮箱/域名,动作,名称,备注,启用状态\n'
const validPreview = {
  rows: [{ line: 2, values: ['email', 'a@example.com', 'deny', '名称', '', '1'], errors: [] }],
  errors: [],
}

describe('mail rule CSV protocol', () => {
  beforeEach(() => vi.mocked(request).mockReset())

  it('reads a template object key and rejects legacy URLs without system setting access', async () => {
    const objectKey =
      'file/.admin-storage/v2/p1/r1/c1/v1/2026/09/30/0123456789abcdef0123456789abcdef.csv'
    vi.mocked(request).mockResolvedValue({ objectKey })
    await expect(getMailRuleImportTemplate()).resolves.toEqual({
      objectKey,
    })
    expect(request).toHaveBeenCalledWith({
      method: 'GET',
      url: '/api/admin/v1/message/mail/recipient-rule/import-template',
    })
    for (const value of [
      { url: 'https://example.com/t.csv' },
      { objectKey: 'https://example.com/t.csv' },
      { objectKey: 'file/template.csv' },
      { objectKey: objectKey.replace('/p1/', '/p0/') },
      { objectKey: objectKey.replace('/09/30/', '/02/30/') },
      { objectKey: objectKey.replace('.csv', '.png') },
      { objectKey: '', extra: true },
    ]) {
      vi.mocked(request).mockResolvedValue(value)
      await expect(getMailRuleImportTemplate()).rejects.toThrow()
    }
    vi.mocked(request).mockResolvedValue({ objectKey: '' })
    await expect(getMailRuleImportTemplate()).resolves.toEqual({ objectKey: '' })
  })

  it('strictly parses a preview and retains server row validation issues', async () => {
    vi.mocked(request).mockResolvedValue(validPreview)
    await expect(previewMailRuleImport(content)).resolves.toEqual(validPreview)
    expect(request).toHaveBeenCalledWith({
      method: 'POST',
      url: '/api/admin/v1/message/mail/recipient-rule/import/preview',
      data: { content },
    })
    const malformed = {
      rows: [{ line: 2, values: ['email', 'a@example.com'], errors: ['invalid_columns'] }],
      errors: [],
    }
    vi.mocked(request).mockResolvedValue(malformed)
    await expect(previewMailRuleImport(content)).resolves.toEqual(malformed)
    for (const value of [
      { ...validPreview, extra: true },
      { ...validPreview, rows: null },
      { ...validPreview, errors: ['unknown'] },
      { rows: [{ ...validPreview.rows[0], line: 1 }], errors: [] },
      {
        rows: [
          { ...validPreview.rows[0], values: ['wrong', 'a@example.com', 'deny', 'name', '', '1'] },
        ],
        errors: [],
      },
      {
        rows: [
          {
            ...validPreview.rows[0],
            values: ['email', 'a@example.com', 'deny', 'name', '', 'true'],
          },
        ],
        errors: [],
      },
    ]) {
      vi.mocked(request).mockResolvedValue(value)
      await expect(previewMailRuleImport(content)).rejects.toThrow()
    }
  })

  it('confirms the original CSV and rejects zero, fractional or excess import counts', async () => {
    vi.mocked(request).mockResolvedValue({ imported: 2 })
    await expect(importMailRules(content)).resolves.toEqual({ imported: 2 })
    expect(request).toHaveBeenCalledWith({
      method: 'POST',
      url: '/api/admin/v1/message/mail/recipient-rule/import',
      data: { content },
    })
    for (const value of [
      { imported: 0 },
      { imported: 1.1 },
      { imported: 1001 },
      { imported: '2' },
      { imported: 2, extra: true },
    ]) {
      vi.mocked(request).mockResolvedValue(value)
      await expect(importMailRules(content)).rejects.toThrow()
    }
  })

  it('exports a safe filename and bounded UTF-8 BOM CSV through the normal envelope', async () => {
    vi.mocked(request).mockResolvedValue({ fileName: 'mail-recipient-rule.csv', content })
    await expect(exportMailRules()).resolves.toEqual({
      fileName: 'mail-recipient-rule.csv',
      content,
    })
    expect(request).toHaveBeenCalledWith({
      method: 'GET',
      url: '/api/admin/v1/message/mail/recipient-rule/export',
    })
    for (const value of [
      { fileName: '../file.csv', content },
      { fileName: 'mail-recipient-rule.csv', content: '' },
      { fileName: 'mail-recipient-rule.csv', content, secret: true },
    ]) {
      vi.mocked(request).mockResolvedValue(value)
      await expect(exportMailRules()).rejects.toThrow()
    }
  })
})
