import { describe, expect, it } from 'vitest'

import { routeViews } from '@src/access/route-views'

describe('route views', () => {
  it('registers every builtin system page view', () => {
    expect(routeViews['system-auth-platforms']).toBeTypeOf('function')
		expect(routeViews['system-sessions']).toBeTypeOf('function')
		expect(routeViews['system-operation-logs']).toBeTypeOf('function')
  })
})
