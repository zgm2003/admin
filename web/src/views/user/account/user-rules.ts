import type { UserListItem, UserRolesResponse, UserRoleSummary } from '@/api/user/account'
import type { SearchField } from '@/components/AppSearch'
import type { TableColumn } from '@/components/AppTable'
import { YesNo } from '@/enums/yes-no'

type Translate = (key: string) => string

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
    { key: 'actions', prop: 'id', label: t('user.actions'), width: 330 },
  ]
}

export function userSearchFields(t: Translate, roles: readonly UserRoleSummary[]): SearchField[] {
  return [
    {
      key: 'keyword',
      type: 'input',
      label: t('user.keyword'),
      placeholder: t('user.keyword'),
      width: 280,
      testId: 'user-keyword',
    },
    {
      key: 'status',
      type: 'select-v2',
      label: t('user.status'),
      options: [
        { label: t('user.status'), value: '' },
        { label: t('user.enabled'), value: YesNo.Yes },
        { label: t('user.disabled'), value: YesNo.No },
      ],
      width: 190,
    },
    {
      key: 'role',
      type: 'select-v2',
      label: t('user.role'),
      options: [
        { label: t('user.role'), value: '' },
        ...roles.map((role) => ({
          label: `${role.name} (${role.code})${role.isEnabled === YesNo.No ? ` · ${t('user.roleDisabled')}` : ''}`,
          value: role.id,
        })),
      ],
      width: 220,
    },
  ]
}

export function normalizedUsername(value: string): string {
  return value.trim()
}

export function normalizedPhone(value: string): string {
  return value.trim()
}

export function isUsernameValid(value: string): boolean {
  const normalized = normalizedUsername(value)
  const characters = [...normalized]
  return (
    characters.length >= 3 &&
    characters.length <= 64 &&
    characters.every((character) => /[\p{L}\p{N}_-]/u.test(character))
  )
}

export function isPhoneValid(value: string): boolean {
  const normalized = normalizedPhone(value)
  return normalized === '' || ([...normalized].length <= 32 && !/\p{Cc}/u.test(normalized))
}

export function hasSuperAdminRole(user: UserListItem): boolean {
  return user.roles.some((role) => role.code === 'super_admin')
}

export function isProtectedTarget(user: UserListItem, actorIsSuperAdmin: boolean): boolean {
  return hasSuperAdminRole(user) && !actorIsSuperAdmin
}

export function protectedRoleIDs(
  data: UserRolesResponse | null,
  actorIsSuperAdmin: boolean,
): number[] {
  if (actorIsSuperAdmin || data === null) return []
  const role = data.roles.find((item) => item.code === 'super_admin')
  return role !== undefined && data.roleIds.includes(role.id) ? [role.id] : []
}

export function isRoleToggleDisabled(role: UserRoleSummary, actorIsSuperAdmin: boolean): boolean {
  return role.code === 'super_admin' && !actorIsSuperAdmin
}
