import { refreshAccessCredential, request } from '@/utils/request'
import {
  expectExactKeys,
  expectEmptyObject,
  expectInteger,
  expectNullableString,
  expectString,
} from '@/api/protocol'

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
  avatar: string
}

export async function login(input: LoginInput): Promise<AccessCredential> {
  return parseAccessCredential(
    await request<unknown>({ method: 'POST', url: '/api/v1/auth/login', data: input }),
  )
}

export async function refresh(): Promise<AccessCredential> {
  return refreshAccessCredential()
}

export async function logout(): Promise<void> {
  expectEmptyObject(
    await request<unknown>({ method: 'POST', url: '/api/v1/auth/logout' }),
    'logout result',
  )
}

export async function getCurrentUser(): Promise<CurrentUser> {
  return parseCurrentUser(await request<unknown>({ method: 'GET', url: '/api/v1/auth/me' }))
}

function parseCurrentUser(value: unknown): CurrentUser {
  const record = expectExactKeys(
    value,
    ['userId', 'username', 'email', 'phone', 'avatar'],
    'current user response',
  )
  return {
    userId: expectInteger(record.userId, 'current user.userId'),
    username: expectString(record.username, 'current user.username'),
    email: expectString(record.email, 'current user.email'),
    phone: expectNullableString(record.phone, 'current user.phone'),
    avatar: expectString(record.avatar, 'current user.avatar'),
  }
}

function parseAccessCredential(value: unknown): AccessCredential {
  const record = expectExactKeys(value, ['accessToken', 'expiresIn'], 'access credential response')
  return {
    accessToken: expectString(record.accessToken, 'access credential.accessToken'),
    expiresIn: expectInteger(record.expiresIn, 'access credential.expiresIn'),
  }
}
