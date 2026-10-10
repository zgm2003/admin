import { request } from '@/utils/request'
import { type YesNo } from '@/enums/yesNo'
import { type MenuIconName } from '@/icons/menuIcons'

export type ManagedMenuType = 'directory' | 'page' | 'action'

export interface MenuPlatformOption {
  id: number
  code: string
  name: string
  isEnabled: YesNo
}

export interface MenuPresentation {
  typeLabel: string
  typeTone: 'primary' | 'success' | 'warning' | 'danger' | 'info'
  visibilityLabel: string
  visibilityTone: 'primary' | 'success' | 'warning' | 'danger' | 'info'
  statusLabel: string
  statusTone: 'primary' | 'success' | 'warning' | 'danger' | 'info'
  statusActionLabel: string
  protectionReason: string
  statusReason: string
}
export interface MenuActions {
  update: boolean
  status: boolean
  delete: boolean
  addChild: boolean
  allowedChildTypes: ManagedMenuType[]
}
export interface ManagedMenuNode {
  presentation: MenuPresentation
  actions: MenuActions
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
  allowedRootTypes: ManagedMenuType[]
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
  rebuiltPlatforms: number
}

export async function getMenus(query?: MenuListQuery): Promise<MenuCatalogResponse> {
  return query === undefined
    ? request.get<MenuCatalogResponse>('/api/admin/v1/permission/menu')
    : request.get<MenuCatalogResponse>('/api/admin/v1/permission/menu', {
        params: { platformId: query.platformId },
      })
}

export async function createMenu(input: CreateMenuInput): Promise<MenuIDResult> {
  return request.post<MenuIDResult>('/api/admin/v1/permission/menu', input)
}

export async function updateMenu(id: number, input: UpdateMenuInput): Promise<MenuIDResult> {
  return request.put<MenuIDResult>(`/api/admin/v1/permission/menu/${id}`, input)
}

export async function updateMenuStatus(id: number, isEnabled: YesNo): Promise<MenuStatusResult> {
  return request.patch<MenuStatusResult>(`/api/admin/v1/permission/menu/${id}/status`, {
    isEnabled,
  })
}

export async function deleteMenu(id: number): Promise<MenuIDResult> {
  return request.delete<MenuIDResult>(`/api/admin/v1/permission/menu/${id}`)
}

export async function rebuildAccessCache(): Promise<RebuildAccessCacheResult> {
  return request.post<RebuildAccessCacheResult>(
    '/api/admin/v1/permission/menu/access-cache/rebuild',
    undefined,
  )
}
