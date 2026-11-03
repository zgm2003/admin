import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'

export type TransactionSourceType = number
export type TransactionDirection = number

export interface TransactionOption<T extends number = number> {
  value: T
  label: string
}
export interface TransactionOptions {
  directions: TransactionOption<TransactionDirection>[]
  sourceTypes: TransactionOption<TransactionSourceType>[]
}

export interface PersonalTransaction {
  id: number
  transactionNo: string
  sourceType: TransactionSourceType
  direction: TransactionDirection
  amount: string
  balanceBefore: string
  balanceAfter: string
  createdAt: string
}

export interface Transaction extends PersonalTransaction {
  userId: number
  username: string
  platformId: number
  platformCode: string
  orderId: number | null
  redeemCodeId?: number | null
  aiUsageChargeId?: number | null
}

export interface PersonalTransactionQuery extends PageRequest {
  direction?: TransactionDirection
  sourceType?: TransactionSourceType
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
export function getTransactionOptions(): Promise<TransactionOptions> {
  return request.get<TransactionOptions>('/api/admin/v1/payment/transaction/options')
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
