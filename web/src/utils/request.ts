import axios, {
  AxiosHeaders,
  type AxiosAdapter,
  type AxiosInstance,
  type AxiosRequestConfig,
  type InternalAxiosRequestConfig,
} from 'axios'

import type { AccessCredential } from '@/api/auth/login'
import { authPlatform } from '@/auth/platform'
import { readDeviceID } from '@/auth/device-id'
import { appI18n, readLocale } from '@/i18n'
import { pinia } from '@/store'
import { usePermissionStore } from '@/store/permission'
import { useAuthStore } from '@/store/auth'
import { ApiError, ProtocolError, type ApiResponse } from '@/types/http'

export { ApiError, ProtocolError } from '@/types/http'

const envelopeKeys = ['code', 'data', 'message']
const noBearerPaths = new Set(['/api/v1/auth/login', '/api/v1/auth/refresh'])
const noRefreshPaths = new Set([...noBearerPaths, '/api/v1/auth/logout'])

interface AuthRequestConfig extends InternalAxiosRequestConfig {
  authRetried?: boolean
}

interface RequestClientBundle {
  client: AxiosInstance
  refreshAccessCredential: () => Promise<AccessCredential>
}

export function unwrapEnvelope<T>(value: unknown): T {
  return unwrapSuccessEnvelope<T>(value)
}

function unwrapSuccessEnvelope<T>(value: unknown): T {
  const envelope = parseEnvelope(value)
  if (envelope.code !== 0) {
    throw new ApiError(envelope.code, envelope.message)
  }
  return envelope.data as T
}

function parseEnvelope(value: unknown): ApiResponse<unknown> {
  if (!isRecord(value)) {
    throw new ProtocolError('API response must be an object')
  }

  const keys = Object.keys(value).sort()
  if (
    keys.length !== envelopeKeys.length ||
    keys.some((key, index) => key !== envelopeKeys[index])
  ) {
    throw new ProtocolError('API response must contain exactly code, data, and message')
  }
  if (
    typeof value.code !== 'number' ||
    !Number.isInteger(value.code) ||
    typeof value.message !== 'string'
  ) {
    throw new ProtocolError('API response code or message has an invalid type')
  }

  return { code: value.code, data: value.data, message: value.message }
}

export function createRequestClient(
  baseURL: string,
  adapter?: AxiosAdapter,
  onUnauthorized: () => void = () => undefined,
): AxiosInstance {
  return buildRequestClient(baseURL, adapter, onUnauthorized).client
}

function buildRequestClient(
  baseURL: string,
  adapter: AxiosAdapter | undefined,
  onUnauthorized: () => void,
): RequestClientBundle {
  if (baseURL.trim() === '') {
    throw new ProtocolError('VITE_API_BASE_URL is required')
  }

  const axiosConfig: AxiosRequestConfig = { baseURL, withCredentials: true }
  if (adapter !== undefined) {
    axiosConfig.adapter = adapter
  }
  const rawClient = axios.create(axiosConfig)
  const client = axios.create(axiosConfig)
  const authStore = useAuthStore(pinia)
  let refreshPromise: Promise<AccessCredential> | null = null

  const coordinatedRefresh = (): Promise<AccessCredential> => {
    if (refreshPromise === null) {
      refreshPromise = performRefresh(rawClient, authStore, onUnauthorized)
      refreshPromise
        .finally(() => {
          refreshPromise = null
        })
        .catch(() => undefined)
    }
    return refreshPromise
  }

  rawClient.interceptors.request.use(applyClientHeaders)

  client.interceptors.request.use((config) => {
    applyClientHeaders(config)
    const path = requestPath(config.url, baseURL)
    if (authStore.accessToken !== '' && !noBearerPaths.has(path)) {
      config.headers = AxiosHeaders.from(config.headers)
      config.headers.set('Authorization', `Bearer ${authStore.accessToken}`)
    }
    return config
  })

  client.interceptors.response.use(
    (response) => {
      try {
        response.data = unwrapSuccessEnvelope(response.data)
        return response
      } catch (error: unknown) {
        return Promise.reject(error)
      }
    },
    async (error: unknown) => {
      const normalizedError = normalizeResponseError(error)
      if (
        !axios.isAxiosError(error) ||
        error.response?.status !== 401 ||
        error.config === undefined
      ) {
        return Promise.reject(normalizedError)
      }

      const originalConfig = error.config as AuthRequestConfig
      const path = requestPath(originalConfig.url, baseURL)
      if (originalConfig.authRetried === true || noRefreshPaths.has(path)) {
        return Promise.reject(normalizedError)
      }

      try {
        const credential = await coordinatedRefresh()
        const retryConfig: AuthRequestConfig = { ...originalConfig, authRetried: true }
        retryConfig.headers = AxiosHeaders.from(originalConfig.headers)
        retryConfig.headers.set('Authorization', `Bearer ${credential.accessToken}`)
        return client.request(retryConfig)
      } catch (refreshError: unknown) {
        return Promise.reject(refreshError)
      }
    },
  )

  return { client, refreshAccessCredential: coordinatedRefresh }
}

