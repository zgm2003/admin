import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'

export type JsonValue =
  string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue }
export type JsonObject = { [key: string]: JsonValue }
export interface NumericOption {
  value: number
  label: string
}
export interface StringOption {
  value: string
  label: string
}
export interface Provider {
  id: string
  code: string
  name: string
  description: string
  status: number
  currentVersion: string
  apiKeyConfigured: boolean
  createdAt: string
  updatedAt: string
}
export interface ProviderConfiguration {
  provider: Provider
  baseUrl: string
  mediaHostAllowlist: string[]
  requestTimeoutMs: number
  headersConfigured: boolean
}
export interface ProviderUpdate {
  name: string
  description: string
  baseUrl: string
  apiKey: string
  headers: Record<string, string> | null
  mediaHostAllowlist: string[]
  requestTimeoutMs: number
}
export interface ProviderCreate extends ProviderUpdate {
  code: string
  status: number
}
export interface ProviderOptions {
  statuses: NumericOption[]
  textProtocols: NumericOption[]
  operations: StringOption[]
  defaults: { requestTimeoutMs: number; status: number }
  constraints: {
    requestTimeoutMs: { min: number; max: number }
    maxNameLength: number
    maxDescriptionLength: number
  }
}
export interface ProviderModel {
  id: string
  providerId: string
  upstreamModelId: string
  name: string
  officialModelId?: string | null
  operations: string[]
  limits: JsonObject
  status: number
  createdAt: string
  updatedAt: string
}
export interface CatalogSelection {
  upstreamModelId: string
  name: string
  officialModelId: string | null
  operations: string[]
  limits: JsonObject
}
export interface SaveProviderModel extends CatalogSelection {
  id: string
}
export interface CatalogModel {
  id: string
  ownedBy: string
  created: number
}
export interface CatalogPreview {
  providerId: string
  providerVersionId: string
  digest: string
  added: CatalogModel[]
  existing: CatalogModel[]
}
export interface OfficialCandidate {
  id: string
  vendorCode: string
  modelKey: string
  name: string
  status: number
}
export interface PriceRate {
  id: string
  operation: string
  usageKey: string
  selector: JsonObject
  unit: string
  unitQuantity: string
  sourceUnitPrice: string
  billingUnitPrice: string | null
}
export interface PriceVersion {
  id: string
  providerModelId: string
  version: string
  status: number
  effectiveAt: string | null
  currency: string
  sourceCurrency: string
  sourceDigest: string
  snapshotDigest: string
  syncedAt: string
  rates: PriceRate[]
}
export interface PriceUpdate {
  versionId: string
  effectiveAt?: string
  rates: { rateId: string; billingUnitPrice: string }[]
}
const base = '/api/admin/v1/ai/provider'
type Empty = Record<string, never>
export function getProviders(query: PageRequest): Promise<PageResult<Provider>> {
  return request.get<PageResult<Provider>>(base, { params: query })
}
export function getProvider(id: string): Promise<ProviderConfiguration> {
  return request.get<ProviderConfiguration>(`${base}/${id}`)
}
export function getProviderOptions(): Promise<ProviderOptions> {
  return request.get<ProviderOptions>(`${base}/options`)
}
export function createProvider(input: ProviderCreate): Promise<Provider> {
  const payload = {
    code: input.code,
    status: input.status,
    name: input.name,
    description: input.description,
    baseUrl: input.baseUrl,
    apiKey: input.apiKey,
    mediaHostAllowlist: input.mediaHostAllowlist,
    requestTimeoutMs: input.requestTimeoutMs,
    ...(input.headers === null ? {} : { headers: input.headers }),
  }
  return request.post<Provider, typeof payload>(base, payload)
}
export function updateProvider(id: string, input: ProviderUpdate): Promise<Provider> {
  const payload = {
    name: input.name,
    description: input.description,
    baseUrl: input.baseUrl,
    apiKey: input.apiKey,
    mediaHostAllowlist: input.mediaHostAllowlist,
    requestTimeoutMs: input.requestTimeoutMs,
    ...(input.headers === null ? {} : { headers: input.headers }),
  }
  return request.put<Provider, typeof payload>(`${base}/${id}`, payload)
}
export function updateProviderStatus(id: string, status: number): Promise<Empty> {
  return request.patch<Empty, { status: number }>(`${base}/${id}/status`, { status })
}
export function deleteProvider(id: string): Promise<Empty> {
  return request.delete<Empty>(`${base}/${id}`)
}
export function testProvider(id: string): Promise<{ connected: boolean; modelCount: number }> {
  return request.post<{ connected: boolean; modelCount: number }>(`${base}/${id}/test`)
}
export function getProviderCandidates(
  id: string | null,
  query: PageRequest & { keyword?: string },
  action: 'update' | 'sync' = 'update',
): Promise<PageResult<OfficialCandidate>> {
  return request.get<PageResult<OfficialCandidate>>(
    `${base}${id === null ? '/candidate/create' : `/${id}/candidate/${action}`}`,
    { params: query },
  )
}
export function getProviderModels(
  id: string,
  query: PageRequest,
): Promise<PageResult<ProviderModel>> {
  return request.get<PageResult<ProviderModel>>(`${base}/${id}/model`, { params: query })
}
export function saveProviderModels(id: string, models: SaveProviderModel[]): Promise<Empty> {
  return request.put<Empty, { models: SaveProviderModel[] }>(`${base}/${id}/model`, {
    models: models.map((m) => ({
      id: m.id,
      upstreamModelId: m.upstreamModelId,
      name: m.name,
      officialModelId: m.officialModelId,
      operations: m.operations,
      limits: m.limits,
    })),
  })
}
export function updateProviderModelStatus(
  id: string,
  modelId: string,
  status: number,
): Promise<Empty> {
  return request.patch<Empty, { status: number }>(`${base}/${id}/model/${modelId}/status`, {
    status,
  })
}
export function previewProviderCatalog(id: string): Promise<CatalogPreview> {
  return request.post<CatalogPreview>(`${base}/${id}/model/sync-preview`)
}
export function confirmProviderCatalog(
  id: string,
  digest: string,
  models: CatalogSelection[],
): Promise<Empty> {
  return request.post<Empty, { digest: string; models: CatalogSelection[] }>(
    `${base}/${id}/model/sync`,
    {
      digest,
      models: models.map((m) => ({
        upstreamModelId: m.upstreamModelId,
        name: m.name,
        officialModelId: m.officialModelId,
        operations: m.operations,
        limits: m.limits,
      })),
    },
  )
}
export function getProviderPrices(
  id: string,
  modelId: string,
  query: PageRequest,
): Promise<PageResult<PriceVersion>> {
  return request.get<PageResult<PriceVersion>>(`${base}/${id}/model/${modelId}/price`, {
    params: query,
  })
}
export function syncProviderPrice(id: string, modelId: string): Promise<PriceVersion> {
  return request.post<PriceVersion>(`${base}/${id}/model/${modelId}/price/sync`)
}
export function previewProviderPrice(
  id: string,
  modelId: string,
  input: PriceUpdate,
): Promise<PriceVersion> {
  return request.put<PriceVersion, PriceUpdate>(`${base}/${id}/model/${modelId}/price`, {
    versionId: input.versionId,
    ...(input.effectiveAt === undefined ? {} : { effectiveAt: input.effectiveAt }),
    rates: input.rates.map((r) => ({ rateId: r.rateId, billingUnitPrice: r.billingUnitPrice })),
  })
}
export function publishProviderPrice(
  id: string,
  modelId: string,
  versionId: string,
  snapshotDigest: string,
): Promise<Empty> {
  return request.post<Empty, { snapshotDigest: string }>(
    `${base}/${id}/model/${modelId}/price/${versionId}/publish`,
    { snapshotDigest },
  )
}
