// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/message/mail'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('getMailConfig preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getMailConfig()
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/message/mail/config')
  })
  it('getMailConfig propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getMailConfig()).rejects.toBe(error)
  })
  it('saveMailConfig preserves the HTTP contract and backend data', async () => {
    const data: Parameters<typeof api.saveMailConfig>[0] = {
      secretId: 'sample',
      secretKey: 'sample',
      region: 'sample',
      endpoint: 'sample',
      fromEmail: 'sample',
      fromName: 'sample',
      replyTo: 'sample',
      ttlMinutes: 1,
      isEnabled: 0,
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.saveMailConfig(data)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/message/mail/config', data)
  })
  it('saveMailConfig propagates request failures unchanged', async () => {
    const data: Parameters<typeof api.saveMailConfig>[0] = {
      secretId: 'sample',
      secretKey: 'sample',
      region: 'sample',
      endpoint: 'sample',
      fromEmail: 'sample',
      fromName: 'sample',
      replyTo: 'sample',
      ttlMinutes: 1,
      isEnabled: 0,
    }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.saveMailConfig(data)).rejects.toBe(error)
  })
  it('deleteMailConfig preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.delete).mockResolvedValue(dataFromServer)
    const result = await api.deleteMailConfig()
    expect(result).toBe(dataFromServer)
    expect(request.delete).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/message/mail/config')
  })
  it('deleteMailConfig propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.delete).mockRejectedValue(error)
    await expect(api.deleteMailConfig()).rejects.toBe(error)
  })
  it('sendMailTest preserves the HTTP contract and backend data', async () => {
    const data: Parameters<typeof api.sendMailTest>[0] = {
      toEmail: 'sample',
      scene: 'sample',
      variables: {},
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.sendMailTest(data)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/message/mail/test', data)
  })
  it('sendMailTest propagates request failures unchanged', async () => {
    const data: Parameters<typeof api.sendMailTest>[0] = {
      toEmail: 'sample',
      scene: 'sample',
      variables: {},
    }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.sendMailTest(data)).rejects.toBe(error)
  })
  it('listMailTemplates preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.listMailTemplates()
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/message/mail/template')
  })
  it('listMailTemplates propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.listMailTemplates()).rejects.toBe(error)
  })
  it('updateMailTemplate preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateMailTemplate>[0] = 1
    const data: Parameters<typeof api.updateMailTemplate>[1] = {
      scene: 'sample',
      name: 'sample',
      subject: 'sample',
      tencentTemplateId: null,
      content: 'sample',
      variableKeys: ['sample'],
      exampleVariables: {},
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateMailTemplate(id, data)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/message/mail/template/${id}`,
      data,
    )
  })
  it('updateMailTemplate propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateMailTemplate>[0] = 1
    const data: Parameters<typeof api.updateMailTemplate>[1] = {
      scene: 'sample',
      name: 'sample',
      subject: 'sample',
      tencentTemplateId: null,
      content: 'sample',
      variableKeys: ['sample'],
      exampleVariables: {},
    }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateMailTemplate(id, data)).rejects.toBe(error)
  })
  it('updateMailTemplateStatus preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateMailTemplateStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateMailTemplateStatus>[1] = 0
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.patch).mockResolvedValue(dataFromServer)
    const result = await api.updateMailTemplateStatus(id, isEnabled)
    expect(result).toBe(dataFromServer)
    expect(request.patch).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/message/mail/template/${id}/status`,
      { isEnabled },
    )
  })
  it('updateMailTemplateStatus propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateMailTemplateStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateMailTemplateStatus>[1] = 0
    const error = new Error('request failed')
    vi.mocked(request.patch).mockRejectedValue(error)
    await expect(api.updateMailTemplateStatus(id, isEnabled)).rejects.toBe(error)
  })
  it('listMailLogs preserves the HTTP contract and backend data', async () => {
    const params: Parameters<typeof api.listMailLogs>[0] = { page: 1, pageSize: 1 }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.listMailLogs(params)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/message/mail/log', {
      params,
    })
  })
  it('listMailLogs propagates request failures unchanged', async () => {
    const params: Parameters<typeof api.listMailLogs>[0] = { page: 1, pageSize: 1 }
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.listMailLogs(params)).rejects.toBe(error)
  })
  it('getMailLogDetail preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.getMailLogDetail>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getMailLogDetail(id)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(`/api/admin/v1/message/mail/log/${id}`)
  })
  it('getMailLogDetail propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.getMailLogDetail>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getMailLogDetail(id)).rejects.toBe(error)
  })
  it('listMailRules preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.listMailRules()
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/message/mail/recipient-rule')
  })
  it('listMailRules propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.listMailRules()).rejects.toBe(error)
  })
  it('createMailRule preserves the HTTP contract and backend data', async () => {
    const data: Parameters<typeof api.createMailRule>[0] = {
      scope: 0,
      pattern: 'sample',
      action: 0,
      name: 'sample',
      remark: 'sample',
      isEnabled: 0,
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.createMailRule(data)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith(
      '/api/admin/v1/message/mail/recipient-rule',
      data,
    )
  })
  it('createMailRule propagates request failures unchanged', async () => {
    const data: Parameters<typeof api.createMailRule>[0] = {
      scope: 0,
      pattern: 'sample',
      action: 0,
      name: 'sample',
      remark: 'sample',
      isEnabled: 0,
    }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.createMailRule(data)).rejects.toBe(error)
  })
  it('updateMailRule preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateMailRule>[0] = 1
    const data: Parameters<typeof api.updateMailRule>[1] = {
      scope: 0,
      pattern: 'sample',
      action: 0,
      name: 'sample',
      remark: 'sample',
      isEnabled: 0,
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateMailRule(id, data)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/message/mail/recipient-rule/${id}`,
      data,
    )
  })
  it('updateMailRule propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateMailRule>[0] = 1
    const data: Parameters<typeof api.updateMailRule>[1] = {
      scope: 0,
      pattern: 'sample',
      action: 0,
      name: 'sample',
      remark: 'sample',
      isEnabled: 0,
    }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateMailRule(id, data)).rejects.toBe(error)
  })
  it('updateMailRuleStatus preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateMailRuleStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateMailRuleStatus>[1] = 0
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.patch).mockResolvedValue(dataFromServer)
    const result = await api.updateMailRuleStatus(id, isEnabled)
    expect(result).toBe(dataFromServer)
    expect(request.patch).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/message/mail/recipient-rule/${id}/status`,
      { isEnabled },
    )
  })
  it('updateMailRuleStatus propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateMailRuleStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateMailRuleStatus>[1] = 0
    const error = new Error('request failed')
    vi.mocked(request.patch).mockRejectedValue(error)
    await expect(api.updateMailRuleStatus(id, isEnabled)).rejects.toBe(error)
  })
  it('deleteMailRule preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.deleteMailRule>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.delete).mockResolvedValue(dataFromServer)
    const result = await api.deleteMailRule(id)
    expect(result).toBe(dataFromServer)
    expect(request.delete).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/message/mail/recipient-rule/${id}`,
    )
  })
  it('deleteMailRule propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.deleteMailRule>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.delete).mockRejectedValue(error)
    await expect(api.deleteMailRule(id)).rejects.toBe(error)
  })
  it('getMailRuleImportTemplate preserves the HTTP contract and backend data', async () => {
    const controller = new AbortController()
    const signal: Parameters<typeof api.getMailRuleImportTemplate>[0] = controller.signal
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getMailRuleImportTemplate(signal)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(
      '/api/admin/v1/message/mail/recipient-rule/import-template',
      { ...(signal ? { signal } : {}) },
    )
  })
  it('getMailRuleImportTemplate propagates request failures unchanged', async () => {
    const controller = new AbortController()
    const signal: Parameters<typeof api.getMailRuleImportTemplate>[0] = controller.signal
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getMailRuleImportTemplate(signal)).rejects.toBe(error)
  })
  it('previewMailRuleXlsx preserves the HTTP contract and backend data', async () => {
    const controller = new AbortController()
    const input: Parameters<typeof api.previewMailRuleXlsx>[0] = {
      fileName: 'sample',
      contentBase64: 'sample',
    }
    const signal: Parameters<typeof api.previewMailRuleXlsx>[1] = controller.signal
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.previewMailRuleXlsx(input, signal)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith(
      '/api/admin/v1/message/mail/recipient-rule/import/preview',
      input,
      { ...(signal ? { signal } : {}) },
    )
  })
  it('previewMailRuleXlsx propagates request failures unchanged', async () => {
    const controller = new AbortController()
    const input: Parameters<typeof api.previewMailRuleXlsx>[0] = {
      fileName: 'sample',
      contentBase64: 'sample',
    }
    const signal: Parameters<typeof api.previewMailRuleXlsx>[1] = controller.signal
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.previewMailRuleXlsx(input, signal)).rejects.toBe(error)
  })
  it('importMailRuleXlsx preserves the HTTP contract and backend data', async () => {
    const controller = new AbortController()
    const input: Parameters<typeof api.importMailRuleXlsx>[0] = {
      fileName: 'sample',
      contentBase64: 'sample',
    }
    const signal: Parameters<typeof api.importMailRuleXlsx>[1] = controller.signal
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.importMailRuleXlsx(input, signal)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith(
      '/api/admin/v1/message/mail/recipient-rule/import',
      input,
      { ...(signal ? { signal } : {}) },
    )
  })
  it('importMailRuleXlsx propagates request failures unchanged', async () => {
    const controller = new AbortController()
    const input: Parameters<typeof api.importMailRuleXlsx>[0] = {
      fileName: 'sample',
      contentBase64: 'sample',
    }
    const signal: Parameters<typeof api.importMailRuleXlsx>[1] = controller.signal
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.importMailRuleXlsx(input, signal)).rejects.toBe(error)
  })
  it('exportMailRuleXlsx preserves the HTTP contract and backend data', async () => {
    const controller = new AbortController()
    const signal: Parameters<typeof api.exportMailRuleXlsx>[0] = controller.signal
    const dataFromServer = {
      fileName: 'server-file.xlsx',
      contentBase64: 'UEsDBAA=',
      serverAdded: true,
    }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.exportMailRuleXlsx(signal)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(
      '/api/admin/v1/message/mail/recipient-rule/export',
      { ...(signal ? { signal } : {}) },
    )
  })
  it('exportMailRuleXlsx propagates request failures unchanged', async () => {
    const controller = new AbortController()
    const signal: Parameters<typeof api.exportMailRuleXlsx>[0] = controller.signal
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.exportMailRuleXlsx(signal)).rejects.toBe(error)
  })
  it('listMailRateLimitPolicies preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.listMailRateLimitPolicies()
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(
      '/api/admin/v1/message/mail/rate-limit-policy',
    )
  })
  it('listMailRateLimitPolicies propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.listMailRateLimitPolicies()).rejects.toBe(error)
  })
  it('updateMailRateLimitPolicy preserves the HTTP contract and backend data', async () => {
    const platformId: Parameters<typeof api.updateMailRateLimitPolicy>[0] = 1
    const key: Parameters<typeof api.updateMailRateLimitPolicy>[1] = 'sample'
    const data: Parameters<typeof api.updateMailRateLimitPolicy>[2] = { limit: 1, windowSeconds: 1 }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateMailRateLimitPolicy(platformId, key, data)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/message/mail/rate-limit-policy/${platformId}/${encodeURIComponent(key)}`,
      data,
    )
  })
  it('updateMailRateLimitPolicy propagates request failures unchanged', async () => {
    const platformId: Parameters<typeof api.updateMailRateLimitPolicy>[0] = 1
    const key: Parameters<typeof api.updateMailRateLimitPolicy>[1] = 'sample'
    const data: Parameters<typeof api.updateMailRateLimitPolicy>[2] = { limit: 1, windowSeconds: 1 }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateMailRateLimitPolicy(platformId, key, data)).rejects.toBe(error)
  })
})
