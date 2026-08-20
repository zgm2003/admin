import type { AppMessageKey } from '../i18n'

export const menuTitleKeys = [
  'navigation.system',
  'navigation.systemMenus',
  'permission.menuCreate',
  'permission.menuUpdate',
  'permission.menuDelete',
  'navigation.systemRoles',
  'permission.roleCreate',
  'permission.roleUpdate',
  'permission.roleStatus',
  'permission.roleSetDefault',
  'permission.roleDelete',
  'permission.roleAuthorize',
	'navigation.systemUsers',
	'permission.userUpdate',
	'permission.userStatus',
	'permission.userDelete',
	'permission.userRoles',
] as const satisfies readonly AppMessageKey[]

export type MenuTitleKey = (typeof menuTitleKeys)[number]

const menuTitleKeySet: ReadonlySet<string> = new Set(menuTitleKeys)

export function isMenuTitleKey(value: string): value is MenuTitleKey {
  return menuTitleKeySet.has(value)
}