async function performRefresh(
  rawClient: AxiosInstance,
  authStore: ReturnType<typeof useAuthStore>,
  onUnauthorized: () => void,
): Promise<AccessCredential> {
  try {
    const response = await rawClient.post<unknown>('/api/v1/auth/refresh', undefined, {
      withCredentials: true,
    })
    const credential = unwrapSuccessEnvelope<unknown>(response.data)
    if (!isAccessCredential(credential)) {
      throw new ProtocolError('access credential response is invalid')
    }
    authStore.setCredential(credential)
    return credential
  } catch (error: unknown) {
    const normalizedError = normalizeResponseError(error)
    if (normalizedError instanceof ApiError && normalizedError.httpStatus === 401) {
      authStore.setAnonymous()
      onUnauthorized()
    } else {
      authStore.setError(errorMessage(normalizedError))
    }
    throw normalizedError
  }
}

function applyClientHeaders(config: InternalAxiosRequestConfig): InternalAxiosRequestConfig {
  config.headers = AxiosHeaders.from(config.headers)
  config.headers.set('Accept-Language', readLocale())
  config.headers.set('X-Auth-Platform', authPlatform)
  config.headers.set('X-Device-ID', readDeviceID())
  return config
}

function normalizeResponseError(error: unknown): unknown {
  if (!axios.isAxiosError(error) || error.response === undefined) {
    return error
  }
  try {
    const envelope = parseEnvelope(error.response.data)
    if (envelope.code === 0) {
      return new ProtocolError('HTTP error response must use a non-zero business code')
    }
    return new ApiError(envelope.code, envelope.message, error.response.status)
  } catch (protocolError: unknown) {
    return protocolError
  }
}

function requestPath(url: string | undefined, baseURL: string): string {
  if (url === undefined || url === '') {
    return ''
  }
  try {
    return new URL(url, baseURL).pathname
  } catch {
    return url.split('?', 1)[0] ?? ''
  }
}

function errorMessage(error: unknown): string {
  if (error instanceof ProtocolError) {
    return appI18n.global.t('request.protocolError')
  }
  if (error instanceof Error && error.message !== '') {
    return error.message
  }
  return appI18n.global.t('auth.login.bootstrapFailed')
}

function handleUnauthorized(): void {
  usePermissionStore(pinia).reset()
  const redirect = `${window.location.pathname}${window.location.search}${window.location.hash}`
  window.location.assign(`/login?redirect=${encodeURIComponent(redirect)}`)
}

const defaultBundle = buildRequestClient(
  import.meta.env.VITE_API_BASE_URL,
  undefined,
  handleUnauthorized,
)
const client = defaultBundle.client

export async function refreshAccessCredential(): Promise<AccessCredential> {
  return defaultBundle.refreshAccessCredential()
}

export async function request<T>(config: AxiosRequestConfig): Promise<T> {
  const response = await client.request<T>(config)
  return response.data
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function isAccessCredential(value: unknown): value is AccessCredential {
  if (!isRecord(value)) return false
  return (
    typeof value.accessToken === 'string' &&
    value.accessToken !== '' &&
    typeof value.expiresIn === 'number' &&
    Number.isInteger(value.expiresIn) &&
    value.expiresIn > 0
  )
}
