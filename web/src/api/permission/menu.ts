import { request } from '@/utils/request'
import { isYesNo, type YesNo } from '@/enums/yes-no'
import { isMenuIconName, type MenuIconName } from '@/icons/menu-icons'
import { ProtocolError } from '@/types/http'
import { expectExactKeys, expectId, expectInteger } from '@/api/protocol'

export const menuI18nKeyPattern = /^[a-z][a-z0-9]*(?:\.[a-z][a-zA-Z0-9]*)+$/
export const menuCodePattern = /^[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?::[a-z][a-z0-9]*(?:-[a-z0-9]+)*)*$/
export const menuPathPattern =
  /^\/[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?:\/[a-z][a-z0-9]*(?:-[a-z0-9]+)*)*$/
export const componentPathPattern =
  /^[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?:\/[a-z][a-z0-9]*(?:-[a-z0-9]+)*)*$/

const staticPaths: ReadonlySet<string> = new Set(['/login', '/dashboard'])

export function isMenuI18nKey(value: string): boolean {
  return value.length <= 128 && menuI18nKeyPattern.test(value)
}

export function isMenuPath(value: string): boolean {
  return value.length <= 255 && !staticPaths.has(value) && menuPathPattern.test(value)
}

export function isComponentPath(value: string): boolean {
  return value.length <= 255 && componentPathPattern.test(value)
}

export function isMenuIcon(value: string): value is MenuIconName {
  return isMenuIconName(value)
}

export type ManagedMenuType = 'directory' | 'page' | 'action'

export interface MenuPlatformOption {
  id: number
  code: string
  name: string
  isEnabled: YesNo
}

export interface ManagedMenuNode {
  id: number
  platformId: number
  platformCode: string
  platformName: string
  parentId: number | null
  menuType: ManagedMenuType
  name: string
  code: string
  i18nKey: string | null
  path: string | null
  componentPath: string | null
  icon: MenuIconName | null
  remark?: string | null
  sortOrder: number
  isEnabled: YesNo
  isHidden: YesNo
  isProtected: YesNo
  createdAt: string
  updatedAt: string
  children: ManagedMenuNode[]
}

export interface MenuCatalogResponse {
  platforms: MenuPlatformOption[]
  menuTree: ManagedMenuNode[]
}

export interface MenuListQuery {
  platformId: number
}

export interface CreateMenuInput {
  platformId: number
  parentId: number | null
  menuType: ManagedMenuType
  name: string
  code: string
  i18nKey: string | null
  path: string | null
  componentPath: string | null
  icon: MenuIconName | null
  remark: string | null
  sortOrder: number
  isEnabled: YesNo
  isHidden: YesNo
}

export interface UpdateMenuInput {
  parentId: number | null
  menuType: ManagedMenuType
  name: string
  i18nKey: string | null
  path: string | null
  componentPath: string | null
  icon: MenuIconName | null
  remark: string | null
  sortOrder: number
  isHidden: YesNo
}

export interface MenuIDResult {
  id: number
}

export interface MenuStatusResult {
  id: number
  isEnabled: YesNo
}

export interface RebuildAccessCacheResult {
  rebuiltUsers: number
}

export async function getMenus(query?: MenuListQuery): Promise<MenuCatalogResponse> {
  if (query !== undefined && !isPositiveInteger(query.platformId)) {
    throw new ProtocolError('menu platform query is invalid')
  }
  const raw =
    query === undefined
      ? await request<unknown>({ method: 'GET', url: '/api/admin/v1/menus' })
      : await request<unknown>({
          method: 'GET',
          url: '/api/admin/v1/menus',
          params: { platformId: query.platformId },
        })
  return parseMenuCatalog(raw, query?.platformId)
}

export async function createMenu(input: CreateMenuInput): Promise<MenuIDResult> {
  return expectId(
    await request<unknown>({ method: 'POST', url: '/api/admin/v1/menus', data: input }),
    'menu create result',
  )
}

export async function updateMenu(id: number, input: UpdateMenuInput): Promise<MenuIDResult> {
  return expectId(
    await request<unknown>({ method: 'PUT', url: `/api/admin/v1/menus/${id}`, data: input }),
    'menu update result',
  )
}

export async function updateMenuStatus(id: number, isEnabled: YesNo): Promise<MenuStatusResult> {
  const value = expectExactKeys(
    await request<unknown>({
      method: 'PATCH',
      url: `/api/admin/v1/menus/${id}/status`,
      data: { isEnabled },
    }),
    ['id', 'isEnabled'],
    'menu status result',
  )
  if (!isYesNo(value.isEnabled)) throw new ProtocolError('menu status result is invalid')
  return { id: expectInteger(value.id, 'menu status result.id'), isEnabled: value.isEnabled }
}

export async function deleteMenu(id: number): Promise<MenuIDResult> {
  return expectId(
    await request<unknown>({ method: 'DELETE', url: `/api/admin/v1/menus/${id}` }),
    'menu delete result',
  )
}

export async function rebuildAccessCache(): Promise<RebuildAccessCacheResult> {
  const value = expectExactKeys(
    await request<unknown>({
      method: 'POST',
      url: '/api/admin/v1/menus/access-cache/rebuild',
    }),
    ['rebuiltUsers'],
    'rebuild access cache result',
  )
  return {
    rebuiltUsers: expectInteger(value.rebuiltUsers, 'rebuild access cache result.rebuiltUsers'),
  }
}

