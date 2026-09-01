import type { Component } from 'vue'
import type { Router } from 'vue-router'

import type { PermissionMenuNode } from '../api/permission/permission'
import { ProtocolError } from '../types/http'

export interface PageModule {
  default: Component
}

export type PageModuleLoader = () => Promise<PageModule>
export type PageModuleMap = Readonly<Record<string, PageModuleLoader>>

interface PageRoute {
  path: string
  name: string
  component: PageModuleLoader
  i18nKey: string
}

const staticPageBinding = {
	code: 'permission:menu:view',
	path: '/access/menus',
	componentPath: 'access/menus',
	routeName: 'access-menus',
} as const

const pageModules: PageModuleMap = import.meta.glob<PageModule>('../views/**/index.vue')

const componentPathMap: Readonly<Record<string, string>> = {
	'account/users': 'account/users',
	'account/profile': 'account/profile',
	'account/sessions': 'account/sessions',
	'user/login-logs': 'account/login-logs',
	'access/menus': 'permission/menus',
	'access/roles': 'permission/roles',
	'access/auth-platforms': 'permission/auth-platforms',
	'system/operation-logs': 'system/operation-logs',
	'storage/object': 'cloud/storage-object',
}

export function registerPermissionRoutes(
  router: Router,
  menuTree: readonly PermissionMenuNode[],
  views: PageModuleMap = pageModules,
): () => void {
  if (!router.hasRoute('admin-layout')) {
    throw new ProtocolError('admin-layout route is required before access routes')
  }

  const pages: PageRoute[] = []
  const paths = new Set<string>()
  const names = new Set<string>()
  const existingRoutes = router.getRoutes()
  const existingPaths = new Set(existingRoutes.map((route) => route.path))
  const existingNames = new Set(existingRoutes.flatMap((route) => route.name === undefined ? [] : [String(route.name)]))

	collectPages(menuTree, router, views, pages, paths, names, existingPaths, existingNames)

  const removers: Array<() => void> = []
  try {
    for (const page of pages) {
      const remove = router.addRoute('admin-layout', {
        path: page.path,
        name: page.name,
        component: page.component,
        meta: { requiresAuth: true, i18nKey: page.i18nKey },
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
  nodes: readonly PermissionMenuNode[],
	router: Router,
  views: PageModuleMap,
  pages: PageRoute[],
  paths: Set<string>,
  names: Set<string>,
  existingPaths: Set<string>,
  existingNames: Set<string>,
): void {
  for (const node of nodes) {
    if (node.menuType === 'directory') {
			collectPages(node.children, router, views, pages, paths, names, existingPaths, existingNames)
      continue
    }
    if (node.path === null || node.componentPath === null) {
      throw new ProtocolError(`access page ${node.code} is incomplete`)
    }
		if (isStaticBindingCandidate(node)) {
			validateStaticBinding(router, node, paths, names)
			continue
		}
    const key = moduleKey(node.componentPath)
    if (!Object.prototype.hasOwnProperty.call(views, key)) {
      throw new ProtocolError(`access page ${node.code} has an unknown componentPath`)
    }
    const name = `access:${node.code}`
    if (paths.has(node.path) || existingPaths.has(node.path)) {
      throw new ProtocolError(`access page path ${node.path} is duplicated`)
    }
    if (names.has(name) || existingNames.has(name)) {
      throw new ProtocolError(`access route name ${name} is duplicated`)
    }
    paths.add(node.path)
    names.add(name)
    pages.push({ path: node.path, name, component: views[key], i18nKey: node.i18nKey })
  }
}

function isStaticBindingCandidate(node: PermissionMenuNode): boolean {
	return node.code === staticPageBinding.code
		|| node.path === staticPageBinding.path
		|| node.componentPath === staticPageBinding.componentPath
}

function validateStaticBinding(
	router: Router,
	node: PermissionMenuNode,
	paths: Set<string>,
	names: Set<string>,
): void {
	if (
		node.code !== staticPageBinding.code
		|| node.path !== staticPageBinding.path
		|| node.componentPath !== staticPageBinding.componentPath
	) {
		throw new ProtocolError('static menu page binding does not match the access protocol')
	}
	const route = router.getRoutes().find((record) => record.path === staticPageBinding.path)
	if (route === undefined || route.name !== staticPageBinding.routeName) {
		throw new ProtocolError('static menu page route is missing or incorrectly named')
	}
	if (paths.has(staticPageBinding.path) || names.has(staticPageBinding.routeName)) {
		throw new ProtocolError('static menu page binding is duplicated')
	}
	paths.add(staticPageBinding.path)
	names.add(staticPageBinding.routeName)
}

function moduleKey(componentPath: string): string {
  const mappedPath = componentPathMap[componentPath] ?? componentPath
  return `../views/${mappedPath}/index.vue`
}

function removeRoutes(removers: Array<() => void>): void {
  for (let index = removers.length - 1; index >= 0; index -= 1) {
    removers[index]()
  }
}
