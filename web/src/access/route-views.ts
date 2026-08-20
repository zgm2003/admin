import type { Component } from 'vue'

export type RouteViewLoader = () => Promise<Component>
export type RouteViewMap = Readonly<Record<string, RouteViewLoader>>

export const routeViews = {
  'system-menus': () => import('../views/system/menus/index.vue').then((module) => module.default),
  'system-roles': () => import('../views/system/roles/index.vue').then((module) => module.default),
	'system-users': () => import('../views/system/users/index.vue').then((module) => module.default),
} as const satisfies RouteViewMap

export type RouteViewKey = keyof typeof routeViews
