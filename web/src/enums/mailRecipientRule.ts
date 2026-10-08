export const MailRuleScope = {
  Email: 0,
  Domain: 1,
} as const

export type MailRuleScope = (typeof MailRuleScope)[keyof typeof MailRuleScope]

export const MailRuleAction = {
  Deny: 0,
  Allow: 1,
} as const

export type MailRuleAction = (typeof MailRuleAction)[keyof typeof MailRuleAction]

export function isMailRuleScope(value: unknown): value is MailRuleScope {
  return value === MailRuleScope.Email || value === MailRuleScope.Domain
}

export function isMailRuleAction(value: unknown): value is MailRuleAction {
  return value === MailRuleAction.Deny || value === MailRuleAction.Allow
}
