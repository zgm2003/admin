import { request } from '../utils/request'
import type { YesNo } from '../enums/yes-no'

export type MenuType = 'directory' | 'page'
export interface AccessMenuNode { code: string; menuType: MenuType; path: string | null; componentPath: string | null; i18nKey: string; icon: string | null; isHidden: YesNo; children: AccessMenuNode[] }
export interface AccessSnapshot { roleCodes: string[]; menuTree: AccessMenuNode[]; permissionCodes: string[] }

export function getAccess(): Promise<AccessSnapshot> {
  return request<AccessSnapshot>({ method: 'GET', url: '/api/v1/access' })
}
