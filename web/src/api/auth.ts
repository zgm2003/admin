import { authPlatform } from '../auth/platform'
import type { YesNo } from '../enums/yes-no'
import { refreshAccessCredential, request } from '../utils/request'

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

export interface AuthPolicy {
  code: typeof authPlatform
  name: string
  allowRegister: YesNo
}

export async function register(input: RegisterInput): Promise<RegisteredUser> {
  return request<RegisteredUser>({ method: 'POST', url: '/api/v1/auth/register', data: input })
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
  return request<CurrentUser>({ method: 'GET', url: '/api/v1/auth/me' })
}

export async function getAuthPolicy(): Promise<AuthPolicy> {
  return request<AuthPolicy>({ method: 'GET', url: '/api/v1/auth/policy' })
}
