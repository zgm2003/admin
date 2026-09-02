import { ProtocolError } from '@/types/http'

export const deviceIDStorageKey = 'admin:device-id'

const canonicalUUIDPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/

export function readDeviceID(storage: Storage = window.localStorage): string {
  const stored = storage.getItem(deviceIDStorageKey)
  if (stored !== null && canonicalUUIDPattern.test(stored)) {
    return stored
  }

  if (typeof crypto.randomUUID !== 'function') {
    throw new ProtocolError('crypto.randomUUID is required')
  }

  const generated = crypto.randomUUID().toLowerCase()
  if (!canonicalUUIDPattern.test(generated)) {
    throw new ProtocolError('crypto.randomUUID returned an invalid UUID')
  }

  storage.setItem(deviceIDStorageKey, generated)
  return generated
}
