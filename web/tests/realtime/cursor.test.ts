import { beforeEach, describe, expect, it } from 'vitest'

import { createCursorStore, cursorStorageKey } from '@/realtime/cursor'

describe('realtime cursor store', () => {
  beforeEach(() => localStorage.clear())

  it('isolates identities and writes monotonically', () => {
    const store = createCursorStore(localStorage)
    store.write('admin', 7, 12)
    store.write('admin', 7, 5)
    expect(store.read('admin', 7)).toBe(12)
    expect(store.read('canvas', 7)).toBe(0)
    expect(store.read('admin', 8)).toBe(0)
  })

  it.each([
    'broken',
    '{}',
    '{"schemaVersion":1,"throughSequence":-1}',
    '{"schemaVersion":1,"throughSequence":1.5}',
    '{"schemaVersion":1,"throughSequence":2,"extra":1}',
  ])('removes corrupt value %s', (value) => {
    const key = cursorStorageKey('admin', 7)
    localStorage.setItem(key, value)
    expect(createCursorStore(localStorage).read('admin', 7)).toBe(0)
    expect(localStorage.getItem(key)).toBeNull()
  })
})
