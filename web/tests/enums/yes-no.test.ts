import { describe, expect, it } from 'vitest'

import { isYesNo, YesNo } from '@src/enums/yes-no'

describe('YesNo', () => {
  it('uses the project-owned 0/1 codes', () => {
    expect(YesNo.No).toBe(0)
    expect(YesNo.Yes).toBe(1)
    expect(Object.values(YesNo)).not.toContain(2)
  })

	it('narrows only the two finite runtime values', () => {
		expect(isYesNo(YesNo.No)).toBe(true)
		expect(isYesNo(YesNo.Yes)).toBe(true)
		expect(isYesNo(2)).toBe(false)
		expect(isYesNo('1')).toBe(false)
		expect(isYesNo(null)).toBe(false)
	})
})
