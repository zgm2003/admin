import { request } from '@/utils/request'
import type { YesNo } from '@/enums/yesNo'
import type { PageRequest, PageResult } from '@/types/pagination'

export interface NumericOption<T extends number = number> {
  value: T
  label: string
}

export interface RechargePackage {
  id: number
  name: string
  amount: string
  badge: string | null
  sortOrder: number
  isEnabled: YesNo
  actions: { update: boolean; status: boolean; delete: boolean }
  presentation: { status: string }
  createdAt: string
  updatedAt: string
}

export interface RechargePackageQuery extends PageRequest {
  keyword?: string
  isEnabled?: YesNo
}

export interface RechargePackageOptions {
  statuses: Array<NumericOption<YesNo>>
  defaults: { sortOrder: number; isEnabled: YesNo }
  limits: {
    name: number
    badge: number
    amountScale: number
    minimumAmount: string
    maximumAmount: string
    sortMinimum: number
    sortMaximum: number
  }
}

export interface CreateRechargePackageInput {
  name: string
  amount: string
  badge: string | null
  sortOrder: number
  isEnabled: YesNo
}

export interface UpdateRechargePackageInput {
  name: string
  amount: string
  badge: string | null
  sortOrder: number
}

const base = '/api/admin/v1/payment/rechargepackage'

export function getRechargePackages(
  query: RechargePackageQuery,
): Promise<PageResult<RechargePackage>> {
  return request.get<PageResult<RechargePackage>>(base, { params: query })
}

export function getRechargePackage(id: number): Promise<RechargePackage> {
  return request.get<RechargePackage>(`${base}/${id}`)
}

export function getRechargePackageOptions(): Promise<RechargePackageOptions> {
  return request.get<RechargePackageOptions>(`${base}/options`)
}

export function createRechargePackage(input: CreateRechargePackageInput): Promise<{ id: number }> {
  return request.post<{ id: number }, CreateRechargePackageInput>(base, input)
}

export function updateRechargePackage(
  id: number,
  input: UpdateRechargePackageInput,
): Promise<Record<string, never>> {
  return request.put<Record<string, never>, UpdateRechargePackageInput>(`${base}/${id}`, input)
}

export function updateRechargePackageStatus(
  id: number,
  isEnabled: YesNo,
): Promise<{ id: number; isEnabled: YesNo }> {
  return request.patch<{ id: number; isEnabled: YesNo }, { isEnabled: YesNo }>(
    `${base}/${id}/status`,
    { isEnabled },
  )
}

export function deleteRechargePackage(id: number): Promise<Record<string, never>> {
  return request.delete<Record<string, never>>(`${base}/${id}`)
}
