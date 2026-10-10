export const SmsRuleScope = {
  Phone: 0,
  Prefix: 1,
} as const
export type SmsRuleScope = (typeof SmsRuleScope)[keyof typeof SmsRuleScope]

export const smsRuleScopeMetadata = [
  { value: SmsRuleScope.Phone, i18nKey: 'sms.scope.phone' },
  { value: SmsRuleScope.Prefix, i18nKey: 'sms.scope.prefix' },
] as const

export const SmsRuleAction = {
  Deny: 0,
  Allow: 1,
} as const
export type SmsRuleAction = (typeof SmsRuleAction)[keyof typeof SmsRuleAction]

export const smsRuleActionMetadata = [
  { value: SmsRuleAction.Allow, i18nKey: 'sms.action.allow' },
  { value: SmsRuleAction.Deny, i18nKey: 'sms.action.deny' },
] as const

export function isSmsRuleScope(value: unknown): value is SmsRuleScope {
  return value === SmsRuleScope.Phone || value === SmsRuleScope.Prefix
}

export function isSmsRuleAction(value: unknown): value is SmsRuleAction {
  return value === SmsRuleAction.Deny || value === SmsRuleAction.Allow
}
