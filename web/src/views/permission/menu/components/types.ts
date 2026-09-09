import type { ManagedMenuType } from '@/api/permission/menu'
import type { YesNo } from '@/enums/yesNo'
import type { MenuIconName } from '@/icons/menuIcons'

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
