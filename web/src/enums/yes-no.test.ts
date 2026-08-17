import { describe, expect, it } from 'vitest'

import { YesNo } from './yes-no'

describe('YesNo', () => {
  it('uses the project-owned 0/1 codes', () => {
    expect(YesNo.No).toBe(0)
    expect(YesNo.Yes).toBe(1)
    expect(Object.values(YesNo)).not.toContain(2)
  })
})
