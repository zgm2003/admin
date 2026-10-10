import type { UserListItem, UserRoleSummary } from '@/api/user/account'
import type { SearchField } from '@/components/AppSearch'
import type { TableColumn } from '@/components/AppTable'
import { YesNo } from '@/enums/yesNo'

type Translate = (key: string) => string
export interface UserSearchModel {
  keyword: string
  status: '' | YesNo
  role: '' | number
}

export function userTableColumns(t: Translate): TableColumn<UserListItem>[] {
  return [
    { prop: 'id', label: t('user.id'), width: 80 },
    { prop: 'username', label: t('user.username'), minWidth: 140 },
    { prop: 'email', label: t('user.email'), minWidth: 210 },
    { prop: 'phone', label: t('user.phone'), minWidth: 170 },
    { key: 'roles', prop: 'id', label: t('user.roles'), minWidth: 240 },
    { key: 'status', prop: 'id', label: t('user.status'), width: 100 },
    { prop: 'createdAt', label: t('user.createdAt'), minWidth: 190 },
    { prop: 'updatedAt', label: t('user.updatedAt'), minWidth: 190 },
    { key: 'actions', prop: 'id', label: t('user.actions'), width: 500 },
  ]
}

export function userSearchFields(
  t: Translate,
  roles: readonly UserRoleSummary[],
): SearchField<UserSearchModel>[] {
  return [
    {
      key: 'keyword',
      type: 'input',
      resetValue: '',
      label: t('user.keyword'),
      placeholder: t('user.keyword'),
      width: 280,
      testId: 'user-keyword',
    },
    {
      key: 'status',
      type: 'select-v2',
      resetValue: '',
      label: t('user.status'),
      placeholder: t('user.allStatus'),
      options: [
        { label: t('user.enabled'), value: YesNo.Yes },
        { label: t('user.disabled'), value: YesNo.No },
      ],
      width: 190,
    },
    {
      key: 'role',
      type: 'select-v2',
      resetValue: '',
      label: t('user.role'),
      placeholder: t('user.allRoles'),
      options: roles.map((role) => ({
        label: `${role.name} (${role.code})${role.isEnabled === YesNo.No ? ` · ${t('user.roleDisabled')}` : ''}`,
        value: role.id,
      })),
      width: 220,
    },
  ]
}

export function normalizedUsername(value: string): string {
  return value.trim()
}
