import { describe, expect, it } from 'vitest'
import { isStorageObjectKey } from '@/utils/storageObjectKey'

const key = 'file/.admin-storage/v2/p1/r1/c1/v1/2026/09/30/0123456789abcdef0123456789abcdef.csv'

describe('storage object key protocol', () => {
  it('accepts a versioned key and rejects URLs, legacy keys and corrupt coordinates', () => {
    expect(isStorageObjectKey(key)).toBe(true)
    for (const invalid of [
      `https://example.com/${key}`,
      'file/template.csv',
      `/${key}`,
      key.replace('/p1/', '/p0/'),
      key.replace('/r1/', '/r9223372036854775808/'),
      key.replace('/09/30/', '/02/30/'),
      key.replace('/v1/', '/v01/'),
      `${key}?download=1`,
      `${key}\n`,
      `${key}\r`,
    ])
      expect(isStorageObjectKey(invalid), invalid).toBe(false)
  })
})
