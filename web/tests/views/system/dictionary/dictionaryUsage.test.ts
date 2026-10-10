import { describe, expect, it } from 'vitest'

import {
  dictionaryUsageByCode,
  getDictionaryUsage,
} from '@/views/system/dictionary/dictionaryUsage'

describe('dictionary usage registry', () => {
  it('describes all built-in dictionaries and their value policy', () => {
    expect(Object.keys(dictionaryUsageByCode).sort()).toEqual([
      'message.mail.region',
      'message.sms.region',
      'storage.cos.region',
      'storage.file.extension',
      'storage.mime.type',
      'user.gender',
    ])
    expect(getDictionaryUsage('user.gender')).toMatchObject({
      valuePolicy: 'fixed',
      consumerLabelKey: 'dictionary.usage.profile',
    })
    expect(getDictionaryUsage('storage.file.extension')).toMatchObject({
      valuePolicy: 'extensible',
      consumerLabelKey: 'dictionary.usage.uploadRule',
    })
  })

  it('returns a safe fallback for a custom dictionary without a consumer', () => {
    expect(getDictionaryUsage('user.level')).toEqual({
      consumerLabelKey: 'dictionary.usage.none',
      impactLabelKey: 'dictionary.impact.none',
      valuePolicy: 'extensible',
      valueHintKey: 'dictionary.valueHint.custom',
    })
  })
})
