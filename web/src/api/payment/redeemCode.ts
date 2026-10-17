import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'

export interface RedeemCodeActionState {
  void: boolean
  export: boolean
}

export interface RedeemCode {
  id: number
  batchId: number
  batchNo: string
  codeMask: string
  status: number
  amount: string
  expiresAt: string | null
  usedBy: number | null
  usedAt: string | null
  createdAt: string
  actions: RedeemCodeActionState
}

export interface RedeemCodeExport extends RedeemCode {
  code: string
}

export interface RedeemCodeOptions {
  limits: {
    maxBatchQuantity: number
    maxVoidQuantity: number
    maxExportQuantity: number
    maxCodeLength: number
    codeLength: number
    remark: number
    minimumAmount: string
    maximumAmount: string
    amountScale: number
  }
  defaults: { quantity: number }
  statuses: Array<{ value: number; label: string }>
}

export interface RedeemCodeQuery extends PageRequest {
  batchId?: number
  status?: number
}

export interface GenerateRedeemCodeInput {
  requestId: string
  amount: string
  quantity: number
  expiresAt: string | null
  remark: string
}

export interface GenerateRedeemCodeResult {
  batchId: number
  batchNo: string
  codes: string[]
}

export interface RedeemResult {
  transactionId: number
  transactionNo: string
  amount: string
  balance: string
  createdAt: string
}

const adminBase = '/api/admin/v1/payment/redeemcode'

export function getRedeemCodeOptions(): Promise<RedeemCodeOptions> {
  return request.get<RedeemCodeOptions>(`${adminBase}/options`)
}

export function getRedeemCodes(query: RedeemCodeQuery): Promise<PageResult<RedeemCode>> {
  return request.get<PageResult<RedeemCode>>(adminBase, { params: query })
}

export function getRedeemCode(id: number): Promise<RedeemCode> {
  return request.get<RedeemCode>(`${adminBase}/${id}`)
}

export function lookupRedeemCode(code: string): Promise<RedeemCode> {
  return request.post<RedeemCode, { code: string }>(`${adminBase}/lookup`, { code })
}

export function generateRedeemCodeBatch(
  input: GenerateRedeemCodeInput,
): Promise<GenerateRedeemCodeResult> {
  return request.post<GenerateRedeemCodeResult, GenerateRedeemCodeInput>(
    `${adminBase}/batch`,
    input,
  )
}

export function voidRedeemCodes(ids: number[]): Promise<Record<string, never>> {
  return request.patch<Record<string, never>, { ids: number[] }>(`${adminBase}/void`, { ids })
}

export function exportRedeemCodes(ids: number[]): Promise<RedeemCodeExport[]> {
  return request.post<RedeemCodeExport[], { ids: number[] }>(`${adminBase}/export`, { ids })
}

export function redeemWalletCode(code: string): Promise<RedeemResult> {
  return request.post<RedeemResult, { code: string }>('/api/v1/payment/wallet/redemption', {
    code,
  })
}
