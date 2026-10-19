import { toast } from 'sonner'
import { ApiError, HttpError, ProtocolError } from '@/types/http'

export { ApiError, HttpError, ProtocolError } from '@/types/http'

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
  post<T, D = unknown>(path: string, body?: D, options?: RequestOptions): Promise<T>
  put<T, D = unknown>(path: string, body?: D, options?: RequestOptions): Promise<T>
  setAccessToken(token: string): void
  getAccessToken(): string
  clearAccessToken(): void
}

export interface RequestOptions {
  signal?: AbortSignal
  headers?: HeadersInit
  successMessage?: string | true
}

export interface ApiEnvelope<T> {
  code: number
  data: T
  message: string
}

const noBearerPaths = new Set([
  '/api/canvas/v1/auth/login-config',
  '/api/canvas/v1/auth/captcha',
  '/api/canvas/v1/auth/send-code',
  '/api/canvas/v1/auth/login',
  '/api/canvas/v1/auth/password/forgot',
  '/api/canvas/v1/auth/password/reset',
  '/api/canvas/v1/auth/refresh',
])

const noRefreshPaths = new Set([...noBearerPaths, '/api/canvas/v1/auth/logout'])

export function createRequestClient(options: RequestClientOptions): RequestClient {
  const fetchImpl = options.fetchImpl ?? fetch
  const baseUrl = options.baseUrl.trim()
  if (options.platform.trim() === '') throw new ProtocolError('authentication platform is required')
  if (options.deviceId.trim() === '') throw new ProtocolError('device ID is required')

  let accessToken = options.initialAccessToken ?? ''
  let refreshPromise: Promise<string> | null = null
  const reportedErrors = new WeakSet<Error>()

  const send = async <T>(path: string, method: 'GET' | 'POST' | 'PUT', body: unknown, requestOptions: RequestOptions | undefined, retry: boolean): Promise<T> => {
    const url = (baseUrl === '' ? new URL(path, window.location.origin) : new URL(path, baseUrl)).toString()
    const requestPath = new URL(url).pathname
    const headers = new Headers(requestOptions?.headers)
    headers.set('Accept', 'application/json')
    headers.set('Accept-Language', typeof document === 'undefined' ? 'zh-CN' : document.documentElement.lang || 'zh-CN')
    headers.set('X-Auth-Platform', options.platform)
    headers.set('X-Device-ID', options.deviceId)
    if (accessToken !== '' && !noBearerPaths.has(requestPath)) headers.set('Authorization', `Bearer ${accessToken}`)
    const init: RequestInit = { method, headers, credentials: 'include', signal: requestOptions?.signal }
    if (body !== undefined) {
      headers.set('Content-Type', 'application/json')
      init.body = JSON.stringify(body)
    }

    let response: Response
    try {
      response = await fetchImpl(url, init)
    } catch (error) {
      if (isCanceled(error, requestOptions?.signal)) throw error
      throw new Error('网络连接失败，请检查网络后重试', { cause: error })
    }

    if (response.status === 401 && !retry && !noRefreshPaths.has(requestPath)) {
      const token = await coordinatedRefresh()
      accessToken = token
      requestOptions?.signal?.throwIfAborted()
      return send<T>(path, method, body, requestOptions, true)
    }
    if (response.status === 401 && (requestPath === '/api/canvas/v1/auth/refresh' || !noRefreshPaths.has(requestPath))) {
      accessToken = ''
      options.onUnauthorized?.()
    }

    const envelope = await parseEnvelope(response)
    requestOptions?.signal?.throwIfAborted()
    if (envelope.code !== 0) throw new ApiError(envelope.code, envelope.message.trim() || '请求失败，请稍后重试', response.status)
    if (!response.ok) throw httpError(response.status)
    if ((requestPath === '/api/canvas/v1/auth/login' || requestPath === '/api/canvas/v1/auth/refresh') && !isAccessCredential(envelope.data)) {
      throw new ProtocolError('登录凭据响应异常，请重新登录')
    }
    if (requestOptions?.successMessage) {
      const message = requestOptions.successMessage === true ? (envelope.message.trim() && envelope.message !== 'ok' ? envelope.message : '操作成功') : requestOptions.successMessage
      toast.success(message)
    }
    return envelope.data as T
  }

  const coordinatedRefresh = (): Promise<string> => {
    if (refreshPromise === null) {
      refreshPromise = send<{ accessToken: string }>('/api/canvas/v1/auth/refresh', 'POST', undefined, undefined, true).then((credential) => credential.accessToken)
      refreshPromise.finally(() => { refreshPromise = null }).catch(() => undefined)
    }
    return refreshPromise
  }

  const execute = async <T>(path: string, method: 'GET' | 'POST' | 'PUT', body: unknown, requestOptions?: RequestOptions): Promise<T> => {
    try {
      return await send<T>(path, method, body, requestOptions, false)
    } catch (cause) {
      if (isCanceled(cause, requestOptions?.signal)) throw cause
      const error = cause instanceof Error ? cause : new Error('请求失败，请稍后重试')
      const requestPath = new URL(path, baseUrl || window.location.origin).pathname
      const inlineLoginError = requestPath === '/api/canvas/v1/auth/login' && error instanceof ApiError && error.code === 10002
      const anonymousProbe = requestPath === '/api/canvas/v1/auth/refresh' && (error instanceof ApiError || error instanceof HttpError) && error.httpStatus === 401
      if (!inlineLoginError && !anonymousProbe && !reportedErrors.has(error)) {
        reportedErrors.add(error)
        toast.error(error.message)
        options.onError?.(error)
      }
      throw error
    }
  }

  return {
    get: <T>(path: string, requestOptions?: RequestOptions) => execute<T>(path, 'GET', undefined, requestOptions),
    post: <T>(path: string, body?: unknown, requestOptions?: RequestOptions) => execute<T>(path, 'POST', body, requestOptions),
    put: <T>(path: string, body?: unknown, requestOptions?: RequestOptions) => execute<T>(path, 'PUT', body, requestOptions),
    setAccessToken: (token) => { accessToken = token },
    getAccessToken: () => accessToken,
    clearAccessToken: () => { accessToken = '' },
  }
}

