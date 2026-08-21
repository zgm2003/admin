import { ProtocolError } from '../types/http'
import { refreshAccessCredential, request } from '../utils/request'
import {
  parseCredential,
  parseAuthPolicy,
  parseCurrentUser,
  parseRegisteredUser,
  type AccessCredential,
  type AuthPolicy,
  type CurrentUser,
  type LoginInput,
  type RegisteredUser,
  type RegisterInput,
} from './auth.contract'

export async function register(input: RegisterInput): Promise<RegisteredUser> {
  const data = await request<unknown>({ method: 'POST', url: '/api/v1/auth/register', data: input })
  return parseRegisteredUser(data)
}

export async function login(input: LoginInput): Promise<AccessCredential> {
  const data = await request<unknown>({ method: 'POST', url: '/api/v1/auth/login', data: input })
  return parseCredential(data)
}

export async function refresh(): Promise<AccessCredential> {
  return refreshAccessCredential()
}

export async function logout(): Promise<void> {
  const data = await request<unknown>({ method: 'POST', url: '/api/v1/auth/logout' })
  if (typeof data !== 'object' || data === null || Array.isArray(data) || Object.keys(data).length !== 0) {
    throw new ProtocolError('logout response data must be an empty object')
  }
}

export async function getCurrentUser(): Promise<CurrentUser> {
  const data = await request<unknown>({ method: 'GET', url: '/api/v1/auth/me' })
  return parseCurrentUser(data)
}

export async function getAuthPolicy(): Promise<AuthPolicy> {
  const data = await request<unknown>({ method: 'GET', url: '/api/v1/auth/policy' })
  return parseAuthPolicy(data)
}
