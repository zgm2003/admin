import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'
import type { NumericOption, StringOption, PriceRate } from './provider'

export interface OfficialModel {
  id: string
  vendorCode: string
  modelKey: string
  name: string
  operations: string[]
  sourceUrls: string[]
  status: number
  createdAt: string
  updatedAt: string
}
export interface OfficialModelUpdate {
  name: string
  operations: string[]
  sourceUrls: string[]
}
export interface OfficialModelCreate extends OfficialModelUpdate {
  vendorCode: string
  modelKey: string
  status: number
}
export interface OfficialModelOptions {
  statuses: NumericOption[]
  operations: StringOption[]
  defaults: { status: number }
  constraints: { maxNameLength: number; maxModelKeyLength: number; maxSourceUrls: number }
}
export interface OfficialModelPrice {
  providerModelId: string
  providerId: string
  providerName: string
  upstreamModelId: string
  priceVersionId: string
  version: string
  effectiveAt: string
  currency: string
  sourceCurrency: string
  rates: PriceRate[]
}
const base = '/api/admin/v1/ai/officialmodel'
export function getOfficialModels(query: PageRequest): Promise<PageResult<OfficialModel>> {
  return request.get<PageResult<OfficialModel>>(base, { params: query })
}
export function getOfficialModel(id: string): Promise<OfficialModel> {
  return request.get<OfficialModel>(`${base}/${id}`)
}
export function getOfficialModelOptions(): Promise<OfficialModelOptions> {
  return request.get<OfficialModelOptions>(`${base}/options`)
}
export function createOfficialModel(input: OfficialModelCreate): Promise<OfficialModel> {
  return request.post<OfficialModel, OfficialModelCreate>(base, input)
}
export function updateOfficialModel(
  id: string,
  input: OfficialModelUpdate,
): Promise<OfficialModel> {
  return request.put<OfficialModel, OfficialModelUpdate>(`${base}/${id}`, {
    name: input.name,
    operations: input.operations,
    sourceUrls: input.sourceUrls,
  })
}
export function updateOfficialModelStatus(
  id: string,
  status: number,
): Promise<Record<string, never>> {
  return request.patch<Record<string, never>, { status: number }>(`${base}/${id}/status`, {
    status,
  })
}
export function getOfficialModelPrices(
  id: string,
  query: PageRequest,
): Promise<PageResult<OfficialModelPrice>> {
  return request.get<PageResult<OfficialModelPrice>>(`${base}/${id}/price`, { params: query })
}
