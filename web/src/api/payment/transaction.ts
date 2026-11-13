import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'

export type TransactionSourceType = number

export interface PersonalTransaction {
  id: number
  transactionNo: string
  sourceType: TransactionSourceType
  amount: string
  balanceBefore: string
  balanceAfter: string
  createdAt: string
  runId?: number
  agentName?: string
  modelName?: string
  inputTokens?: number
  outputTokens?: number
}

export interface Transaction extends PersonalTransaction {
  userId: number
  username: string
  platformId: number
  platformCode: string
  orderId: number | null
}

export type LedgerFlow = 'credit' | 'debit'

export interface PersonalTransactionQuery extends PageRequest {
  flow?: LedgerFlow
  keyword?: string
  from?: string
  to?: string
}

export interface TransactionQuery extends PersonalTransactionQuery {
  userId?: number
  platformId?: number
}

export function getTransactions(query: TransactionQuery): Promise<PageResult<Transaction>> {
  return request.get<PageResult<Transaction>>('/api/admin/v1/payment/transaction', {
    params: query,
  })
}

export function getTransaction(id: number): Promise<Transaction> {
  return request.get<Transaction>(`/api/admin/v1/payment/transaction/${id}`)
}

export function getMyTransactions(
  query: PersonalTransactionQuery,
): Promise<PageResult<PersonalTransaction>> {
  return request.get<PageResult<PersonalTransaction>>('/api/admin/v1/payment/wallet/transaction', {
    params: query,
  })
}
