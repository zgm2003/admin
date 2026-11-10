import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'

export interface OfficialModelActions {
  update: boolean
  reset: boolean
}

export interface OfficialVideoPrice {
  tierKey: string
  tierLabel: string
  priceCnyPerSecond: string | null
  unit: string
}

export interface OfficialModel {
  id: number
  vendor: number
  vendorName: string
  modelKey: string
  displayName: string
  inputUsdPerMillion: string
  outputUsdPerMillion: string
  inputCnyPerMillion: string
  outputCnyPerMillion: string
  modelKind: number
  cacheReadCnyPerMillion: string | null
  cacheWriteCnyPerMillion: string | null
  cacheReadUsdPerMillion: string | null
  cacheWriteUsdPerMillion: string | null
  videoPrices: OfficialVideoPrice[]
  catalogVersion: string
  sourceUrl: string
  verifiedAt: string
  version: number
  actions: OfficialModelActions
  createdAt: string
  updatedAt: string
}

export interface OfficialModelQuery extends PageRequest {
  keyword?: string
  vendor?: number
}

export interface VendorOption {
  value: number
  label: string
}

export interface OfficialModelOptions {
  vendors: VendorOption[]
}

export interface UpdateOfficialModelPriceInput {
  expectedVersion: number
  inputCnyPerMillion: string
  outputCnyPerMillion: string
  cacheReadCnyPerMillion?: string
  cacheWriteCnyPerMillion?: string
}

export interface OfficialModelSyncResult {
  inserted: number
  updated: number
  unchanged: number
}

const base = '/api/admin/v1/ai/official-model'

export function getOfficialModels(query: OfficialModelQuery): Promise<PageResult<OfficialModel>> {
  return request.get<PageResult<OfficialModel>>(base, { params: query })
}

export function getOfficialModelOptions(): Promise<OfficialModelOptions> {
  return request.get<OfficialModelOptions>(`${base}/options`)
}

export function updateOfficialModelPrice(
  id: number,
  input: UpdateOfficialModelPriceInput,
): Promise<Record<string, never>> {
  return request.put<Record<string, never>, UpdateOfficialModelPriceInput>(
    `${base}/${id}/price`,
    input,
  )
}

export function resetOfficialModelPrice(
  id: number,
  expectedVersion: number,
): Promise<Record<string, never>> {
  return request.post<Record<string, never>, { expectedVersion: number }>(
    `${base}/${id}/reset-price`,
    { expectedVersion },
  )
}

export function syncOfficialModelCatalog(): Promise<OfficialModelSyncResult> {
  return request.post<OfficialModelSyncResult>(`${base}/sync`)
}
