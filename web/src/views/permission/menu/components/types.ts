import type { ManagedMenuType } from '@/api/permission/menu'
import type { YesNo } from '@/enums/yes-no'
import type { MenuIconName } from '@/icons/menu-icons'

export interface MenuFormState {
  parentId: number | null
  menuType: ManagedMenuType
  name: string
  code: string
  i18nKey: string
  path: string | null
  componentPath: string | null
  icon: MenuIconName | null
  remark: string
  sortOrder: number
  isEnabled: YesNo
  isHidden: YesNo
  isProtected: YesNo
}
