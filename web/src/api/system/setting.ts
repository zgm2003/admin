import { request } from '@/utils/request'
import { type YesNo } from '@/enums/yesNo'
import type { SettingPresentation } from '@/api/system/settingOptions'

export type SettingValueType = number

export interface SystemSetting {
  presentation: SettingPresentation
  id: number
  key: string
  value: string
  valueType: SettingValueType
  description: string
  isEnabled: YesNo
  isBuiltin: YesNo
  createdAt: string
  updatedAt: string
}

export interface SettingPage {
  list: SystemSetting[]
  total: number
  page: number
  pageSize: number
}

export interface BrandSettings {
  titleZhCN: string
  titleEnUS: string
  defaultAvatar: string
}

export type LegalDocumentKind = 'userAgreement' | 'privacyPolicy'

export interface LegalDocument {
  kind: LegalDocumentKind
  contentHtml: string
}

export async function getSettings(params: {
  page: number
  pageSize: number
  keyword?: string
  isEnabled?: YesNo
}): Promise<SettingPage> {
  return request.get<SettingPage>('/api/admin/v1/system/setting', { params })
}

export async function getBrandSettings(): Promise<BrandSettings> {
  return request.get<BrandSettings>('/api/admin/v1/system/setting/brand')
}

export async function updateBrandSettings(input: BrandSettings): Promise<void> {
  return request.put<void>('/api/admin/v1/system/setting/brand', input)
}

export async function getPublicLegalDocument(kind: LegalDocumentKind): Promise<LegalDocument> {
  return request.get<LegalDocument>(`/api/v1/system/setting/legal/${kind}`)
}

export async function getLegalDocument(kind: LegalDocumentKind): Promise<LegalDocument> {
  return request.get<LegalDocument>(`/api/admin/v1/system/setting/legal/${kind}`)
}

export async function updateLegalDocument(
  kind: LegalDocumentKind,
  contentHtml: string,
): Promise<void> {
  return request.put<void>(`/api/admin/v1/system/setting/legal/${kind}`, { contentHtml })
}

export interface CreateSettingResult {
  id: number
  presentation: SettingPresentation
}

export async function createSetting(input: {
  key: string
  value: string
  valueType: SettingValueType
  description?: string
}): Promise<CreateSettingResult> {
  return request.post<CreateSettingResult>('/api/admin/v1/system/setting', input)
}

export async function updateSetting(
  key: string,
  input: { value: string; valueType: SettingValueType; description?: string },
): Promise<void> {
  return request.put<void>(`/api/admin/v1/system/setting/${encodeURIComponent(key)}`, input)
}

export async function updateSettingStatus(key: string, isEnabled: YesNo): Promise<void> {
  return request.patch<void>(`/api/admin/v1/system/setting/${encodeURIComponent(key)}/status`, {
    isEnabled,
  })
}

export async function deleteSetting(key: string): Promise<void> {
  return request.delete<void>(`/api/admin/v1/system/setting/${encodeURIComponent(key)}`)
}
