export interface RequestClientOptions {
  baseUrl: string
  platform: string
  deviceId: string
  fetchImpl?: typeof fetch
  initialAccessToken?: string
  onUnauthorized?: () => void
  onError?: (error: unknown) => void
}

export interface RequestClient {
  get<T>(path: string, options?: RequestOptions): Promise<T>
  post<T>(path: string, body?: unknown, options?: RequestOptions): Promise<T>
  setAccessToken(token: string): void
  getAccessToken(): string
  clearAccessToken(): void
}

export interface RequestOptions {
  signal?: AbortSignal
  headers?: HeadersInit
}

export interface ApiEnvelope<T> {
  code: number
  data: T
  message: string
}

import { ApiError, ProtocolError } from '@/types/http'

export { ApiError, ProtocolError } from '@/types/http'

const noBearerPaths = new Set([
  '/api/v1/auth/login-config',
  '/api/v1/auth/captcha',
  '/api/v1/auth/send-code',
  '/api/v1/auth/login',
  '/api/v1/auth/password/forgot',
  '/api/v1/auth/password/reset',
  '/api/v1/auth/refresh',
])

const noRefreshPaths = new Set([
  ...noBearerPaths,
  '/api/v1/auth/logout',
])

export function createRequestClient(options: RequestClientOptions): RequestClient {
  const fetchImpl = options.fetchImpl ?? fetch
  const baseUrl = options.baseUrl.trim()
  if (options.platform.trim() === '') throw new ProtocolError('authentication platform is required')
  if (options.deviceId.trim() === '') throw new ProtocolError('device ID is required')

  let accessToken = options.initialAccessToken ?? ''
  let refreshPromise: Promise<string> | null = null

  const send = async <T>(path: string, method: 'GET' | 'POST', body: unknown, requestOptions: RequestOptions | undefined, retry: boolean): Promise<T> => {
    const url = (baseUrl === '' ? new URL(path, window.location.origin) : new URL(path, baseUrl)).toString()
    const headers = new Headers(requestOptions?.headers)
    headers.set('Accept', 'application/json')
    headers.set('Accept-Language', typeof document === 'undefined' ? 'zh-CN' : document.documentElement.lang || 'zh-CN')
    headers.set('X-Auth-Platform', options.platform)
    headers.set('X-Device-ID', options.deviceId)
    if (accessToken !== '' && !noBearerPaths.has(new URL(url).pathname)) {
      headers.set('Authorization', `Bearer ${accessToken}`)
    }
    const init: RequestInit = {
      method,
      headers,
      credentials: 'include',
      signal: requestOptions?.signal,
    }
    if (body !== undefined) {
      headers.set('Content-Type', 'application/json')
      init.body = JSON.stringify(body)
    }

    let response: Response
    try {
      response = await fetchImpl(url, init)
    } catch (error) {
      options.onError?.(error)
      throw error
    }

    let envelope: ApiEnvelope<unknown>
    try {
      envelope = await parseEnvelope(response)
    } catch (error) {
      options.onError?.(error)
      throw error
    }
    if (envelope.code === 0 && response.ok) return envelope.data as T

    const error = new ApiError(envelope.code, envelope.message, response.status)
    const requestPath = new URL(url).pathname
    if (response.status === 401 && !retry && !noRefreshPaths.has(requestPath)) {
      try {
        const token = await coordinatedRefresh()
        accessToken = token
        return await send<T>(path, method, body, requestOptions, true)
      } catch (refreshError) {
        options.onError?.(refreshError)
        throw refreshError
      }
    }
    if (response.status === 401 && requestPath === '/api/v1/auth/refresh') {
      accessToken = ''
      options.onUnauthorized?.()
    }
    options.onError?.(error)
    throw error
  }

  const coordinatedRefresh = (): Promise<string> => {
    if (refreshPromise === null) {
      refreshPromise = send<{ accessToken: string }>(
        '/api/v1/auth/refresh',
        'POST',
        undefined,
        undefined,
        true,
      ).then((credential) => {
        if (typeof credential.accessToken !== 'string' || credential.accessToken === '') {
          throw new ProtocolError('refresh response access token is invalid')
        }
        return credential.accessToken
      })
      refreshPromise.finally(() => {
        refreshPromise = null
      }).catch(() => undefined)
    }
    return refreshPromise
  }

  return {
    get: <T>(path: string, requestOptions?: RequestOptions) => send<T>(path, 'GET', undefined, requestOptions, false),
    post: <T>(path: string, body?: unknown, requestOptions?: RequestOptions) => send<T>(path, 'POST', body, requestOptions, false),
    setAccessToken: (token) => { accessToken = token },
    getAccessToken: () => accessToken,
    clearAccessToken: () => { accessToken = '' },
  }
}

async function parseEnvelope(response: Response): Promise<ApiEnvelope<unknown>> {
  let value: unknown
  try {
    value = await response.json()
  } catch {
    throw new ProtocolError('API response must be JSON')
  }
  if (!isRecord(value)) throw new ProtocolError('API response must be an object')
  const keys = Object.keys(value).sort()
  if (keys.join(',') !== 'code,data,message') {
    throw new ProtocolError('API response must contain exactly code, data, and message')
  }
  if (typeof value.code !== 'number' || !Number.isInteger(value.code) || typeof value.message !== 'string') {
    throw new ProtocolError('API response code or message has an invalid type')
  }
  return { code: value.code, data: value.data, message: value.message }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
