import { request } from '@/utils/request'
import type { BackendOption } from '@/types/option'
import type { CursorInput, CursorPage, SequencePage } from './chat'
export interface RunItem {
  id: string
  agentId: string
  conversationId: string | null
  operation: string
  executionStatus: number
  usageStatus: number
  billingStatus: number
  transferStatus: number
  estimatedUnits: string
  acceptedAt: string
  finishedAt: string | null
  errorCode: string | null
  errorMessage: string | null
  stateVersion: number
  nextEventSequence: number
  canCancel: boolean
}
export interface RunDetail extends RunItem {
  input: Record<string, unknown>
  result: unknown
  configuration?: Record<string, unknown>
  userId?: string
  platformId?: string
  providerModelId?: string
  priceVersionId?: string
}
export interface RunEvent {
  id: string
  runId: string
  sequence: number
  eventType: string
  payload: unknown
  createdAt: string
}
export interface RunMedia {
  id: string
  runId: string
  role: number
  mimeType: string | null
  transferStatus: number
  errorCode: string | null
  errorMessage: string | null
  storageFileId: string | null
  canPreview: boolean
  canRetryTransfer: boolean
}
export interface RunRequest {
  id: string
  runId: string
  sequence: number
  status: number
  responseItems: unknown[]
  usage: Record<string, unknown>
  usageStatus: number
  startedAt: string | null
  finishedAt: string | null
}
export interface RunToolCall {
  id: string
  runId: string
  providerRequestId: string
  exposedName: string
  status: number
  startedAt: string | null
  finishedAt: string | null
}
export interface RunCharge {
  id: string
  runId: string
  status: number
  currency: string
  priceVersionId: string
  multiplierPpm: number
  actualUnits: string | null
  usageComplete: number
  settledAt: string | null
  transactionId: string | null
}
export interface RunChargeItem {
  id: string
  chargeId: string
  providerRequestId: string
  priceRateId: string
  operation: string
  usageKey: string
  selector: Record<string, unknown>
  quantity: string
  unitQuantity: string
  unit: string
  billingUnitPriceUnits: string
  multiplierPpm: number
  amountUnits: string
}
export interface RunRecordMap {
  request: RunRequest
  'tool-call': RunToolCall
  'usage-charge': RunCharge
  'usage-charge-item': RunChargeItem
}
export type RunRecordKind = keyof RunRecordMap
export type RunRecord = RunRecordMap[RunRecordKind]
export interface RunOptions {
  operations: BackendOption<string>[]
  executionStatuses: BackendOption<number>[]
  usageStatuses: BackendOption<number>[]
  billingStatuses: BackendOption<number>[]
  transferStatuses: BackendOption<number>[]
  mediaTransferStatuses: BackendOption<number>[]
  requestStatuses: BackendOption<number>[]
  toolCallStatuses: BackendOption<number>[]
  requestUsageStatuses: BackendOption<number>[]
  usageCompleteness: BackendOption<number>[]
  constraints: {
    defaultLimit: number
    maxLimit: number
    maxHistoryDays: number
    maxSummaryHours: number
    maxFiles: number
  }
}
export interface RunSummary {
  runCount: number
  estimatedUnits: string
  settledUnits: string
}
export interface RunFilter extends CursorInput {
  startAt?: string
  endAt?: string
  agentId?: string
  userId?: string
  platformId?: string
  providerId?: string
  providerModelId?: string
  operation?: string
  executionStatus?: number
  usageStatus?: number
  billingStatus?: number
  transferStatus?: number
}
export interface CreateRunInput {
  agentId: string
  requestKey: string
  operation: string
  conversationId?: string
  input: Record<string, unknown>
}
const own = '/api/admin/v1/ai/chat/run',
  admin = '/api/admin/v1/ai/run'
export function createRun(input: CreateRunInput) {
  return request.post<RunItem, CreateRunInput>(own, input)
}
export function getRun(id: string, management = false) {
  return request.get<RunDetail>(`${management ? admin : own}/${id}`)
}
export function listRuns(params: RunFilter = {}, management = false) {
  return request.get<CursorPage<RunItem>>(management ? admin : own, { params })
}
export function getRunOptions(management = false) {
  return request.get<RunOptions>(management ? `${admin}/options` : '/api/admin/v1/ai/chat/options')
}
export function getRunSummary(params: RunFilter) {
  return request.get<RunSummary>(`${admin}/summary`, { params })
}
export function listRunEvents(id: string, params: { afterSequence?: number; limit?: number } = {}) {
  return request.get<SequencePage<RunEvent>>(`${own}/${id}/event`, { params })
}
export function listRunMedia(id: string, params: CursorInput = {}, management = false) {
  return request.get<CursorPage<RunMedia>>(`${management ? admin : own}/${id}/media`, { params })
}
export function listRunRecords<Kind extends RunRecordKind>(
  id: string,
  kind: Kind,
  params: CursorInput = {},
  management = true,
) {
  return request.get<CursorPage<RunRecordMap[Kind]>>(`${management ? admin : own}/${id}/${kind}`, {
    params,
  })
}
export function cancelRun(id: string) {
  return request.post<{ accepted: boolean }>(`${own}/${id}/cancel`, {})
}
export function retryRunTransfer(id: string) {
  return request.post<{ accepted: boolean }>(`${own}/${id}/media/retry`, {})
}
export function previewRunMedia(
  id: string,
  mediaId: string,
  disposition: 'inline' | 'attachment' = 'inline',
  management = false,
) {
  return request.get<{ url: string; expiresAt: string }>(
    `${management ? admin : own}/${id}/media/${mediaId}/preview`,
    { params: { disposition } },
  )
}
