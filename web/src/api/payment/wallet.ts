import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'

export interface MyWallet {
  hasWallet: boolean
  currency: string
  balance: string
}

export interface Wallet {
  userId: number
  username: string
  currency: string
  balance: string
  createdAt: string
  updatedAt: string
}

export function getMyWallet(): Promise<MyWallet> {
  return request.get<MyWallet>('/api/admin/v1/payment/wallet/me')
}

export function getWallet(userId: number): Promise<Wallet> {
  return request.get<Wallet>(`/api/admin/v1/payment/wallet/${userId}`)
}

export function getWallets(query: PageRequest): Promise<PageResult<Wallet>> {
  return request.get<PageResult<Wallet>>('/api/admin/v1/payment/wallet', { params: query })
}
