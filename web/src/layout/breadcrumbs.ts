import type { AccessMenuNode } from '../api/access.contract'
import type { AppMessageKey } from '../i18n'

export interface HeaderBreadcrumb {
  path: string | null
  titleKey: AppMessageKey
}

interface SearchEntry {
  node: AccessMenuNode
  ancestors: readonly HeaderBreadcrumb[]
}

export function resolveBreadcrumbs(
  routePath: string,
  menuTree: readonly AccessMenuNode[],
): HeaderBreadcrumb[] | null {
  if (routePath === '/dashboard') {
    return [{ path: '/dashboard', titleKey: 'navigation.dashboard' }]
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
      return [...ancestors, { path: node.path, titleKey: node.titleKey }]
    }

    if (node.menuType !== 'directory') continue
    const nextAncestors = [...ancestors, { path: null, titleKey: node.titleKey }]
    for (const child of node.children.slice().reverse()) {
      stack.push({ node: child, ancestors: nextAncestors })
    }
  }

  return null
}
