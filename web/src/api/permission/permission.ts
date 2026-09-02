import { request } from '@/utils/request'
import { isYesNo, type YesNo } from '@/enums/yes-no'
import { expectArray, expectRecord, expectString } from '@/api/protocol'
import { ProtocolError } from '@/types/http'

export type MenuType = 'directory' | 'page'
export interface PermissionMenuNode {
  code: string
  menuType: MenuType
  path: string | null
  componentPath: string | null
  i18nKey: string
  icon: string | null
  isHidden: YesNo
  children: PermissionMenuNode[]
}
export interface PermissionSnapshot {
  roleCodes: string[]
  menuTree: PermissionMenuNode[]
  permissionCodes: string[]
}

export async function getPermission(): Promise<PermissionSnapshot> {
  return parsePermission(await request<unknown>({ method: 'GET', url: '/api/v1/access' }))
}

function parsePermission(value: unknown): PermissionSnapshot {
  const item = expectRecord(value, 'permission snapshot')
  const roleCodes = expectArray(item.roleCodes, 'permission roleCodes').map((code, index) =>
    expectString(code, `permission roleCodes[${index}]`),
  )
  const permissionCodes = expectArray(item.permissionCodes, 'permission permissionCodes').map(
    (code, index) => expectString(code, `permission permissionCodes[${index}]`),
  )
  const menuTree = expectArray(item.menuTree, 'permission menuTree').map((node) => parseNode(node))
  return { roleCodes, permissionCodes, menuTree }
}

function parseNode(value: unknown): PermissionMenuNode {
  const item = expectRecord(value, 'permission menu node')
  const menuType = item.menuType
  const isHidden = item.isHidden
  if (menuType !== 'directory' && menuType !== 'page')
    throw new ProtocolError('permission menu type is invalid')
  if (!isYesNo(isHidden)) throw new ProtocolError('permission menu hidden flag is invalid')
  return {
    code: expectString(item.code, 'permission menu code'),
    menuType,
    path:
      item.path === null || typeof item.path === 'string'
        ? item.path
        : (() => {
            throw new ProtocolError('permission menu path is invalid')
          })(),
    componentPath:
      item.componentPath === null || typeof item.componentPath === 'string'
        ? item.componentPath
        : (() => {
            throw new ProtocolError('permission menu componentPath is invalid')
          })(),
    i18nKey: expectString(item.i18nKey, 'permission menu i18nKey'),
    icon:
      item.icon === null || typeof item.icon === 'string'
        ? item.icon
        : (() => {
            throw new ProtocolError('permission menu icon is invalid')
          })(),
    isHidden,
    children: expectArray(item.children, 'permission menu children').map(parseNode),
  }
}
