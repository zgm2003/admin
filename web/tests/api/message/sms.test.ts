// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/message/sms'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('getSmsPageInit preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getSmsPageInit()
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/message/sms/page-init')
  })
  it('getSmsPageInit propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getSmsPageInit()).rejects.toBe(error)
  })
  it('getSmsConfig preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getSmsConfig()
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/message/sms/config')
  })
  it('getSmsConfig propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getSmsConfig()).rejects.toBe(error)
  })
  it('saveSmsConfig preserves the HTTP contract and backend data', async () => {
    const data: Parameters<typeof api.saveSmsConfig>[0] = {
      secretId: 'sample',
      secretKey: 'sample',
      smsSdkAppId: 'sample',
      signName: 'sample',
      region: 'sample',
      endpoint: 'sample',
      ttlMinutes: 1,
      isEnabled: 0,
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.saveSmsConfig(data)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/message/sms/config', data)
  })
  it('saveSmsConfig propagates request failures unchanged', async () => {
    const data: Parameters<typeof api.saveSmsConfig>[0] = {
      secretId: 'sample',
      secretKey: 'sample',
      smsSdkAppId: 'sample',
      signName: 'sample',
      region: 'sample',
      endpoint: 'sample',
      ttlMinutes: 1,
      isEnabled: 0,
    }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.saveSmsConfig(data)).rejects.toBe(error)
  })
  it('deleteSmsConfig preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.delete).mockResolvedValue(dataFromServer)
    const result = await api.deleteSmsConfig()
    expect(result).toBe(dataFromServer)
    expect(request.delete).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/message/sms/config')
  })
  it('deleteSmsConfig propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.delete).mockRejectedValue(error)
    await expect(api.deleteSmsConfig()).rejects.toBe(error)
  })
  it('sendSmsTest preserves the HTTP contract and backend data', async () => {
    const data: Parameters<typeof api.sendSmsTest>[0] = { toPhone: 'sample', scene: 'login' }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.sendSmsTest(data)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/message/sms/test', data)
  })
  it('sendSmsTest propagates request failures unchanged', async () => {
    const data: Parameters<typeof api.sendSmsTest>[0] = { toPhone: 'sample', scene: 'login' }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.sendSmsTest(data)).rejects.toBe(error)
  })
  it('listSmsTemplates preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.listSmsTemplates()
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/message/sms/template')
  })
  it('listSmsTemplates propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.listSmsTemplates()).rejects.toBe(error)
  })
  it('updateSmsTemplate preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateSmsTemplate>[0] = 1
    const data: Parameters<typeof api.updateSmsTemplate>[1] = {
      scene: 'login',
      name: 'sample',
      tencentTemplateId: 'sample',
      content: 'sample',
      variableKeys: ['sample'],
      exampleVariables: {},
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateSmsTemplate(id, data)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/message/sms/template/${id}`,
      data,
    )
  })
  it('updateSmsTemplate propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateSmsTemplate>[0] = 1
    const data: Parameters<typeof api.updateSmsTemplate>[1] = {
      scene: 'login',
      name: 'sample',
      tencentTemplateId: 'sample',
      content: 'sample',
      variableKeys: ['sample'],
      exampleVariables: {},
    }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateSmsTemplate(id, data)).rejects.toBe(error)
  })
  it('updateSmsTemplateStatus preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateSmsTemplateStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateSmsTemplateStatus>[1] = 0
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.patch).mockResolvedValue(dataFromServer)
    const result = await api.updateSmsTemplateStatus(id, isEnabled)
    expect(result).toBe(dataFromServer)
    expect(request.patch).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/message/sms/template/${id}/status`,
      { isEnabled },
    )
  })
  it('updateSmsTemplateStatus propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateSmsTemplateStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateSmsTemplateStatus>[1] = 0
    const error = new Error('request failed')
    vi.mocked(request.patch).mockRejectedValue(error)
    await expect(api.updateSmsTemplateStatus(id, isEnabled)).rejects.toBe(error)
  })
  it('listSmsRules preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.listSmsRules()
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/message/sms/recipient-rule')
  })
  it('listSmsRules propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.listSmsRules()).rejects.toBe(error)
  })
  it('createSmsRule preserves the HTTP contract and backend data', async () => {
    const data: Parameters<typeof api.createSmsRule>[0] = {
      scope: 0,
      pattern: 'sample',
      action: 0,
      name: 'sample',
      remark: 'sample',
      isEnabled: 0,
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.createSmsRule(data)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith(
      '/api/admin/v1/message/sms/recipient-rule',
      data,
    )
  })
  it('createSmsRule propagates request failures unchanged', async () => {
    const data: Parameters<typeof api.createSmsRule>[0] = {
      scope: 0,
      pattern: 'sample',
      action: 0,
      name: 'sample',
      remark: 'sample',
      isEnabled: 0,
    }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.createSmsRule(data)).rejects.toBe(error)
  })
  it('updateSmsRule preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateSmsRule>[0] = 1
    const data: Parameters<typeof api.updateSmsRule>[1] = {
      scope: 0,
      action: 0,
      name: 'sample',
      remark: 'sample',
      isEnabled: 0,
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateSmsRule(id, data)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/message/sms/recipient-rule/${id}`,
      data,
    )
  })
  it('updateSmsRule propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateSmsRule>[0] = 1
    const data: Parameters<typeof api.updateSmsRule>[1] = {
      scope: 0,
      action: 0,
      name: 'sample',
      remark: 'sample',
      isEnabled: 0,
    }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateSmsRule(id, data)).rejects.toBe(error)
  })
  it('updateSmsRuleStatus preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateSmsRuleStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateSmsRuleStatus>[1] = 0
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.patch).mockResolvedValue(dataFromServer)
    const result = await api.updateSmsRuleStatus(id, isEnabled)
    expect(result).toBe(dataFromServer)
    expect(request.patch).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/message/sms/recipient-rule/${id}/status`,
      { isEnabled },
    )
  })
  it('updateSmsRuleStatus propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateSmsRuleStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateSmsRuleStatus>[1] = 0
    const error = new Error('request failed')
    vi.mocked(request.patch).mockRejectedValue(error)
    await expect(api.updateSmsRuleStatus(id, isEnabled)).rejects.toBe(error)
  })
  it('deleteSmsRule preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.deleteSmsRule>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.delete).mockResolvedValue(dataFromServer)
    const result = await api.deleteSmsRule(id)
    expect(result).toBe(dataFromServer)
    expect(request.delete).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/message/sms/recipient-rule/${id}`,
    )
  })
  it('deleteSmsRule propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.deleteSmsRule>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.delete).mockRejectedValue(error)
    await expect(api.deleteSmsRule(id)).rejects.toBe(error)
  })
  it('listSmsLogs preserves the HTTP contract and backend data', async () => {
    const params: Parameters<typeof api.listSmsLogs>[0] = { page: 1, pageSize: 1 }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.listSmsLogs(params)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/message/sms/log', { params })
  })
  it('listSmsLogs propagates request failures unchanged', async () => {
    const params: Parameters<typeof api.listSmsLogs>[0] = { page: 1, pageSize: 1 }
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.listSmsLogs(params)).rejects.toBe(error)
  })
  it('getSmsLogDetail preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.getSmsLogDetail>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getSmsLogDetail(id)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(`/api/admin/v1/message/sms/log/${id}`)
  })
  it('getSmsLogDetail propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.getSmsLogDetail>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getSmsLogDetail(id)).rejects.toBe(error)
  })
  it('listSmsRateLimitPolicies preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.listSmsRateLimitPolicies()
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(
      '/api/admin/v1/message/sms/rate-limit-policy',
    )
  })
  it('listSmsRateLimitPolicies propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.listSmsRateLimitPolicies()).rejects.toBe(error)
  })
  it('updateSmsRateLimitPolicy preserves the HTTP contract and backend data', async () => {
    const platformId: Parameters<typeof api.updateSmsRateLimitPolicy>[0] = 1
    const key: Parameters<typeof api.updateSmsRateLimitPolicy>[1] = 'business_phone_minute'
    const data: Parameters<typeof api.updateSmsRateLimitPolicy>[2] = { limit: 1, windowSeconds: 1 }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateSmsRateLimitPolicy(platformId, key, data)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/message/sms/rate-limit-policy/${platformId}/${encodeURIComponent(key)}`,
      data,
    )
  })
  it('updateSmsRateLimitPolicy propagates request failures unchanged', async () => {
    const platformId: Parameters<typeof api.updateSmsRateLimitPolicy>[0] = 1
    const key: Parameters<typeof api.updateSmsRateLimitPolicy>[1] = 'business_phone_minute'
    const data: Parameters<typeof api.updateSmsRateLimitPolicy>[2] = { limit: 1, windowSeconds: 1 }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateSmsRateLimitPolicy(platformId, key, data)).rejects.toBe(error)
  })
})
