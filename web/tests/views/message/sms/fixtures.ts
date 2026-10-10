import type { SmsOptions } from '@/api/message/sms'

export const ttlConstraints = { minTTLMinutes: 1, maxTTLMinutes: 60 }
export const rateLimitConstraints = {
  minLimit: 1,
  maxLimit: 100000,
  minWindowSeconds: 1,
  maxWindowSeconds: 86400,
}

function defaultOptions(english = false): SmsOptions {
  return {
    rateLimitConstraints,
    scenes: [
      {
        value: 'login',
        label: english ? 'Login verification code' : '登录验证码',
        variableKeys: ['code', 'ttl_minutes'],
      },
    ],
    statuses: [
      { value: 1, label: english ? 'Pending' : '待发送', tone: 'info' },
      { value: 2, label: english ? 'Sent' : '已发送', tone: 'success' },
      { value: 3, label: english ? 'Failed' : '发送失败', tone: 'danger' },
    ],
    ruleScopes: [
      { value: 0, label: english ? 'Email' : '邮箱' },
      { value: 1, label: english ? 'Domain' : '域名' },
    ],
    ruleActions: [
      { value: 1, label: english ? 'Allowlist' : '允许', tone: 'success' },
      { value: 0, label: english ? 'Denylist' : '拒绝', tone: 'danger' },
    ],
    ruleDefaults: { scope: 0, action: 0 },
    rateLimitPolicies: [
      { value: 'business_email_minute', label: '每分钟发送上限' },
      { value: 'business_email_10m', label: '每 10 分钟发送上限' },
    ],
    rateLimitModes: [{ value: 'business', label: '业务发送' }],
    rateLimitDimensions: [{ value: 'platform_email', label: '平台·邮箱' }],
  }
}

export function smsOptions(english = false): SmsOptions {
  const mail = defaultOptions(english)
  return {
    ...mail,
    scenes: [
      { value: 'login', label: '登录验证码', variableKeys: ['code', 'ttl_minutes'] },
      { value: 'forget', label: '找回密码', variableKeys: ['code', 'ttl_minutes'] },
      { value: 'bind_phone', label: '绑定/换绑手机', variableKeys: ['code', 'ttl_minutes'] },
      { value: 'change_password', label: '修改密码', variableKeys: ['code', 'ttl_minutes'] },
    ],
    ruleScopes: [
      { value: 0, label: '手机号' },
      { value: 1, label: '号段' },
    ],
    rateLimitPolicies: [
      { value: 'business_phone_minute', label: '每分钟发送上限' },
      { value: 'business_phone_10m', label: '每 10 分钟发送上限' },
    ],
    rateLimitDimensions: [{ value: 'platform_phone', label: '平台·手机号' }],
  }
}
