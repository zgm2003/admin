import { request } from '@/utils/request'
import {
  expectArray,
  expectEmptyObject,
  expectExactKeys,
  expectId,
  expectInteger,
  expectRecord,
  expectString,
} from '@/api/protocol'
import { isYesNo, type YesNo } from '@/enums/yesNo'
import { ProtocolError } from '@/types/http'

export interface DictionaryItem {
  id: number
  dictionaryId: number
  value: string
  labelZh: string
  labelEn: string
  sort: number
  isEnabled: YesNo
  isBuiltin: YesNo
  createdAt: string
  updatedAt: string
}

export interface Dictionary {
  id: number
  code: string
  nameZh: string
  nameEn: string
  description: string
  isEnabled: YesNo
  isBuiltin: YesNo
  itemCount?: number
  createdAt: string
  updatedAt: string
}

export interface DictionaryPage {
  list: Dictionary[]
  total: number
  page: number
  pageSize: number
}

export interface DictionaryDetail {
  dictionary: Dictionary
  items: DictionaryItem[]
}

export interface DictionaryOptions {
  [code: string]: Array<{ label: string; value: string }>
}

export interface DictionaryStatusResult {
  id: number
  isEnabled: YesNo
}

function parseDictionary(value: unknown, context: string): Dictionary {
  const record = expectRecord(value, context)
  const keys = [
    'id',
    'code',
    'nameZh',
    'nameEn',
    'description',
    'isEnabled',
    'isBuiltin',
    'createdAt',
    'updatedAt',
  ]
  if ('itemCount' in record) keys.push('itemCount')
  const r = expectExactKeys(record, keys, context)
  const isEnabled = r.isEnabled,
    isBuiltin = r.isBuiltin
  if (!isYesNo(isEnabled) || !isYesNo(isBuiltin))
    throw new ProtocolError(`${context} status is invalid`)
  return {
    id: expectInteger(r.id, `${context}.id`),
    code: expectString(r.code, `${context}.code`),
    nameZh: expectString(r.nameZh, `${context}.nameZh`),
    nameEn: expectString(r.nameEn, `${context}.nameEn`),
    description: expectString(r.description, `${context}.description`),
    isEnabled,
    isBuiltin,
    itemCount:
      r.itemCount === undefined ? undefined : expectInteger(r.itemCount, `${context}.itemCount`),
    createdAt: expectString(r.createdAt, `${context}.createdAt`),
    updatedAt: expectString(r.updatedAt, `${context}.updatedAt`),
  }
}

function parseItem(value: unknown, context: string): DictionaryItem {
  const r = expectExactKeys(
    value,
    [
      'id',
      'dictionaryId',
      'value',
      'labelZh',
      'labelEn',
      'sort',
      'isEnabled',
      'isBuiltin',
      'createdAt',
      'updatedAt',
    ],
    context,
  )
  const isEnabled = r.isEnabled,
    isBuiltin = r.isBuiltin
  if (!isYesNo(isEnabled) || !isYesNo(isBuiltin))
    throw new ProtocolError(`${context} status is invalid`)
  return {
    id: expectInteger(r.id, `${context}.id`),
    dictionaryId: expectInteger(r.dictionaryId, `${context}.dictionaryId`),
    value: expectString(r.value, `${context}.value`),
    labelZh: expectString(r.labelZh, `${context}.labelZh`),
    labelEn: expectString(r.labelEn, `${context}.labelEn`),
    sort: expectInteger(r.sort, `${context}.sort`),
    isEnabled,
    isBuiltin,
    createdAt: expectString(r.createdAt, `${context}.createdAt`),
    updatedAt: expectString(r.updatedAt, `${context}.updatedAt`),
  }
}

export async function getDictionaries(params: {
  page: number
  pageSize: number
  keyword?: string
  isEnabled?: YesNo
}): Promise<DictionaryPage> {
  const value = await request<unknown>({
    method: 'GET',
    url: '/api/admin/v1/system/dictionary',
    params,
  })
  const r = expectExactKeys(value, ['list', 'total', 'page', 'pageSize'], 'dictionaries')
  const list = expectArray(r.list, 'dictionaries.list').map((x, i) =>
    parseDictionary(x, `dictionaries.list[${i}]`),
  )
  return {
    list,
    total: expectInteger(r.total, 'dictionaries.total'),
    page: expectInteger(r.page, 'dictionaries.page'),
    pageSize: expectInteger(r.pageSize, 'dictionaries.pageSize'),
  }
}

