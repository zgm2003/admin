import { ProtocolError } from '@/types/http'

export function expectRecord(value: unknown, context: string): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    throw new ProtocolError(`${context} must be an object`)
  }
  return value as Record<string, unknown>
}

export function expectExactKeys(
  value: unknown,
  keys: readonly string[],
  context: string,
): Record<string, unknown> {
  const record = expectRecord(value, context)
  const actual = Object.keys(record).sort()
  const expected = [...keys].sort()
  if (actual.length !== expected.length || actual.some((key, index) => key !== expected[index])) {
    throw new ProtocolError(`${context} has invalid fields`)
  }
  return record
}

export function expectString(value: unknown, context: string): string {
  if (typeof value !== 'string') throw new ProtocolError(`${context} must be a string`)
  return value
}

export function expectInteger(value: unknown, context: string): number {
  if (typeof value !== 'number' || !Number.isInteger(value)) {
    throw new ProtocolError(`${context} must be an integer`)
  }
  return value
}

export function expectBoolean(value: unknown, context: string): boolean {
  if (typeof value !== 'boolean') throw new ProtocolError(`${context} must be a boolean`)
  return value
}

export function expectArray(value: unknown, context: string): unknown[] {
  if (!Array.isArray(value)) throw new ProtocolError(`${context} must be an array`)
  return value
}

export function expectNullableString(value: unknown, context: string): string | null {
  if (value !== null && typeof value !== 'string') {
    throw new ProtocolError(`${context} must be a string or null`)
  }
  return value
}

export function expectPage<T>(
  value: unknown,
  parseItem: (item: unknown, index: number) => T,
  context: string,
) {
  const record = expectRecord(value, context)
  const list = expectArray(record.list, `${context}.list`).map(parseItem)
  return {
    list,
    total: expectInteger(record.total, `${context}.total`),
    page: expectInteger(record.page, `${context}.page`),
    pageSize: expectInteger(record.pageSize, `${context}.pageSize`),
  }
}

export function expectEmptyObject(value: unknown, context: string): Record<string, never> {
  const record = expectRecord(value, context)
  if (Object.keys(record).length !== 0) throw new ProtocolError(`${context} must be empty`)
  return {}
}

export function expectId(value: unknown, context: string): { id: number } {
  const record = expectRecord(value, context)
  if (Object.keys(record).some((key) => key !== 'id'))
    throw new ProtocolError(`${context} has unknown fields`)
  return { id: expectInteger(record.id, `${context}.id`) }
}
