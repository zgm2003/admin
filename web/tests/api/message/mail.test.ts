import { describe, expect, it } from 'vitest'
import {
  parseMailConfig,
  parseMailLog,
  parseMailLogDetail,
  parseMailRule,
  parseMailTemplate,
  parseMailLogPage,
  parseMailRateLimitPolicy,
  parseMailRateLimitSnapshot,
  parseMailRateLimitUpdateResult,
} from '@/api/message/mail'

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

describe('mail rate limit protocol', () => {
  const policyMetadata = {
    business_email_minute: ['business', 'platform_scene_email'],
    business_email_10m: ['business', 'platform_scene_email'],
    business_ip_minute: ['business', 'platform_ip'],
    business_scene_minute: ['business', 'platform_scene'],
    admin_test_user_10m: ['admin_test', 'admin_user'],
    admin_test_ip_minute: ['admin_test', 'ip'],
    admin_test_email_10m: ['admin_test', 'email'],
  } as const
  const policy = policyFor('business_email_minute')

  function policyFor(key: keyof typeof policyMetadata) {
    const [mode, dimension] = policyMetadata[key]
    return { key, mode, dimension, limit: 1, windowSeconds: 60, updatedAt: '2026-09-04T12:00:00Z' }
  }

  it('accepts the exact policy and snapshot shapes', () => {
    expect(parseMailRateLimitPolicy(policy)).toEqual(policy)
    const sevenPolicies = Object.keys(policyMetadata).map((key) =>
      policyFor(key as keyof typeof policyMetadata),
    )
    expect(parseMailRateLimitSnapshot({ version: 3, policies: sevenPolicies })).toEqual({
      version: 3,
      policies: sevenPolicies,
    })
    expect(parseMailRateLimitUpdateResult({ version: 3, policy })).toEqual({
      version: 3,
      policy,
    })
    expect(() => parseMailRateLimitUpdateResult({ version: 3, policy }, 'business_ip_minute')).toThrow()
  })

  it('rejects unknown fields, invalid keys and out-of-range values', () => {
    expect(() => parseMailRateLimitPolicy({ ...policy, extra: true })).toThrow()
    expect(() => parseMailRateLimitPolicy({ ...policy, key: 'unknown' })).toThrow()
    expect(() => parseMailRateLimitPolicy({ ...policy, limit: 0 })).toThrow()
    expect(() => parseMailRateLimitPolicy({ ...policy, windowSeconds: 0 })).toThrow()
    expect(() => parseMailRateLimitPolicy({ ...policy, mode: 'invalid' })).toThrow()
    expect(() => parseMailRateLimitPolicy({ ...policy, updatedAt: 'not-a-date' })).toThrow()
    expect(() => parseMailRateLimitPolicy({ ...policy, mode: 'admin_test' })).toThrow()
    expect(() => parseMailRateLimitPolicy({ ...policy, dimension: 'email' })).toThrow()
  })

  it('rejects incomplete snapshots and missing version', () => {
    expect(() => parseMailRateLimitSnapshot({ policies: [policy] })).toThrow()
    expect(() => parseMailRateLimitSnapshot({ version: 0, policies: [policy] })).toThrow()
    expect(() => parseMailRateLimitUpdateResult({ policy })).toThrow()
  })

  it('rejects snapshots with duplicate policy keys', () => {
    const sevenPolicies = Object.keys(policyMetadata).map((key) =>
      policyFor(key as keyof typeof policyMetadata),
    )
    expect(() =>
      parseMailRateLimitSnapshot({
        version: 3,
        policies: [...sevenPolicies, policyFor('business_email_minute')],
      }),
    ).toThrow()
  })
})
