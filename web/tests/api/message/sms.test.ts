import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  createSmsRule,
  deleteSmsConfig,
  deleteSmsRule,
  getSmsConfig,
  getSmsLogDetail,
  getSmsPageInit,
  listSmsLogs,
  listSmsRateLimitPolicies,
  listSmsRules,
  listSmsTemplates,
  saveSmsConfig,
  sendSmsTest,
  updateSmsRateLimitPolicy,
  updateSmsRule,
  updateSmsRuleStatus,
  updateSmsTemplate,
  updateSmsTemplateStatus,
} from '@/api/message/sms'
import { YesNo } from '@/enums/yesNo'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({ request: vi.fn() }))

const requestMock = vi.mocked(request)
const timestamp = '2026-09-11T08:00:00Z'

const config = {
  configured: true,
  smsSdkAppId: '1400000000',
  signName: 'Admin',
  region: 'ap-guangzhou',
  endpoint: '',
  ttlMinutes: 5,
  isEnabled: YesNo.Yes,
  lastTestAt: timestamp,
  lastTestError: '',
}

const template = {
  id: 1,
  scene: 'login' as const,
  name: 'Login code',
  tencentTemplateId: '100001',
  parameterKeys: ['code', 'ttl_minutes'] as ['code', 'ttl_minutes'],
  exampleVariables: { code: '123456', ttl_minutes: '5' },
  isEnabled: YesNo.Yes,
  createdAt: timestamp,
  updatedAt: timestamp,
}

const rule = {
  id: 2,
  scope: 'phone' as const,
  patternHint: '156****8271',
  action: 'deny' as const,
  name: 'Blocked recipient',
  remark: '',
  isEnabled: YesNo.Yes,
  createdAt: timestamp,
  updatedAt: timestamp,
}

const log = {
  id: 3,
  platformId: 1,
  platform: 'admin',
  userId: 7,
  username: 'alice',
  scene: 'login' as const,
  templateId: 1,
  toPhoneHint: '156****8271',
  status: 'sent' as const,
  requestId: 'request-id',
  serialNo: 'serial-no',
  fee: 1,
  errorCode: '',
  errorSummary: '',
  latencyMs: 30,
  sentAt: timestamp,
  createdAt: timestamp,
  updatedAt: timestamp,
}

const policies = [
  {
    key: 'business_phone_minute' as const,
    mode: 'business' as const,
    dimension: 'platform_phone' as const,
    limit: 1,
    windowSeconds: 60,
    revision: 2,
    updatedAt: timestamp,
  },
  {
    key: 'business_phone_10m' as const,
    mode: 'business' as const,
    dimension: 'platform_phone' as const,
    limit: 5,
    windowSeconds: 600,
    revision: 2,
    updatedAt: timestamp,
  },
]

