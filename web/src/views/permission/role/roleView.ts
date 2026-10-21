import type { RoleListItem } from '@/api/permission/role'
import type { SearchField } from '@/components/AppSearch'
import type { TableColumn } from '@/components/AppTable'
import { YesNo } from '@/enums/yesNo'

type Translate = (key: string) => string
export interface RoleSearchModel {
  keyword: string
  status: '' | YesNo
}

export function roleSearchFields(t: Translate): SearchField<RoleSearchModel>[] {
  return [
    {
      key: 'keyword',
      type: 'input',
      resetValue: '',
      label: t('role.keyword'),
      placeholder: t('role.keyword'),
      width: 260,
      testId: 'role-keyword',
    },
    {
      key: 'status',
      type: 'select-v2',
      resetValue: '',
      label: t('common.status'),
      placeholder: t('common.allStatuses'),
      options: [
        { label: t('common.enable'), value: YesNo.Yes },
        { label: t('common.disable'), value: YesNo.No },
      ],
      width: 160,
    },
  ]
}

export function roleTableColumns(t: Translate): TableColumn<RoleListItem>[] {
  return [
    { prop: 'name', label: t('role.column.name'), minWidth: 150 },
    { prop: 'code', label: t('role.column.code'), minWidth: 170 },
    { key: 'default', prop: 'id', label: t('role.column.default'), width: 100 },
    { key: 'status', prop: 'id', label: t('common.status'), width: 100 },
    { prop: 'userCount', label: t('role.column.users'), width: 100 },
    { prop: 'permissionCount', label: t('role.column.permissions'), width: 130 },
    { prop: 'createdAt', label: t('common.createdAt'), minWidth: 190 },
    { prop: 'updatedAt', label: t('common.updatedAt'), minWidth: 190 },
    { key: 'actions', prop: 'id', label: t('common.actions'), width: 360 },
  ]
}
