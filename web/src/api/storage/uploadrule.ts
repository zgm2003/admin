import { request } from '@/utils/request'
import { isYesNo, type YesNo } from '@/enums/yes-no'
import type { PageRequest, PageResult } from '@/types/pagination'
import {
  expectArray,
  expectEmptyObject,
  expectId,
  expectInteger,
  expectPage,
  expectRecord,
  expectString,
} from '@/api/protocol'
import { ProtocolError } from '@/types/http'
export interface UploadRule {
  id: number
  platformId: number
  platformCode: string
  platformName: string
  codes: string[]
  name: string
  cosConfigId: number
  cosConfigName: string
  maxFileSizeBytes: number
  allowedExtensions: string[]
  allowedMimeTypes: string[]
  accessMode: 'private' | 'public'
  isEnabled: YesNo
  remark: string
  createdAt: string
  updatedAt: string
}
export interface UploadRuleQuery extends PageRequest {
  platformId?: number
  cosConfigId?: number
  keyword?: string
  isEnabled?: YesNo
}
export interface UploadRuleInput {
  platformId?: number
  codes?: string[]
  name: string
  cosConfigId: number
  maxFileSizeBytes: number
  allowedExtensions: string[]
  allowedMimeTypes: string[]
  accessMode: 'private' | 'public'
  isEnabled?: YesNo
  remark: string
}
export interface PlatformOption {
  id: number
  code: string
  name: string
  isEnabled: YesNo
}
export interface ConfigSummary {
  id: number
  name: string
  bucket: string
  region: string
  isEnabled: YesNo
}
export interface UploadRulePageInit {
  platforms: PlatformOption[]
  configs: ConfigSummary[]
}
export async function listUploadRules(query: UploadRuleQuery): Promise<PageResult<UploadRule>> {
  return expectPage(
    await request<unknown>({
      method: 'GET',
      url: '/api/admin/v1/storage/upload-rules',
      params: query,
    }),
    parseUploadRule,
    'upload rules',
  )
}
export async function getUploadRule(id: number): Promise<UploadRule> {
  return parseUploadRule(
    await request<unknown>({ method: 'GET', url: `/api/admin/v1/storage/upload-rules/${id}` }),
    0,
  )
}
export async function getUploadRulePageInit(): Promise<UploadRulePageInit> {
  const result = expectRecord(
    await request<unknown>({ method: 'GET', url: '/api/admin/v1/storage/upload-rules/page-init' }),
    'upload rule page init',
  )
  return {
    platforms: expectArray(result.platforms, 'upload rule page init.platforms').map(parsePlatform),
    configs: expectArray(result.configs, 'upload rule page init.configs').map(parseConfig),
  }
}
export async function createUploadRule(data: UploadRuleInput): Promise<{ id: number }> {
  return expectId(
    await request<unknown>({ method: 'POST', url: '/api/admin/v1/storage/upload-rules', data }),
    'upload rule create result',
  )
}
export async function updateUploadRule(
  id: number,
  data: UploadRuleInput,
): Promise<Record<string, never>> {
  return expectEmptyObject(
    await request<unknown>({
      method: 'PUT',
      url: `/api/admin/v1/storage/upload-rules/${id}`,
      data,
    }),
    'upload rule update result',
  )
}
export async function updateUploadRuleStatus(
  id: number,
  isEnabled: YesNo,
): Promise<{ id: number; isEnabled: YesNo }> {
  const result = expectRecord(
    await request<unknown>({
      method: 'PATCH',
      url: `/api/admin/v1/storage/upload-rules/${id}/status`,
      data: { isEnabled },
    }),
    'upload rule status result',
  )
  if (!isYesNo(result.isEnabled)) throw new ProtocolError('upload rule status is invalid')
  return { id: expectInteger(result.id, 'upload rule status id'), isEnabled: result.isEnabled }
}
export async function deleteUploadRule(id: number): Promise<Record<string, never>> {
  return expectEmptyObject(
    await request<unknown>({ method: 'DELETE', url: `/api/admin/v1/storage/upload-rules/${id}` }),
    'upload rule delete result',
  )
}

function parseUploadRule(value: unknown, index: number): UploadRule {
  const item = expectRecord(value, `upload rules[${index}]`)
  const accessMode = item.accessMode
  const isEnabled = item.isEnabled
  if (accessMode !== 'private' && accessMode !== 'public')
    throw new ProtocolError('upload rule access mode is invalid')
  if (!isYesNo(isEnabled)) throw new ProtocolError('upload rule status is invalid')
  const strings = (field: string) =>
    expectArray(item[field], `upload rule.${field}`).map((entry, entryIndex) =>
      expectString(entry, `upload rule.${field}[${entryIndex}]`),
    )
  return {
    id: expectInteger(item.id, 'upload rule.id'),
    platformId: expectInteger(item.platformId, 'upload rule.platformId'),
    platformCode: expectString(item.platformCode, 'upload rule.platformCode'),
    platformName: expectString(item.platformName, 'upload rule.platformName'),
    codes: strings('codes'),
    name: expectString(item.name, 'upload rule.name'),
    cosConfigId: expectInteger(item.cosConfigId, 'upload rule.cosConfigId'),
    cosConfigName: expectString(item.cosConfigName, 'upload rule.cosConfigName'),
    maxFileSizeBytes: expectInteger(item.maxFileSizeBytes, 'upload rule.maxFileSizeBytes'),
    allowedExtensions: strings('allowedExtensions'),
    allowedMimeTypes: strings('allowedMimeTypes'),
    accessMode,
    isEnabled,
    remark: expectString(item.remark, 'upload rule.remark'),
    createdAt: expectString(item.createdAt, 'upload rule.createdAt'),
    updatedAt: expectString(item.updatedAt, 'upload rule.updatedAt'),
  }
}
function parsePlatform(value: unknown, index: number): PlatformOption {
  const item = expectRecord(value, `platforms[${index}]`)
  const isEnabled = item.isEnabled
  if (!isYesNo(isEnabled)) throw new ProtocolError('platform status is invalid')
  return {
    id: expectInteger(item.id, 'platform.id'),
    code: expectString(item.code, 'platform.code'),
    name: expectString(item.name, 'platform.name'),
    isEnabled,
  }
}
function parseConfig(value: unknown, index: number): ConfigSummary {
  const item = expectRecord(value, `configs[${index}]`)
  const isEnabled = item.isEnabled
  if (!isYesNo(isEnabled)) throw new ProtocolError('config status is invalid')
  return {
    id: expectInteger(item.id, 'config.id'),
    name: expectString(item.name, 'config.name'),
    bucket: expectString(item.bucket, 'config.bucket'),
    region: expectString(item.region, 'config.region'),
    isEnabled,
  }
}
