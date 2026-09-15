import type { Router, RouteLocationNormalized } from 'vue-router'

const recoveryStorageKey = 'admin:route-load-recovery'
const routeLoadErrorPatterns = [
  /Failed to fetch dynamically imported module/i,
  /Importing a module script failed/i,
  /error loading dynamically imported module/i,
  /Loading chunk \S+ failed/i,
  /ChunkLoadError/i,
]

interface RouteLoadRecoveryEnvironment {
  storage: Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>
  reload: () => void
}

export function installRouteLoadRecovery(
  router: Router,
  environment: RouteLoadRecoveryEnvironment = {
    storage: window.sessionStorage,
    reload: () => window.location.reload(),
  },
): void {
  router.onError((error, to) => {
    if (!isRouteLoadError(error)) return
    reloadOnce(to, environment)
  })

  router.afterEach((_to, _from, failure) => {
    if (failure !== undefined) return
    environment.storage.removeItem(recoveryStorageKey)
  })
}

function isRouteLoadError(error: unknown): boolean {
  const message = error instanceof Error ? error.message : String(error)
  return routeLoadErrorPatterns.some((pattern) => pattern.test(message))
}

function reloadOnce(
  target: RouteLocationNormalized,
  environment: RouteLoadRecoveryEnvironment,
): void {
  try {
    if (environment.storage.getItem(recoveryStorageKey) === target.fullPath) return
    environment.storage.setItem(recoveryStorageKey, target.fullPath)
  } catch {
    return
  }
  environment.reload()
}
