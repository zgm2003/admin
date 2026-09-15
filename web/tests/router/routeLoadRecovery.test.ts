import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it, vi } from 'vitest'

import { installRouteLoadRecovery } from '@/router/routeLoadRecovery'

describe('route module load recovery', () => {
  it('reloads once when a lazy route module cannot be fetched', async () => {
    const storage = createStorage()
    const reload = vi.fn()
    const router = createFailingRouter(
      new TypeError(
        'Failed to fetch dynamically imported module: http://localhost:16300/src/layout/index.vue',
      ),
    )
    installRouteLoadRecovery(router, { storage, reload })

    await expect(router.push('/lazy')).rejects.toThrow(
      'Failed to fetch dynamically imported module',
    )
    expect(reload).toHaveBeenCalledOnce()

    await expect(router.push('/lazy')).rejects.toThrow(
      'Failed to fetch dynamically imported module',
    )
    expect(reload).toHaveBeenCalledOnce()
  })

  it('does not reload for an application error raised during navigation', async () => {
    const reload = vi.fn()
    const router = createFailingRouter(new Error('permission contract is invalid'))
    installRouteLoadRecovery(router, { storage: createStorage(), reload })

    await expect(router.push('/lazy')).rejects.toThrow('permission contract is invalid')
    expect(reload).not.toHaveBeenCalled()
  })
})

function createFailingRouter(error: Error) {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
      {
        path: '/lazy',
        component: () => Promise.reject(error),
      },
    ],
  })
}

function createStorage(): Pick<Storage, 'getItem' | 'setItem' | 'removeItem'> {
  const values = new Map<string, string>()
  return {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, value),
    removeItem: (key) => values.delete(key),
  }
}
