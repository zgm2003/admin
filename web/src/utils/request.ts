import axios, { type AxiosInstance, type AxiosRequestConfig } from 'axios'

import type { ApiResponse } from '../types/http'

const envelopeKeys = ['code', 'data', 'message']

export class ProtocolError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'ProtocolError'
  }
}

export class ApiError extends Error {
  readonly code: number
  readonly httpStatus?: number

  constructor(code: number, message: string, httpStatus?: number) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.httpStatus = httpStatus
  }
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
  if (keys.length !== envelopeKeys.length || keys.some((key, index) => key !== envelopeKeys[index])) {
    throw new ProtocolError('API response must contain exactly code, data, and message')
  }
  if (!Number.isInteger(value.code) || typeof value.message !== 'string') {
    throw new ProtocolError('API response code or message has an invalid type')
  }

  return value as unknown as ApiResponse<unknown>
}

export function createRequestClient(baseURL: string): AxiosInstance {
  if (baseURL.trim() === '') {
    throw new ProtocolError('VITE_API_BASE_URL is required')
  }

  const client = axios.create({ baseURL, withCredentials: true })
  client.interceptors.response.use(
    (response) => {
      response.data = unwrapSuccessEnvelope(response.data)
      return response
    },
    (error: unknown) => {
      if (!axios.isAxiosError(error) || !error.response) {
        return Promise.reject(error)
      }

      const envelope = parseEnvelope(error.response.data)
      if (envelope.code === 0) {
        return Promise.reject(new ProtocolError('HTTP error response must use a non-zero business code'))
      }
      return Promise.reject(new ApiError(envelope.code, envelope.message, error.response.status))
    },
  )
  return client
}

const client = createRequestClient(import.meta.env.VITE_API_BASE_URL)

export async function request<T>(config: AxiosRequestConfig): Promise<T> {
  const response = await client.request<T>(config)
  return response.data
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
