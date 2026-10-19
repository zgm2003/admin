import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'

export interface NumericOption {
  value: number
  label: string
}

export interface RechargeOrder {
  id: number
  orderNo: string
  packageName: string
  amount: string
  status: number
  statusLabel: string
  expiresAt: string
  paidAt: string | null
  creditedAt: string | null
  closedAt: string | null
  actions: { pay: boolean; sync: boolean; close: boolean }
}

export interface RechargeOptions {
  payMethods: NumericOption[]
  packages: Array<{ id: number; name: string; amount: string }>
  available: boolean
}

export interface Checkout {
  order: RechargeOrder
  checkout: string
}

export interface CreateRechargeInput {
  requestId: string
  packageId: number
  payMethod: number
  returnUrl: string
}

export interface OrderQuery extends PageRequest {
  userId?: number
  platformId?: number
}

export function getRechargeOptions(): Promise<RechargeOptions> {
  return request.get<RechargeOptions>('/api/admin/v1/payment/recharge/options')
}

export function getMyRechargeOrders(query: PageRequest): Promise<PageResult<RechargeOrder>> {
  return request.get<PageResult<RechargeOrder>>('/api/admin/v1/payment/recharge', { params: query })
}

export function getMyRechargeOrder(id: number): Promise<RechargeOrder> {
  return request.get<RechargeOrder>(`/api/admin/v1/payment/recharge/${id}`)
}

export function createRecharge(input: CreateRechargeInput): Promise<Checkout> {
  return request.post<Checkout, CreateRechargeInput>('/api/admin/v1/payment/recharge', input)
}

export function payRecharge(id: number): Promise<Checkout> {
  return request.post<Checkout>(`/api/admin/v1/payment/recharge/${id}/pay`)
}

export function getOrders(query: OrderQuery): Promise<PageResult<RechargeOrder>> {
  return request.get<PageResult<RechargeOrder>>('/api/admin/v1/payment/order', { params: query })
}

export function getOrder(id: number): Promise<RechargeOrder> {
  return request.get<RechargeOrder>(`/api/admin/v1/payment/order/${id}`)
}
