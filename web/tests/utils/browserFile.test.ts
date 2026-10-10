import { describe, expect, it } from 'vitest'
import { decodeBase64Bytes } from '@/utils/browserFile'

describe('browser file decoding', () => {
  it('decodes arbitrary bytes without applying business file rules', () => {
    expect(Array.from(new Uint8Array(decodeBase64Bytes('AP+AUEs=')))).toEqual([0, 255, 128, 80, 75])
  })

  it('accepts an empty file', () => {
    expect(decodeBase64Bytes('').byteLength).toBe(0)
  })

  it.each(['?', 'AAA', 'AB==', 'AA==\n', '=AAA'])('rejects invalid Base64 %j', (value) => {
    expect(() => decodeBase64Bytes(value)).toThrow('Invalid Base64 content')
  })
})
