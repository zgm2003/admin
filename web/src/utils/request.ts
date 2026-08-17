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

  constructor(code: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
  }
}

export function unwrapEnvelope<T>(value: unknown): T {
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

  const envelope = value as unknown as ApiResponse<T>
  if (envelope.code !== 0) {
    throw new ApiError(envelope.code, envelope.message)
  }
  return envelope.data
}

export function createRequestClient(baseURL: string): AxiosInstance {
  if (baseURL.trim() === '') {
    throw new ProtocolError('VITE_API_BASE_URL is required')
  }

  const client = axios.create({ baseURL, withCredentials: true })
  client.interceptors.response.use((response) => {
    response.data = unwrapEnvelope(response.data)
    return response
  })
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

