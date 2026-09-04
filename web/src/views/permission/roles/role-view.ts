import type { RoleListItem } from '@/api/permission/role'
import type { SearchField } from '@/components/AppSearch'
import type { TableColumn } from '@/components/AppTable'
import { YesNo } from '@/enums/yes-no'

type Translate = (key: string) => string

export function roleSearchFields(t: Translate): SearchField[] {
  return [
    {
      key: 'keyword',
      type: 'input',
      label: t('role.keyword'),
      placeholder: t('role.keyword'),
      width: 260,
      testId: 'role-keyword',
    },
    {
      key: 'status',
      type: 'select-v2',
      label: t('role.status.all'),
      options: [
        { label: t('role.status.all'), value: '' },
        { label: t('role.status.enabled'), value: YesNo.Yes },
        { label: t('role.status.disabled'), value: YesNo.No },
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
    { key: 'status', prop: 'id', label: t('role.column.status'), width: 100 },
    { prop: 'userCount', label: t('role.column.users'), width: 100 },
    { prop: 'permissionCount', label: t('role.column.permissions'), width: 130 },
    { prop: 'createdAt', label: t('role.column.createdAt'), minWidth: 190 },
    { prop: 'updatedAt', label: t('role.column.updatedAt'), minWidth: 190 },
    { key: 'actions', prop: 'id', label: t('role.column.actions'), width: 360 },
  ]
}

