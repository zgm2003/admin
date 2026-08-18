import { ProtocolError } from '../types/http'

export interface RegisterInput {
  username: string
  email: string
  password: string
  confirmPassword: string
}

export interface LoginInput {
  username: string
  password: string
}

export interface RegisteredUser {
  userId: number
  username: string
  email: string
}

export interface AccessCredential {
  accessToken: string
  expiresIn: number
}

export interface CurrentUser {
  userId: number
  username: string
  email: string
}

export function parseRegisteredUser(value: unknown): RegisteredUser {
  return parseUser(value, 'registered user')
}

export function parseCredential(value: unknown): AccessCredential {
  const record = closedRecord(value, ['accessToken', 'expiresIn'], 'access credential')
  if (typeof record.accessToken !== 'string' || record.accessToken === '') {
    throw new ProtocolError('access credential accessToken must be a non-empty string')
  }
  if (typeof record.expiresIn !== 'number' || !Number.isInteger(record.expiresIn) || record.expiresIn <= 0) {
    throw new ProtocolError('access credential expiresIn must be a positive integer')
  }
  return { accessToken: record.accessToken, expiresIn: record.expiresIn }
}

export function parseCurrentUser(value: unknown): CurrentUser {
  return parseUser(value, 'current user')
}

function parseUser(value: unknown, label: string): RegisteredUser {
  const record = closedRecord(value, ['email', 'userId', 'username'], label)
  if (typeof record.userId !== 'number' || !Number.isInteger(record.userId) || record.userId <= 0) {
    throw new ProtocolError(`${label} userId must be a positive integer`)
  }
  if (typeof record.username !== 'string' || record.username === '') {
    throw new ProtocolError(`${label} username must be a non-empty string`)
  }
  if (typeof record.email !== 'string' || record.email === '') {
    throw new ProtocolError(`${label} email must be a non-empty string`)
  }
  return { userId: record.userId, username: record.username, email: record.email }
}

function closedRecord(value: unknown, expectedKeys: string[], label: string): Record<string, unknown> {
  if (!isRecord(value)) {
    throw new ProtocolError(`${label} must be an object`)
  }
  const record = value
  const actualKeys = Object.keys(record).sort()
  const sortedExpectedKeys = [...expectedKeys].sort()
  if (actualKeys.length !== sortedExpectedKeys.length || actualKeys.some((key, index) => key !== sortedExpectedKeys[index])) {
    throw new ProtocolError(`${label} contains unexpected or missing fields`)
  }
  return record
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