describe('SMS admin API protocol', () => {
  beforeEach(() => requestMock.mockReset())

  it('parses exact config and sends exact config requests', async () => {
    requestMock.mockResolvedValueOnce(config)
    await expect(getSmsConfig()).resolves.toEqual(config)
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'GET',
      url: '/api/admin/v1/message/sms/config',
    })

    const input = {
      secretId: '',
      secretKey: '',
      smsSdkAppId: '1400000000',
      signName: 'Admin',
      region: 'ap-guangzhou',
      endpoint: '',
      ttlMinutes: 5,
      isEnabled: YesNo.Yes,
    }
    requestMock.mockResolvedValueOnce(config)
    await expect(saveSmsConfig(input)).resolves.toEqual(config)
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'PUT',
      url: '/api/admin/v1/message/sms/config',
      data: input,
    })

    requestMock.mockResolvedValueOnce({})
    await expect(deleteSmsConfig()).resolves.toBeUndefined()
  })

  it.each([
    { ...config, secretId: 'must-not-leak' },
    { ...config, configured: undefined },
    { ...config, ttlMinutes: 0 },
    { ...config, ttlMinutes: 61 },
    { ...config, isEnabled: 2 },
    { ...config, lastTestAt: '2026-09-11' },
  ])('rejects malformed or unsafe config responses', async (response) => {
    requestMock.mockResolvedValue(response)
    await expect(getSmsConfig()).rejects.toThrow()
  })

  it('freezes the exact four scene catalog and strict template shape', async () => {
    const scenes = [
      { scene: 'login', name: 'Login', parameterKeys: ['code', 'ttl_minutes'] },
      { scene: 'forget', name: 'Forget', parameterKeys: ['code', 'ttl_minutes'] },
      { scene: 'bind_phone', name: 'Bind phone', parameterKeys: ['code', 'ttl_minutes'] },
      {
        scene: 'change_password',
        name: 'Change password',
        parameterKeys: ['code', 'ttl_minutes'],
      },
    ]
    requestMock.mockResolvedValueOnce({ scenes })
    await expect(getSmsPageInit()).resolves.toEqual({ scenes })
    requestMock.mockResolvedValueOnce({ list: [template] })
    await expect(listSmsTemplates()).resolves.toEqual([template])
  })

  it.each([
    {
      scenes: Array.from({ length: 4 }, () => ({
        scene: 'login',
        name: 'Duplicate',
        parameterKeys: ['code', 'ttl_minutes'],
      })),
    },
    {
      scenes: [
        { scene: 'login', name: 'Login', parameterKeys: ['code', 'ttl_minutes'] },
        { scene: 'forget', name: 'Forget', parameterKeys: ['code', 'ttl_minutes'] },
        { scene: 'bind_phone', name: 'Bind', parameterKeys: ['code', 'ttl_minutes'] },
      ],
    },
  ])('rejects incomplete or duplicate scene catalogs', async (response) => {
    requestMock.mockResolvedValue(response)
    await expect(getSmsPageInit()).rejects.toThrow()
  })

  it.each([
    { ...template, exampleVariables: { ...template.exampleVariables, extra: 'value' } },
    { ...template, parameterKeys: ['ttl_minutes', 'code'] },
    { ...template, scene: 'test' },
    { ...template, id: 0 },
    { ...template, createdAt: 'not-a-date' },
  ])('rejects malformed template responses', async (item) => {
    requestMock.mockResolvedValue({ list: [item] })
    await expect(listSmsTemplates()).rejects.toThrow()
  })

  it('uses exact template mutation paths and bodies', async () => {
    const input = {
      scene: template.scene,
      name: template.name,
      tencentTemplateId: template.tencentTemplateId,
      parameterKeys: template.parameterKeys,
      exampleVariables: template.exampleVariables,
    }
    requestMock.mockResolvedValueOnce(template)
    await updateSmsTemplate(template.id, input)
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'PUT',
      url: '/api/admin/v1/message/sms/template/1',
      data: input,
    })
    requestMock.mockResolvedValueOnce({})
    await updateSmsTemplateStatus(template.id, YesNo.No)
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'PATCH',
      url: '/api/admin/v1/message/sms/template/1/status',
      data: { isEnabled: YesNo.No },
    })
  })

  it('parses strict recipient rules and uses exact mutation paths', async () => {
    requestMock.mockResolvedValueOnce({ list: [rule] })
    await expect(listSmsRules()).resolves.toEqual([rule])

    const createInput = {
      scope: 'phone' as const,
      pattern: '15671628271',
      action: 'deny' as const,
      name: 'Blocked recipient',
      remark: '',
      isEnabled: YesNo.Yes,
    }
    requestMock.mockResolvedValueOnce(rule)
    await createSmsRule(createInput)
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'POST',
      url: '/api/admin/v1/message/sms/recipient-rule',
      data: createInput,
    })

    const updateInput = { ...createInput, pattern: undefined }
    requestMock.mockResolvedValueOnce(rule)
    await updateSmsRule(rule.id, updateInput)
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'PUT',
      url: '/api/admin/v1/message/sms/recipient-rule/2',
      data: updateInput,
    })
    requestMock.mockResolvedValueOnce({})
    await updateSmsRuleStatus(rule.id, YesNo.No)
    requestMock.mockResolvedValueOnce({})
    await deleteSmsRule(rule.id)
  })

  it.each([
    { ...rule, scope: 'domain' },
    { ...rule, action: 'block' },
    { ...rule, isEnabled: 2 },
    { ...rule, unknown: true },
  ])('rejects malformed recipient rules', async (item) => {
    requestMock.mockResolvedValue({ list: [item] })
    await expect(listSmsRules()).rejects.toThrow()
  })

  it('parses logs, protected detail, exact-phone filters, and admin test results', async () => {
    requestMock.mockResolvedValueOnce({ list: [log], total: 1, page: 1, pageSize: 20 })
    await expect(
      listSmsLogs({ page: 1, pageSize: 20, toPhone: '15671628271', scene: 'login' }),
    ).resolves.toEqual({ list: [log], total: 1, page: 1, pageSize: 20 })
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'GET',
      url: '/api/admin/v1/message/sms/log',
      params: { page: 1, pageSize: 20, toPhone: '15671628271', scene: 'login' },
    })

    const detail = {
      log,
      toPhone: '+8615671628271',
      verificationCode: '123456',
      verificationExpiresAt: timestamp,
    }
    requestMock.mockResolvedValueOnce(detail)
    await expect(getSmsLogDetail(log.id)).resolves.toEqual(detail)

    const testInput = { toPhone: '15671628271', scene: 'login' as const }
    const testResult = { logId: 3, status: 'sent', requestId: 'request-id', serialNo: 'serial-no' }
    requestMock.mockResolvedValueOnce(testResult)
    await expect(sendSmsTest(testInput)).resolves.toEqual(testResult)
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'POST',
      url: '/api/admin/v1/message/sms/test',
      data: testInput,
    })
  })

  it.each([
    { ...log, id: 0 },
    { ...log, platformId: -1 },
    { ...log, fee: -1 },
    { ...log, latencyMs: -1 },
    { ...log, status: 'unknown' },
    { ...log, sentAt: '2026-09-11' },
  ])('rejects malformed log responses', async (item) => {
    requestMock.mockResolvedValue({ list: [item], total: 1, page: 1, pageSize: 20 })
    await expect(listSmsLogs({ page: 1, pageSize: 20 })).rejects.toThrow()
  })

  it('rejects unsafe detail fields and malformed pages', async () => {
    requestMock.mockResolvedValueOnce({ list: [], total: -1, page: 1, pageSize: 20 })
    await expect(listSmsLogs({ page: 1, pageSize: 20 })).rejects.toThrow()
    requestMock.mockResolvedValueOnce({
      log,
      toPhone: '15671628271',
      verificationCode: '123456',
      verificationExpiresAt: timestamp,
    })
    await expect(getSmsLogDetail(log.id)).rejects.toThrow()
    requestMock.mockResolvedValueOnce({
      log,
      toPhone: '+8615671628271',
      verificationCode: '123456',
      verificationExpiresAt: timestamp,
      ciphertext: 'secret',
    })
    await expect(getSmsLogDetail(log.id)).rejects.toThrow()
  })

  it('parses two fixed rate policies per platform and updates one exact policy', async () => {
    const snapshot = {
      platforms: [{ platformId: 1, platformCode: 'admin', platformName: 'Admin', policies }],
    }
    requestMock.mockResolvedValueOnce(snapshot)
    await expect(listSmsRateLimitPolicies()).resolves.toEqual(snapshot)

    const updated = {
      ...snapshot.platforms[0],
      policies: [{ ...policies[0], limit: 2 }, policies[1]],
    }
    requestMock.mockResolvedValueOnce(updated)
    await expect(
      updateSmsRateLimitPolicy(1, 'business_phone_minute', {
        limit: 2,
        windowSeconds: 60,
      }),
    ).resolves.toEqual(updated)
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'PUT',
      url: '/api/admin/v1/message/sms/rate-limit-policy/1/business_phone_minute',
      data: { limit: 2, windowSeconds: 60 },
    })
  })

  it.each([
    { invalidPolicies: [{ ...policies[0], limit: 0 }, policies[1]] },
    { invalidPolicies: [{ ...policies[0], windowSeconds: 0 }, policies[1]] },
    { invalidPolicies: [{ ...policies[0], revision: 0 }, policies[1]] },
    {
      invalidPolicies: [policies[0], { ...policies[1], key: 'business_phone_minute' }],
    },
    { invalidPolicies: [policies[0], { ...policies[1], key: 'unknown' }] },
  ])('rejects malformed rate policy snapshots', async ({ invalidPolicies }) => {
    requestMock.mockResolvedValue({
      platforms: [
        {
          platformId: 1,
          platformCode: 'admin',
          platformName: 'Admin',
          policies: invalidPolicies,
        },
      ],
    })
    await expect(listSmsRateLimitPolicies()).rejects.toThrow()
  })
})
