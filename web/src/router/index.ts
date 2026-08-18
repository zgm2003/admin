import {
  createRouter,
  createWebHistory,
  type RouterHistory,
  type RouteRecordRaw,
} from 'vue-router'

import Dashboard from '../views/dashboard/index.vue'
import type { AppMessageKey } from '../i18n/messages/zh-CN'

declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth: boolean
    titleKey?: AppMessageKey
    affix?: boolean
  }
}

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('../views/auth/login/index.vue'),
    meta: { requiresAuth: false },
  },
  {
    path: '/',
    component: () => import('../layout/index.vue'),
    redirect: '/dashboard',
    meta: { requiresAuth: true },
    children: [
      {
        path: 'dashboard',
        name: 'dashboard',
        component: Dashboard,
        meta: { requiresAuth: true, titleKey: 'navigation.dashboard', affix: true },
      },
    ],
  },
]

export function createAppRouter(history: RouterHistory = createWebHistory()) {
  return createRouter({ history, routes })
}

export const router = createAppRouter()
