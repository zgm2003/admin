import {
  isComponentPath,
  isMenuI18nKey,
  isMenuIcon,
  isMenuPath,
  menuCodePattern,
} from '@/api/permission/menu'
import type {
  CreateMenuInput,
  ManagedMenuNode,
  ManagedMenuType,
  UpdateMenuInput,
} from '@/api/permission/menu'
import { YesNo } from '@/enums/yes-no'
import type { MenuFormState } from './components/types'

export type MenuCodeError = 'page-code-suffix' | 'action-code-suffix' | null

export function createMenuForm(parent: ManagedMenuNode | null = null): MenuFormState {
  const menuType: ManagedMenuType =
    parent === null ? 'directory' : parent.menuType === 'directory' ? 'page' : 'action'
  return {
    parentId: parent?.id ?? null,
    menuType,
    name: '',
    code: '',
    i18nKey: menuType === 'action' ? '' : 'navigation.system',
    path: menuType === 'page' ? '' : null,
    componentPath: menuType === 'page' ? '' : null,
    icon: null,
    remark: '',
    sortOrder: 100,
    isEnabled: YesNo.Yes,
    isHidden: menuType === 'action' ? YesNo.Yes : YesNo.No,
    isProtected: YesNo.No,
  }
}

export function editMenuForm(node: ManagedMenuNode): MenuFormState {
  return {
    parentId: node.parentId,
    menuType: node.menuType,
    name: node.name,
    code: node.code,
    i18nKey: node.i18nKey ?? '',
    path: node.path,
    componentPath: node.componentPath,
    icon: node.icon,
    remark: node.remark ?? '',
    sortOrder: node.sortOrder,
    isEnabled: node.isEnabled,
    isHidden: node.isHidden,
    isProtected: node.isProtected,
  }
}

export function changeMenuFormType(
  current: MenuFormState,
  nextType: ManagedMenuType,
  validParentIDs: ReadonlySet<number>,
): MenuFormState {
  const next = { ...current, menuType: nextType }
  if (current.menuType === 'action' && nextType !== 'action') {
    next.isHidden = YesNo.No
    next.i18nKey = ''
  }
  if (nextType === 'directory') {
    next.path = null
    next.componentPath = null
  } else if (nextType === 'page') {
    next.path = next.path ?? ''
    next.componentPath = next.componentPath ?? ''
  } else {
    next.path = null
    next.componentPath = null
    next.icon = null
    next.isHidden = YesNo.Yes
    next.i18nKey = ''
  }
  if (next.parentId !== null && !validParentIDs.has(next.parentId)) next.parentId = null
  return next
}

export function menuCodeError(form: MenuFormState): MenuCodeError {
  if (form.menuType === 'page' && !form.code.endsWith(':view')) return 'page-code-suffix'
  if (form.menuType === 'action' && form.code.endsWith(':view')) return 'action-code-suffix'
  return null
}

export function isMenuFormSubmittable(form: MenuFormState): boolean {
  if (form.name === '' || form.name.trim() !== form.name || form.name.length > 128) return false
  if (form.code.length > 128 || !menuCodePattern.test(form.code)) return false
  if (form.menuType !== 'action' && !isMenuI18nKey(form.i18nKey)) return false
  if (form.icon !== null && !isMenuIcon(form.icon)) return false
  if (form.menuType === 'page') {
    return (
      form.path !== null &&
      isMenuPath(form.path) &&
      form.componentPath !== null &&
      isComponentPath(form.componentPath)
    )
  }
  if (form.menuType === 'directory') return form.path === null && form.componentPath === null
  return (
    form.path === null &&
    form.componentPath === null &&
    form.icon === null &&
    form.isHidden === YesNo.Yes
  )
}

function normalizedRemark(form: MenuFormState): string | null {
  const remark = form.remark.trim()
  return remark === '' ? null : remark
}

export function createMenuInput(platformId: number, form: MenuFormState): CreateMenuInput {
  return {
    platformId,
    parentId: form.parentId,
    menuType: form.menuType,
    name: form.name,
    code: form.code,
    i18nKey: form.menuType === 'action' ? null : form.i18nKey,
    path: form.path,
    componentPath: form.componentPath,
    icon: form.icon,
    remark: normalizedRemark(form),
    sortOrder: form.sortOrder,
    isEnabled: form.isEnabled,
    isHidden: form.isHidden,
  }
}

export function updateMenuInput(form: MenuFormState): UpdateMenuInput {
  return {
    parentId: form.parentId,
    menuType: form.menuType,
    name: form.name,
    i18nKey: form.menuType === 'action' ? null : form.i18nKey,
    path: form.path,
    componentPath: form.componentPath,
    icon: form.icon,
    remark: normalizedRemark(form),
    sortOrder: form.sortOrder,
    isHidden: form.isHidden,
  }
}
