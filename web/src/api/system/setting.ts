import { request } from '@/utils/request'
import { expectArray, expectEmptyObject, expectExactKeys, expectInteger, expectString } from '@/api/protocol'
import { isYesNo, type YesNo } from '@/enums/yesNo'
import { ProtocolError } from '@/types/http'

export type SettingValueType = 1 | 2 | 3 | 4
export interface SystemSetting { id: number; key: string; value: string; valueType: SettingValueType; description: string; isEnabled: YesNo; isBuiltin: YesNo; createdAt: string; updatedAt: string }
export interface SettingPage { list: SystemSetting[]; total: number; page: number; pageSize: number }

function parseSetting(value: unknown, context: string): SystemSetting {
  const record = expectExactKeys(value, ['id', 'key', 'value', 'valueType', 'description', 'isEnabled', 'isBuiltin', 'createdAt', 'updatedAt'], context)
  const valueType = expectInteger(record.valueType, `${context}.valueType`)
  if (valueType < 1 || valueType > 4 || !isYesNo(record.isEnabled) || !isYesNo(record.isBuiltin)) throw new ProtocolError(`${context} has invalid fields`)
  return { id: expectInteger(record.id, `${context}.id`), key: expectString(record.key, `${context}.key`), value: expectString(record.value, `${context}.value`), valueType: valueType as SettingValueType, description: expectString(record.description, `${context}.description`), isEnabled: record.isEnabled, isBuiltin: record.isBuiltin, createdAt: expectString(record.createdAt, `${context}.createdAt`), updatedAt: expectString(record.updatedAt, `${context}.updatedAt`) }
}

export async function getSettings(params: { page: number; pageSize: number; keyword?: string; isEnabled?: YesNo }): Promise<SettingPage> {
  const value = await request<unknown>({ method: 'GET', url: '/api/admin/v1/system/setting', params })
  const record = expectExactKeys(value, ['list', 'total', 'page', 'pageSize'], 'settings')
  const list = expectArray(record.list, 'settings.list').map((item, index) => parseSetting(item, `settings.list[${index}]`))
  return { list, total: expectInteger(record.total, 'settings.total'), page: expectInteger(record.page, 'settings.page'), pageSize: expectInteger(record.pageSize, 'settings.pageSize') }
}

export async function createSetting(input: { key: string; value: string; valueType: SettingValueType; description?: string }): Promise<number> {
  const value = await request<unknown>({ method: 'POST', url: '/api/admin/v1/system/setting', data: input })
  return expectInteger(expectExactKeys(value, ['id'], 'create setting').id, 'create setting.id')
}

export async function updateSetting(key: string, input: { value: string; valueType: SettingValueType; description?: string }): Promise<void> {
  expectEmptyObject(await request<unknown>({ method: 'PUT', url: `/api/admin/v1/system/setting/${encodeURIComponent(key)}`, data: input }), 'update setting')
}

export async function updateSettingStatus(key: string, isEnabled: YesNo): Promise<void> {
  expectExactKeys(await request<unknown>({ method: 'PATCH', url: `/api/admin/v1/system/setting/${encodeURIComponent(key)}/status`, data: { isEnabled } }), ['key', 'isEnabled'], 'setting status')
}

export async function deleteSetting(key: string): Promise<void> {
  expectEmptyObject(await request<unknown>({ method: 'DELETE', url: `/api/admin/v1/system/setting/${encodeURIComponent(key)}` }), 'delete setting')
}
