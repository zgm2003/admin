import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ProtocolError } from '../types/http'
import { deviceIDStorageKey, readDeviceID } from './device-id'

const validUUID = '550e8400-e29b-41d4-a716-446655440000'

describe('device id', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.stubGlobal('crypto', { randomUUID: vi.fn(() => validUUID) })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('generates and persists a canonical UUID on first read', () => {
    expect(readDeviceID()).toBe(validUUID)
    expect(localStorage.getItem(deviceIDStorageKey)).toBe(validUUID)
  })

  it('reuses a valid persisted UUID without generating another', () => {
    localStorage.setItem(deviceIDStorageKey, validUUID)
    const randomUUID = vi.mocked(crypto.randomUUID)

    expect(readDeviceID()).toBe(validUUID)
    expect(randomUUID).not.toHaveBeenCalled()
  })

  it('replaces corrupt storage with a newly generated UUID', () => {
    localStorage.setItem(deviceIDStorageKey, 'not-a-uuid')

    expect(readDeviceID()).toBe(validUUID)
    expect(localStorage.getItem(deviceIDStorageKey)).toBe(validUUID)
  })

  it('canonicalizes generated UUIDs to lower case', () => {
    vi.stubGlobal('crypto', { randomUUID: vi.fn(() => validUUID.toUpperCase()) })

    expect(readDeviceID()).toBe(validUUID)
  })

  it('propagates storage write failures', () => {
    const storage = {
      getItem: () => null,
      setItem: () => { throw new Error('storage is unavailable') },
    } as unknown as Storage

    expect(() => readDeviceID(storage)).toThrow('storage is unavailable')
  })

  it('fails explicitly when randomUUID is unavailable', () => {
    vi.stubGlobal('crypto', {})

    expect(() => readDeviceID()).toThrow(ProtocolError)
  })

  it('fails explicitly when randomUUID returns an invalid value', () => {
    vi.stubGlobal('crypto', { randomUUID: vi.fn(() => 'invalid') })

    expect(() => readDeviceID()).toThrow(ProtocolError)
  })
})
