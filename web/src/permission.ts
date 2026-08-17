import type { Router } from 'vue-router'

export function installPermissionGuard(router: Router): void {
  router.beforeEach((to) => {
    if (to.matched.some((route) => typeof route.meta.requiresAuth !== 'boolean')) {
      throw new Error(`Route ${to.fullPath} must declare requiresAuth`)
    }
    return true
  })
}
