import {
  createRouter,
  createWebHistory,
  type RouterHistory,
  type RouteRecordRaw,
} from 'vue-router'

import AdminLayout from '../layout/AdminLayout.vue'
import Dashboard from '../views/dashboard/index.vue'

declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth: boolean
  }
}

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    component: AdminLayout,
    redirect: '/dashboard',
    meta: { requiresAuth: false },
    children: [
      {
        path: 'dashboard',
        name: 'dashboard',
        component: Dashboard,
        meta: { requiresAuth: false },
      },
    ],
  },
]

export function createAppRouter(history: RouterHistory = createWebHistory()) {
  return createRouter({ history, routes })
}

export const router = createAppRouter()
