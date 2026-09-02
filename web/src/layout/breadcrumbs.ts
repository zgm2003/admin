import type { PermissionMenuNode } from '@/api/permission/permission'

export interface HeaderBreadcrumb {
  path: string | null
  i18nKey: string
}

interface SearchEntry {
  node: PermissionMenuNode
  ancestors: readonly HeaderBreadcrumb[]
}

export function resolveBreadcrumbs(
  routePath: string,
  menuTree: readonly PermissionMenuNode[],
): HeaderBreadcrumb[] | null {
  if (routePath === '/dashboard') {
    return [{ path: '/dashboard', i18nKey: 'navigation.dashboard' }]
  }
  const stack: SearchEntry[] = menuTree
    .slice()
    .reverse()
    .map((node) => ({ node, ancestors: [] }))

  while (stack.length > 0) {
    const entry = stack.pop()
    if (entry === undefined) continue
    const { node, ancestors } = entry

    if (node.menuType === 'page' && node.path === routePath) {
      return [...ancestors, { path: node.path, i18nKey: node.i18nKey }]
    }

    if (node.menuType !== 'directory') continue
    const nextAncestors = [...ancestors, { path: null, i18nKey: node.i18nKey }]
    for (const child of node.children.slice().reverse()) {
      stack.push({ node: child, ancestors: nextAncestors })
    }
  }

  return null
}
