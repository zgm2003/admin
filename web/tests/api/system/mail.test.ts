import { describe, expect, it } from 'vitest'
import {
  parseMailConfig,
  parseMailLog,
  parseMailLogDetail,
  parseMailRule,
  parseMailTemplate,
  parseMailLogPage,
} from '@/api/system/mail'

describe('mail config protocol', () => {
  const valid = {
    configured: true,
    region: 'ap-guangzhou',
    endpoint: '',
    fromEmail: 'noreply@example.com',
    fromName: 'Admin',
    replyTo: '',
    ttlMinutes: 10,
    isEnabled: 1,
    lastTestAt: null,
    lastTestError: '',
  }
  it('accepts the safe configuration projection', () => {
    expect(parseMailConfig(valid)).toEqual(valid)
  })
  it('rejects secrets and unknown fields', () => {
    expect(() => parseMailConfig({ ...valid, secretId: 'secret' })).toThrow()
    expect(() => parseMailConfig({ ...valid, configured: undefined })).toThrow()
  })
})

describe('mail admin protocol', () => {
  const template = {
    id: 1,
    platformId: 1,
    scene: 'login',
    name: '登录验证码',
    subject: '登录验证码',
    tencentTemplateId: 47941,
    variables: { code: '123456', ttl_minutes: '10' },
    exampleVariables: { code: '123456', ttl_minutes: '10' },
    isEnabled: 1,
    createdAt: '2026-09-01T00:00:00Z',
    updatedAt: '2026-09-01T00:00:00Z',
  }
  const log = {
    id: 2,
    platformId: 1,
    userId: 169,
    scene: 'login',
    templateId: 47941,
    toEmail: 'admin@example.com',
    subject: '登录验证码',
    status: 'sent',
    requestId: 'req',
    messageId: 'msg',
    errorCode: '',
    errorSummary: '',
    latencyMs: 32,
    sentAt: '2026-09-01T00:00:01Z',
    createdAt: '2026-09-01T00:00:00Z',
    updatedAt: '2026-09-01T00:00:01Z',
  }
  const rule = {
    id: 3,
    platformId: 1,
    scope: 'domain',
    pattern: 'example.com',
    action: 'deny',
    name: '临时邮箱',
    remark: '阻断',
    isEnabled: 1,
    createdAt: '2026-09-01T00:00:00Z',
    updatedAt: '2026-09-01T00:00:00Z',
  }

  it('accepts exact template, log and recipient rule projections', () => {
    expect(parseMailTemplate(template)).toEqual(template)
    expect(parseMailLog(log)).toEqual(log)
    expect(parseMailRule(rule)).toEqual(rule)
  })

  it('rejects unknown fields and malformed pages', () => {
    expect(() => parseMailTemplate({ ...template, secretId: 'secret' })).toThrow()
    expect(() => parseMailLogPage({ list: [log], total: 1, page: 1 })).toThrow()
  })

  it('accepts protected log detail without ciphertext', () => {
    const value = { log, verificationCode: '123456', verificationExpiresAt: '2026-09-01T00:10:00Z' }
    expect(parseMailLogDetail(value)).toEqual(value)
    expect(() => parseMailLogDetail({ ...value, codeCiphertext: 'mail:v1:secret' })).toThrow()
  })
})
