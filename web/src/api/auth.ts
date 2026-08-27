import { ProtocolError, refreshAccessCredential, request } from '../utils/request'

export interface LoginInput {
  email: string
  password: string
}

export interface AccessCredential {
  accessToken: string
  expiresIn: number
}

export interface CurrentUser {
  userId: number
  username: string
  email: string
  phone: string | null
}

export async function login(input: LoginInput): Promise<AccessCredential> {
  return request<AccessCredential>({ method: 'POST', url: '/api/v1/auth/login', data: input })
}

export async function refresh(): Promise<AccessCredential> {
  return refreshAccessCredential()
}

export async function logout(): Promise<void> {
  await request<Record<string, never>>({ method: 'POST', url: '/api/v1/auth/logout' })
}

export async function getCurrentUser(): Promise<CurrentUser> {
  return parseCurrentUser(await request<unknown>({ method: 'GET', url: '/api/v1/auth/me' }))
}

function parseCurrentUser(value: unknown): CurrentUser {
  if (!isExactRecord(value, ['userId', 'username', 'email', 'phone']) ||
    !isPositiveInteger(value.userId) || typeof value.username !== 'string' ||
    typeof value.email !== 'string' || !isNullableString(value.phone)) {
    throw new ProtocolError('current user response is invalid')
  }
  return { userId: value.userId, username: value.username, email: value.email, phone: value.phone }
}

function isExactRecord(value: unknown, keys: readonly string[]): value is Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return false
  const actual = Object.keys(value).sort()
  const expected = [...keys].sort()
  return actual.length === expected.length && actual.every((key, index) => key === expected[index])
}

function isPositiveInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value > 0
}

function isNullableString(value: unknown): value is string | null {
  return value === null || typeof value === 'string'
}
