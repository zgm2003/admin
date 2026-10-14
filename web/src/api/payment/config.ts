import { request } from '@/utils/request'
import type { YesNo } from '@/enums/yesNo'
import type { PageResult, PageRequest } from '@/types/pagination'
export interface NumericOption {
  value: number
  label: string
}
export interface CertificateInfo {
  subject: string
  issuer: string
  serialNumber: string
  notBefore: string
  notAfter: string
}
export interface PaymentConfig {
  id: number
  name: string
  provider: number
  environment: number
  appId: string
  sellerId: string
  currentVersion: number
  sortOrder: number
  isEnabled: YesNo
  notifyUrl: string
  notifyUrlTemplate: string
  payMethods: number[]
  credentialConfigured: boolean
  certificates: { app: CertificateInfo; alipay: CertificateInfo; roots: CertificateInfo[] }
  actions: { update: boolean; status: boolean; delete: boolean; test: boolean }
  presentation: {
    provider: string
    environment: string
    payMethods: NumericOption[]
    status: string
  }
  createdAt: string
  updatedAt: string
}
export interface PaymentConfigQuery extends PageRequest {
  keyword?: string
  isEnabled?: YesNo
}
export interface PaymentConfigOptions {
  providers: NumericOption[]
  environments: NumericOption[]
  payMethods: NumericOption[]
  statuses: NumericOption[]
  defaults: {
    provider: number
    environment: number
    payMethods: number[]
    isEnabled: YesNo
    sortOrder: number
    notifyUrl: string
  }
  limits: {
    name: number
    identity: number
    notifyUrl: number
    certificateBytes: number
    privateKeyBytes: number
    sortMinimum: number
    sortMaximum: number
  }
  notifyPathTemplate: string
}
export interface CreatePaymentConfigInput {
  name: string
  provider: number
  environment: number
  appId: string
  sellerId: string
  privateKey: string
  appCertPEM: string
  alipayCertPEM: string
  rootCertPEM: string
  notifyUrl: string
  payMethods: number[]
  sortOrder: number
  isEnabled: YesNo
}
export interface UpdatePaymentConfigInput {
  name: string
  sortOrder: number
}
export interface PaymentConfigVersionInput {
  privateKey: string | null
  appCertPEM: string | null
  alipayCertPEM: string | null
  rootCertPEM: string | null
  notifyUrl: string
  payMethods: number[]
}
const base = '/api/admin/v1/payment/config'
export function getPaymentConfigs(query: PaymentConfigQuery): Promise<PageResult<PaymentConfig>> {
  return request.get<PageResult<PaymentConfig>>(base, { params: query })
}
export function getPaymentConfig(id: number): Promise<PaymentConfig> {
  return request.get<PaymentConfig>(`${base}/${id}`)
}
export function getPaymentConfigOptions(): Promise<PaymentConfigOptions> {
  return request.get<PaymentConfigOptions>(`${base}/options`)
}
export function createPaymentConfig(input: CreatePaymentConfigInput): Promise<{ id: number }> {
  return request.post<{ id: number }, CreatePaymentConfigInput>(base, input)
}
export function updatePaymentConfig(
  id: number,
  input: UpdatePaymentConfigInput,
): Promise<Record<string, never>> {
  return request.put<Record<string, never>, UpdatePaymentConfigInput>(`${base}/${id}`, {
    name: input.name,
    sortOrder: input.sortOrder,
  })
}
export function publishPaymentConfigVersion(
  id: number,
  input: PaymentConfigVersionInput,
): Promise<Record<string, never>> {
  return request.post<Record<string, never>, PaymentConfigVersionInput>(
    `${base}/${id}/version`,
    input,
  )
}
export function updatePaymentConfigStatus(
  id: number,
  isEnabled: YesNo,
): Promise<{ id: number; isEnabled: YesNo }> {
  return request.patch<{ id: number; isEnabled: YesNo }, { isEnabled: YesNo }>(
    `${base}/${id}/status`,
    { isEnabled },
  )
}
export function deletePaymentConfig(id: number): Promise<Record<string, never>> {
  return request.delete<Record<string, never>>(`${base}/${id}`)
}
export function testPaymentConfig(id: number): Promise<Record<string, never>> {
  return request.post<Record<string, never>>(`${base}/${id}/test`)
}
