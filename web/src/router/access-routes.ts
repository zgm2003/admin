import type { Router } from 'vue-router'

import { routeViews, type RouteViewLoader, type RouteViewMap } from '../access/route-views'
import type { AccessMenuNode } from '../api/access.contract'
import type { AppMessageKey } from '../i18n'
import { ProtocolError } from '../types/http'

interface PageRoute {
  path: string
  name: string
  component: RouteViewLoader
  titleKey: AppMessageKey
}

export function registerAccessRoutes(
  router: Router,
  menuTree: readonly AccessMenuNode[],
  views: RouteViewMap = routeViews,
): () => void {
  if (!router.hasRoute('admin-layout')) {
    throw new ProtocolError('admin-layout route is required before access routes')
  }

  const pages: PageRoute[] = []
  const paths = new Set<string>()
  const names = new Set<string>()
  const existingPaths = new Set(router.getRoutes().map((route) => route.path))

  collectPages(menuTree, views, pages, paths, names, existingPaths)

  const removers: Array<() => void> = []
  try {
    for (const page of pages) {
      const remove = router.addRoute('admin-layout', {
        path: page.path,
        name: page.name,
        component: page.component,
        meta: { requiresAuth: true, titleKey: page.titleKey },
      })
      removers.push(remove)
    }
  } catch (error: unknown) {
    removeRoutes(removers)
    throw error
  }

  let cleaned = false
  return () => {
    if (cleaned) return
    cleaned = true
    removeRoutes(removers)
  }
}

function collectPages(
  nodes: readonly AccessMenuNode[],
  views: RouteViewMap,
  pages: PageRoute[],
  paths: Set<string>,
  names: Set<string>,
  existingPaths: Set<string>,
): void {
  for (const node of nodes) {
    if (node.menuType === 'directory') {
      collectPages(node.children, views, pages, paths, names, existingPaths)
      continue
    }
    if (node.path === null || !node.path.startsWith('/') || node.path === '/') {
      throw new ProtocolError(`access page ${node.code} must use an absolute non-root path`)
    }
    if (node.viewKey === null || !Object.prototype.hasOwnProperty.call(views, node.viewKey)) {
      throw new ProtocolError(`access page ${node.code} has an unknown viewKey`)
    }
    const name = `access:${node.code}`
    if (paths.has(node.path) || existingPaths.has(node.path)) {
      throw new ProtocolError(`access page path ${node.path} is duplicated`)
    }
    if (names.has(name)) {
      throw new ProtocolError(`access route name ${name} is duplicated`)
    }
    paths.add(node.path)
    names.add(name)
    pages.push({ path: node.path, name, component: views[node.viewKey], titleKey: node.titleKey })
  }
}

function removeRoutes(removers: Array<() => void>): void {
  for (let index = removers.length - 1; index >= 0; index -= 1) {
    removers[index]()
  }
}
