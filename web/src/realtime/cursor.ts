import { ProtocolError } from '@/types/http'

export function cursorStorageKey(platformCode: string, userId: number): string {
  if (platformCode.trim() === '' || !Number.isInteger(userId) || userId <= 0)
    throw new ProtocolError('invalid realtime cursor identity')
  return `admin:realtime:cursor:v1:${platformCode}:${userId}`
}

export function createCursorStore(storage: Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>) {
  return {
    read(platformCode: string, userId: number): number {
      const key = cursorStorageKey(platformCode, userId)
      const raw = storage.getItem(key)
      if (raw === null) return 0
      try {
        const value: unknown = JSON.parse(raw)
        if (value === null || typeof value !== 'object' || Array.isArray(value))
          throw new Error('invalid')
        const keys = Object.keys(value)
        const record = value as Record<string, unknown>
        if (
          keys.length !== 2 ||
          !keys.includes('schemaVersion') ||
          !keys.includes('throughSequence') ||
          record.schemaVersion !== 1 ||
          typeof record.throughSequence !== 'number' ||
          !Number.isInteger(record.throughSequence) ||
          record.throughSequence < 0
        )
          throw new Error('invalid')
        return record.throughSequence
      } catch {
        storage.removeItem(key)
        return 0
      }
    },
    write(platformCode: string, userId: number, throughSequence: number): void {
      if (!Number.isInteger(throughSequence) || throughSequence < 0)
        throw new ProtocolError('throughSequence must be a nonnegative integer')
      const current = this.read(platformCode, userId)
      if (throughSequence > current)
        storage.setItem(
          cursorStorageKey(platformCode, userId),
          JSON.stringify({ schemaVersion: 1, throughSequence }),
        )
    },
  }
}
