import type { PermissionMenuNode } from '@/api/permission/permission'

export interface RouteTab {
  path: string
  i18nKey: string
  affix: boolean
}

export interface ScrollbarHandle {
  wrapRef?: HTMLElement
  setScrollLeft: (value: number) => void
}

export interface RouteTabOperation {
  tabs: RouteTab[]
  nextPath?: string
}

export function findMenuPage(
  path: string,
  roots: readonly PermissionMenuNode[],
): PermissionMenuNode | null {
  const stack = [...roots].reverse()
  while (stack.length > 0) {
    const node = stack.pop()
    if (node === undefined) continue
    if (node.menuType === 'page' && node.path === path) return node
    if (node.menuType === 'directory') stack.push(...[...node.children].reverse())
  }
  return null
}

export function closeRouteTab(
  tabs: readonly RouteTab[],
  path: string,
  activePath: string,
): RouteTabOperation {
  const index = tabs.findIndex((tab) => tab.path === path)
  const tab = tabs[index]
  if (index < 0 || tab === undefined || tab.affix) return { tabs: [...tabs] }
  const nextTabs = [...tabs.slice(0, index), ...tabs.slice(index + 1)]
  if (activePath !== path) return { tabs: nextTabs }
  const destination = nextTabs[index - 1] ?? nextTabs[index] ?? nextTabs.find((item) => item.affix)
  return { tabs: nextTabs, nextPath: destination?.path ?? '/dashboard' }
}

export function closeOtherRouteTabs(
  tabs: readonly RouteTab[],
  path: string,
  activePath: string,
): RouteTabOperation {
  const selected = tabs.find((tab) => tab.path === path)
  const nextTabs = tabs.filter((tab) => tab.affix || tab.path === path)
  return {
    tabs: nextTabs,
    ...(selected !== undefined && activePath !== selected.path ? { nextPath: selected.path } : {}),
  }
}

export function closeAllRouteTabs(tabs: readonly RouteTab[]): RouteTabOperation {
  return { tabs: tabs.filter((tab) => tab.affix), nextPath: '/dashboard' }
}
