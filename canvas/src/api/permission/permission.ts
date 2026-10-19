import type { RequestClient } from '@/utils/request'

export interface PermissionMenuNode {
  code: string
  menuType: 'directory' | 'page'
  path: string | null
  componentPath: string | null
  i18nKey: string
  icon: string | null
  isHidden: 0 | 1
  children: PermissionMenuNode[]
}
export interface PermissionSnapshot { roleCodes: string[]; permissionCodes: string[]; menuTree: PermissionMenuNode[] }
export const getPermission = (client: RequestClient): Promise<unknown> => client.get<unknown>('/api/canvas/v1/access')