async function parseEnvelope(response: Response): Promise<ApiEnvelope<unknown>> {
  let value: unknown
  try {
    value = await response.json()
  } catch (cause) {
    if (isCanceled(cause)) throw cause
    if (!response.ok) throw httpError(response.status)
    throw new ProtocolError('服务响应格式异常，请稍后重试')
  }
  if (!isRecord(value) || Object.keys(value).sort().join(',') !== 'code,data,message' || typeof value.code !== 'number' || !Number.isInteger(value.code) || typeof value.message !== 'string') {
    if (!response.ok) throw httpError(response.status)
    throw new ProtocolError('服务响应格式异常，请稍后重试')
  }
  return { code: value.code, data: value.data, message: value.message }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function isCanceled(error: unknown, signal?: AbortSignal): boolean {
  return signal?.aborted === true || isRecord(error) && error.name === 'AbortError'
}

function httpError(status: number): HttpError {
  const messages: Record<number, string> = { 401: '登录状态已失效，请重新登录', 403: '没有执行此操作的权限', 404: '请求的接口不存在', 408: '请求超时，请稍后重试', 429: '请求过于频繁，请稍后重试' }
  return new HttpError(status, messages[status] ?? (status >= 500 ? '服务暂时不可用，请稍后重试' : `请求失败（HTTP ${status}），请稍后重试`))
}

function isAccessCredential(value: unknown): value is { accessToken: string; expiresIn: number; isNewUser: boolean; passwordSetRequired: boolean } {
  return isRecord(value) && typeof value.accessToken === 'string' && value.accessToken.trim() !== '' && typeof value.expiresIn === 'number' && Number.isInteger(value.expiresIn) && value.expiresIn > 0 && typeof value.isNewUser === 'boolean' && typeof value.passwordSetRequired === 'boolean'
}
