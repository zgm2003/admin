export type DictionaryValuePolicy = 'fixed' | 'extensible'

export interface DictionaryUsage {
  consumerLabelKey: string
  impactLabelKey: string
  valuePolicy: DictionaryValuePolicy
  valueHintKey: string
}

const customDictionaryUsage: DictionaryUsage = {
  consumerLabelKey: 'dictionary.usage.none',
  impactLabelKey: 'dictionary.impact.none',
  valuePolicy: 'extensible',
  valueHintKey: 'dictionary.valueHint.custom',
}

export const dictionaryUsageByCode: Readonly<Record<string, DictionaryUsage>> = {
  'user.gender': {
    consumerLabelKey: 'dictionary.usage.profile',
    impactLabelKey: 'dictionary.impact.gender',
    valuePolicy: 'fixed',
    valueHintKey: 'dictionary.valueHint.gender',
  },
  'message.mail.region': {
    consumerLabelKey: 'dictionary.usage.mailConfig',
    impactLabelKey: 'dictionary.impact.mailRegion',
    valuePolicy: 'extensible',
    valueHintKey: 'dictionary.valueHint.mailRegion',
  },
  'message.sms.region': {
    consumerLabelKey: 'dictionary.usage.smsConfig',
    impactLabelKey: 'dictionary.impact.smsRegion',
    valuePolicy: 'extensible',
    valueHintKey: 'dictionary.valueHint.smsRegion',
  },
  'storage.cos.region': {
    consumerLabelKey: 'dictionary.usage.cosConfig',
    impactLabelKey: 'dictionary.impact.cosRegion',
    valuePolicy: 'extensible',
    valueHintKey: 'dictionary.valueHint.cosRegion',
  },
  'storage.file.extension': {
    consumerLabelKey: 'dictionary.usage.uploadRule',
    impactLabelKey: 'dictionary.impact.fileExtension',
    valuePolicy: 'extensible',
    valueHintKey: 'dictionary.valueHint.fileExtension',
  },
  'storage.mime.type': {
    consumerLabelKey: 'dictionary.usage.uploadRule',
    impactLabelKey: 'dictionary.impact.mimeType',
    valuePolicy: 'extensible',
    valueHintKey: 'dictionary.valueHint.mimeType',
  },
}

export function getDictionaryUsage(code: string): DictionaryUsage {
  return dictionaryUsageByCode[code] ?? customDictionaryUsage
}
