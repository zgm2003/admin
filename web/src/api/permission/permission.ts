import { request } from '../../utils/request'
import type { YesNo } from '../../enums/yes-no'

export type MenuType = 'directory' | 'page'
export interface PermissionMenuNode { code: string; menuType: MenuType; path: string | null; componentPath: string | null; i18nKey: string; icon: string | null; isHidden: YesNo; children: PermissionMenuNode[] }
export interface PermissionSnapshot { roleCodes: string[]; menuTree: PermissionMenuNode[]; permissionCodes: string[] }

export function getPermission(): Promise<PermissionSnapshot> {
  return request<PermissionSnapshot>({ method: 'GET', url: '/api/v1/access' })
}