export async function getDictionary(id: number): Promise<DictionaryDetail> {
  const value = await request<unknown>({
    method: 'GET',
    url: `/api/admin/v1/system/dictionary/${id}`,
  })
  const r = expectExactKeys(value, ['dictionary', 'items'], 'dictionary detail')
  return {
    dictionary: parseDictionary(r.dictionary, 'dictionary detail.dictionary'),
    items: expectArray(r.items, 'dictionary detail.items').map((x, i) =>
      parseItem(x, `dictionary detail.items[${i}]`),
    ),
  }
}

export async function getDictionaryOptions(codes: string[]): Promise<DictionaryOptions> {
  const value = await request<unknown>({
    method: 'GET',
    url: '/api/v1/system/dictionary/options',
    params: { codes: codes.join(',') },
  })
  const r = expectRecord(value, 'dictionary options')
  const actualCodes = Object.keys(r).sort()
  const expectedCodes = [...codes].sort()
  if (
    actualCodes.length !== expectedCodes.length ||
    actualCodes.some((code, index) => code !== expectedCodes[index])
  ) {
    throw new ProtocolError('dictionary options has invalid fields')
  }
  const result: DictionaryOptions = {}
  for (const [code, raw] of Object.entries(r)) {
    result[code] = expectArray(raw, `dictionary options.${code}`).map((x, i) => {
      const item = expectExactKeys(x, ['label', 'value'], `dictionary options.${code}[${i}]`)
      return {
        label: expectString(item.label, 'dictionary option.label'),
        value: expectString(item.value, 'dictionary option.value'),
      }
    })
  }
  return result
}

export async function createDictionary(input: {
  code: string
  nameZh: string
  nameEn: string
  description: string
}): Promise<number> {
  const value = await request<unknown>({
    method: 'POST',
    url: '/api/admin/v1/system/dictionary',
    data: input,
  })
  return expectId(value, 'dictionary create').id
}

export async function updateDictionary(
  id: number,
  input: {
    nameZh: string
    nameEn: string
    description: string
  },
): Promise<void> {
  const value = await request<unknown>({
    method: 'PUT',
    url: `/api/admin/v1/system/dictionary/${id}`,
    data: {
      nameZh: input.nameZh,
      nameEn: input.nameEn,
      description: input.description,
    },
  })
  expectEmptyObject(value, 'dictionary update')
}

export async function updateDictionaryStatus(
  id: number,
  isEnabled: YesNo,
): Promise<DictionaryStatusResult> {
  const value = await request<unknown>({
    method: 'PATCH',
    url: `/api/admin/v1/system/dictionary/${id}/status`,
    data: { isEnabled },
  })
  return parseStatusResult(value, 'dictionary status update')
}

export async function deleteDictionary(id: number): Promise<void> {
  const value = await request<unknown>({
    method: 'DELETE',
    url: `/api/admin/v1/system/dictionary/${id}`,
  })
  expectEmptyObject(value, 'dictionary delete')
}

export async function createDictionaryItem(
  dictionaryId: number,
  input: { value: string; labelZh: string; labelEn: string; sort: number },
): Promise<number> {
  const value = await request<unknown>({
    method: 'POST',
    url: `/api/admin/v1/system/dictionary/${dictionaryId}/item`,
    data: input,
  })
  return expectId(value, 'dictionary item create').id
}

export async function updateDictionaryItem(
  dictionaryId: number,
  itemId: number,
  input: { labelZh: string; labelEn: string; sort: number },
): Promise<void> {
  const value = await request<unknown>({
    method: 'PUT',
    url: `/api/admin/v1/system/dictionary/${dictionaryId}/item/${itemId}`,
    data: {
      labelZh: input.labelZh,
      labelEn: input.labelEn,
      sort: input.sort,
    },
  })
  expectEmptyObject(value, 'dictionary item update')
}

export async function updateDictionaryItemStatus(
  dictionaryId: number,
  itemId: number,
  isEnabled: YesNo,
): Promise<DictionaryStatusResult> {
  const value = await request<unknown>({
    method: 'PATCH',
    url: `/api/admin/v1/system/dictionary/${dictionaryId}/item/${itemId}/status`,
    data: { isEnabled },
  })
  return parseStatusResult(value, 'dictionary item status update')
}

function parseStatusResult(value: unknown, context: string): DictionaryStatusResult {
  const result = expectExactKeys(value, ['id', 'isEnabled'], context)
  if (!isYesNo(result.isEnabled)) throw new ProtocolError(`${context}.isEnabled is invalid`)
  return {
    id: expectInteger(result.id, `${context}.id`),
    isEnabled: result.isEnabled,
  }
}

export async function deleteDictionaryItem(dictionaryId: number, itemId: number): Promise<void> {
  const value = await request<unknown>({
    method: 'DELETE',
    url: `/api/admin/v1/system/dictionary/${dictionaryId}/item/${itemId}`,
  })
  expectEmptyObject(value, 'dictionary item delete')
}
