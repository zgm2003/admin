import { describe, expect, it } from 'vitest'

import { routeViews } from './route-views'

describe('route views', () => {
  it('registers the authentication platform view', () => {
    expect(routeViews['system-auth-platforms']).toBeTypeOf('function')
  })
})
