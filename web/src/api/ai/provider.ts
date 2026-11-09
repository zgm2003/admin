import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'

export interface ProviderActions {
  update: boolean
  status: boolean
  delete: boolean
}

export interface AiProvider {
  id: number
  name: string
  vendor: number
  vendorName: string
  protocol: number
  protocolName: string
  baseUrl: string
  timeoutMs: number
  sortOrder: number
  status: number
  statusName: string
  apiKeyConfigured: boolean
  actions: ProviderActions
  createdAt: string
  updatedAt: string
}

export interface ProviderQuery extends PageRequest {
  keyword?: string
  vendor?: number
  status?: number
}

export interface ProviderOption {
  value: number
  label: string
}

export interface ProviderOptions {
  vendors: ProviderOption[]
  protocols: ProviderOption[]
  statuses: ProviderOption[]
}

export interface CreateProviderInput {
  name: string
  vendor: number
  protocol: number
  baseUrl: string
  apiKey: string
  timeoutMs: number
  sortOrder: number
  status: number
}

export interface UpdateProviderInput {
  name: string
  vendor: number
  protocol: number
  baseUrl: string
  apiKey?: string
  timeoutMs: number
  sortOrder: number
  status: number
}

const base = '/api/admin/v1/ai/provider'

export function getProviders(query: ProviderQuery): Promise<PageResult<AiProvider>> {
  return request.get<PageResult<AiProvider>>(base, { params: query })
}

export function getProvider(id: number): Promise<AiProvider> {
  return request.get<AiProvider>(`${base}/${id}`)
}

export function getProviderOptions(): Promise<ProviderOptions> {
  return request.get<ProviderOptions>(`${base}/options`)
}

export function createProvider(input: CreateProviderInput): Promise<{ id: number }> {
  return request.post<{ id: number }, CreateProviderInput>(base, input)
}

export function updateProvider(
  id: number,
  input: UpdateProviderInput,
): Promise<Record<string, never>> {
  return request.put<Record<string, never>, UpdateProviderInput>(`${base}/${id}`, input)
}

export function updateProviderStatus(id: number, status: number): Promise<Record<string, never>> {
  return request.patch<Record<string, never>, { status: number }>(`${base}/${id}/status`, {
    status,
  })
}

export function deleteProvider(id: number): Promise<Record<string, never>> {
  return request.delete<Record<string, never>>(`${base}/${id}`)
}
