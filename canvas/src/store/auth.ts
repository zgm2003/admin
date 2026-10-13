import type { AccessCredential, AuthApi, CurrentUser, LoginConfig, LoginInput } from '@/api/auth/login'
import { createAuthApi } from '@/api/auth/login'
import type { RequestClient } from '@/utils/request'

export type AuthStatus = 'unknown' | 'loading' | 'anonymous' | 'authenticated' | 'error'

export interface AuthState {
  status: AuthStatus
  accessToken: string
  user: CurrentUser | null
  loginConfig: LoginConfig | null
  error: string
}

type Listener = () => void

export class AuthStore {
  readonly api: AuthApi
  state: AuthState = {
    status: 'unknown',
    accessToken: '',
    user: null,
    loginConfig: null,
    error: '',
  }
  private readonly listeners = new Set<Listener>()
  private readonly client: RequestClient
  private bootstrapPromise: Promise<AuthStatus> | null = null
  private loginConfigPromise: Promise<LoginConfig> | null = null

  constructor(client: RequestClient) {
    this.client = client
    this.api = createAuthApi(client)
  }

  subscribe(listener: Listener): () => void {
    this.listeners.add(listener)
    return () => this.listeners.delete(listener)
  }

  getSnapshot = (): AuthState => this.state

  async loadLoginConfig(): Promise<LoginConfig> {
    if (this.loginConfigPromise !== null) return this.loginConfigPromise
    const pending = this.api.getLoginConfig().then((config) => {
      this.update({ loginConfig: config })
      return config
    })
    this.loginConfigPromise = pending
    pending.finally(() => {
      if (this.loginConfigPromise === pending) this.loginConfigPromise = null
    }).catch(() => undefined)
    return pending
  }

  async bootstrap(): Promise<AuthStatus> {
    if (this.bootstrapPromise !== null) return this.bootstrapPromise
    const pending = this.bootstrapOnce()
    this.bootstrapPromise = pending
    pending.finally(() => {
      if (this.bootstrapPromise === pending) this.bootstrapPromise = null
    }).catch(() => undefined)
    return pending
  }

  private async bootstrapOnce(): Promise<AuthStatus> {
    this.update({ status: 'loading', error: '' })
    try {
      if (this.state.loginConfig === null) await this.loadLoginConfig()
      const credential = await this.api.refresh()
      this.setCredential(credential)
      const user = await this.api.getCurrentUser()
      this.update({ status: 'authenticated', user })
      return 'authenticated'
    } catch (error) {
      if (isUnauthorized(error)) {
        this.client.clearAccessToken()
        this.update({ status: 'anonymous', accessToken: '', user: null, error: '' })
        return 'anonymous'
      }
      this.client.clearAccessToken()
      this.update({ accessToken: '' })
      this.update({ status: 'error', error: errorMessage(error) })
      throw error
    }
  }

  async login(input: LoginInput): Promise<CurrentUser> {
    this.update({ status: 'loading', error: '' })
    try {
      const credential = await this.api.login(input)
      this.setCredential(credential)
      const user = await this.api.getCurrentUser()
      this.update({ status: 'authenticated', user })
      return user
    } catch (error) {
      this.client.clearAccessToken()
      this.update({ accessToken: '' })
      this.update({ status: 'error', error: errorMessage(error) })
      throw error
    }
  }

  async logout(): Promise<void> {
    try {
      await this.api.logout()
    } finally {
      this.client.clearAccessToken()
      this.update({ status: 'anonymous', accessToken: '', user: null, error: '' })
    }
  }

  markAnonymous(): void {
    this.client.clearAccessToken()
    this.update({ status: 'anonymous', accessToken: '', user: null, error: '' })
  }

  setAuthenticated(credential: AccessCredential, user: CurrentUser): void {
    this.setCredential(credential)
    this.update({ status: 'authenticated', user, error: '' })
  }

  async reloadUser(): Promise<void> {
    const token = this.client.getAccessToken()
    const user = await this.api.getCurrentUser()
    if (this.state.status === 'authenticated' && this.client.getAccessToken() === token) this.update({ user })
  }

  private setCredential(credential: AccessCredential): void {
    if (credential.accessToken.trim() === '') throw new Error('access token is empty')
    this.client.setAccessToken(credential.accessToken)
    this.update({ accessToken: credential.accessToken })
  }

  private update(patch: Partial<AuthState>): void {
    this.state = { ...this.state, ...patch }
    this.listeners.forEach((listener) => listener())
  }
}

function isUnauthorized(error: unknown): boolean {
  return typeof error === 'object' && error !== null && 'code' in error && error.code === 10002
}

function errorMessage(error: unknown): string {
  return error instanceof Error && error.message !== '' ? error.message : '请求失败，请稍后重试'
}

export function createAuthStore(client: RequestClient): AuthStore {
  return new AuthStore(client)
}
