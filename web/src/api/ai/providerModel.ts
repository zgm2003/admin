import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'

export interface ProviderModelActions {
  update: boolean
}

export interface ProviderModelPrice {
  id: number
  tierKey: string
  tierLabel: string
  priceCny: string
  status: number
  version: number
}

export interface ProviderModelOfficial {
  id: number
  modelKey: string
  displayName: string
  modelKind: number
  kindName: string
  inputCnyPerMillion: string
  outputCnyPerMillion: string
  cacheReadCnyPerMillion: string | null
  cacheWriteCnyPerMillion: string | null
}

export interface AiProviderModel {
  id: number
  providerId: number
  upstreamModel: string
  modelKey: string
  displayName: string
  modelKind: number
  kindName: string
  pricingMode: number
  pricingName: string
  priceUnit: number
  unitName: string
  source: number
  sourceName: string
  status: number
  statusName: string
  officialModel: ProviderModelOfficial | null
  prices: ProviderModelPrice[]
  lastSeenAt: string | null
  version: number
  actions: ProviderModelActions
  createdAt: string
  updatedAt: string
}

export interface ProviderModelOption {
  value: number
  label: string
}

export interface ProviderModelOfficialOption {
  id: number
  modelKey: string
  displayName: string
  modelKind: number
  kindName: string
}

export interface ProviderModelOptions {
  modelKinds: ProviderModelOption[]
  pricingModes: ProviderModelOption[]
  priceUnits: ProviderModelOption[]
  officialModels: ProviderModelOfficialOption[]
}

export interface ProviderModelQuery extends PageRequest {
  keyword?: string
  modelKind?: number
  status?: number
}

export interface ProviderModelPriceInput {
  tierKey: string
  tierLabel: string
  priceCny: string
}

export interface UpdateProviderModelInput {
  displayName: string
  modelKind: number
  pricingMode: number
  priceUnit: number
  officialModelId: number | null
  status: number
  prices: ProviderModelPriceInput[]
}

export interface ProviderModelSyncResult {
  total: number
  inserted: number
  updated: number
  deactivated: number
  unmatched: number
}

const base = (providerId: number) => `/api/admin/v1/ai/provider/${providerId}/models`

export function getProviderModels(
  providerId: number,
  query: ProviderModelQuery,
): Promise<PageResult<AiProviderModel>> {
  return request.get<PageResult<AiProviderModel>>(base(providerId), { params: query })
}

export function getProviderModelOptions(providerId: number): Promise<ProviderModelOptions> {
  return request.get<ProviderModelOptions>(`${base(providerId)}/options`)
}

export function syncProviderModels(providerId: number): Promise<ProviderModelSyncResult> {
  return request.post<ProviderModelSyncResult>(`${base(providerId)}/sync`)
}

export function updateProviderModel(
  providerId: number,
  modelId: number,
  input: UpdateProviderModelInput,
): Promise<Record<string, never>> {
  return request.put<Record<string, never>, UpdateProviderModelInput>(
    `${base(providerId)}/${modelId}`,
    input,
  )
}