const platformKeys = ['id', 'code', 'name', 'isEnabled'] as const
const menuNodeKeys = [
  'id',
  'platformId',
  'platformCode',
  'platformName',
  'parentId',
  'menuType',
  'name',
  'code',
  'i18nKey',
  'path',
  'componentPath',
  'icon',
  'remark',
  'sortOrder',
  'isEnabled',
  'isHidden',
  'isProtected',
  'createdAt',
  'updatedAt',
  'children',
] as const

function parseMenuCatalog(value: unknown, requestedPlatformID?: number): MenuCatalogResponse {
  if (
    !hasExactKeys(value, ['platforms', 'menuTree']) ||
    !Array.isArray(value.platforms) ||
    !Array.isArray(value.menuTree)
  ) {
    throw invalidMenuCatalog()
  }
  const platforms = value.platforms.map(parseMenuPlatform)
  if (platforms.length === 0) throw invalidMenuCatalog()
  const platformsByID = new Map<number, MenuPlatformOption>()
  for (const platform of platforms) {
    if (platformsByID.has(platform.id)) throw invalidMenuCatalog()
    platformsByID.set(platform.id, platform)
  }
  if (requestedPlatformID !== undefined && !platformsByID.has(requestedPlatformID))
    throw invalidMenuCatalog()
  const menuTree = value.menuTree.map((node) =>
    parseManagedMenuNode(node, platformsByID, null, requestedPlatformID),
  )
  return { platforms, menuTree }
}

function parseMenuPlatform(value: unknown): MenuPlatformOption {
  if (
    !hasExactKeys(value, platformKeys) ||
    !isPositiveInteger(value.id) ||
    !isNonEmptyString(value.code) ||
    !isNonEmptyString(value.name) ||
    !isYesNo(value.isEnabled)
  ) {
    throw invalidMenuCatalog()
  }
  return { id: value.id, code: value.code, name: value.name, isEnabled: value.isEnabled }
}

function parseManagedMenuNode(
  value: unknown,
  platformsByID: ReadonlyMap<number, MenuPlatformOption>,
  expectedParentID: number | null,
  requestedPlatformID?: number,
): ManagedMenuNode {
  if (!isManagedMenuNodeRecord(value) || value.parentId !== expectedParentID) {
    throw invalidMenuCatalog()
  }
  const platform = platformsByID.get(value.platformId)
  if (
    platform === undefined ||
    platform.code !== value.platformCode ||
    platform.name !== value.platformName ||
    (requestedPlatformID !== undefined && value.platformId !== requestedPlatformID)
  ) {
    throw invalidMenuCatalog()
  }
  return {
    id: value.id,
    platformId: value.platformId,
    platformCode: value.platformCode,
    platformName: value.platformName,
    parentId: value.parentId,
    menuType: value.menuType,
    name: value.name,
    code: value.code,
    i18nKey: value.i18nKey,
    path: value.path,
    componentPath: value.componentPath,
    icon: value.icon,
    remark: value.remark,
    sortOrder: value.sortOrder,
    isEnabled: value.isEnabled,
    isHidden: value.isHidden,
    isProtected: value.isProtected,
    createdAt: value.createdAt,
    updatedAt: value.updatedAt,
    children: value.children.map((child) =>
      parseManagedMenuNode(child, platformsByID, value.id, requestedPlatformID),
    ),
  }
}

type ManagedMenuNodeRecord = Omit<ManagedMenuNode, 'children'> & { children: unknown[] }

function isManagedMenuNodeRecord(value: unknown): value is ManagedMenuNodeRecord {
  return (
    hasExactKeys(value, menuNodeKeys) &&
    isPositiveInteger(value.id) &&
    isPositiveInteger(value.platformId) &&
    isNonEmptyString(value.platformCode) &&
    isNonEmptyString(value.platformName) &&
    isNullablePositiveInteger(value.parentId) &&
    isManagedMenuType(value.menuType) &&
    isNonEmptyString(value.name) &&
    isNonEmptyString(value.code) &&
    isNullableString(value.i18nKey) &&
    isNullableString(value.path) &&
    isNullableString(value.componentPath) &&
    isNullableMenuIcon(value.icon) &&
    isNullableString(value.remark) &&
    isNonNegativeInteger(value.sortOrder) &&
    isYesNo(value.isEnabled) &&
    isYesNo(value.isHidden) &&
    isYesNo(value.isProtected) &&
    isNonEmptyString(value.createdAt) &&
    isNonEmptyString(value.updatedAt) &&
    Array.isArray(value.children)
  )
}

function hasExactKeys<const Keys extends readonly string[]>(
  value: unknown,
  keys: Keys,
): value is Record<Keys[number], unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return false
  const actual = Object.keys(value)
  return (
    actual.length === keys.length &&
    keys.every((key) => Object.prototype.hasOwnProperty.call(value, key))
  )
}

function isPositiveInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value > 0
}

function isNonNegativeInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value >= 0
}

function isNullablePositiveInteger(value: unknown): value is number | null {
  return value === null || isPositiveInteger(value)
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === 'string' && value !== ''
}

function isNullableString(value: unknown): value is string | null {
  return value === null || typeof value === 'string'
}

function isNullableMenuIcon(value: unknown): value is MenuIconName | null {
  return value === null || (typeof value === 'string' && isMenuIcon(value))
}

function isManagedMenuType(value: unknown): value is ManagedMenuType {
  return value === 'directory' || value === 'page' || value === 'action'
}

function invalidMenuCatalog(): ProtocolError {
  return new ProtocolError('menu catalog response is invalid')
}
