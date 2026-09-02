import { describe, expect, it } from 'vitest'

import { authPlatform } from '@/auth/platform'

describe('auth platform', () => {
  it('uses the fixed admin platform code', () => {
    expect(authPlatform).toBe('admin')
  })
})
