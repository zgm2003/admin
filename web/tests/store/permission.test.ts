import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getPermission } from '@/api/permission/permission'
import type { PermissionSnapshot } from '@/api/permission/permission'
import { setLocale } from '@/i18n'
import { ApiError, ProtocolError } from '@/types/http'
import { usePermissionStore } from '@/store/permission'
import { YesNo } from '@/enums/yes-no'

vi.mock('@/api/permission/permission', () => ({ getPermission: vi.fn() }))

const getPermissionMock = vi.mocked(getPermission)

describe('access store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getPermissionMock.mockReset()
    setLocale('zh-CN')
  })

  it('applies a snapshot and checks button permissions', () => {
    const store = usePermissionStore()
    const snapshot = emptySnapshot()
    snapshot.roleCodes.push('registered_user')
    snapshot.permissionCodes.push('account:user:create')

    store.applySnapshot(snapshot)

    expect(store.status).toBe('ready')
    expect(store.roleCodes).toEqual(['registered_user'])
    expect(store.hasPermission('account:user:create')).toBe(true)
    expect(store.hasPermission('account:user:delete')).toBe(false)
  })

  it('shares one in-flight request across concurrent loads', async () => {
    const request = deferred<PermissionSnapshot>()
    getPermissionMock.mockReturnValue(request.promise)
    const store = usePermissionStore()

    const first = store.load()
    const second = store.load()

    expect(getPermissionMock).toHaveBeenCalledOnce()
    request.resolve(emptySnapshot())

    await Promise.all([first, second])
    expect(store.status).toBe('ready')
  })

  it('does not restore access from a response that resolves after reset', async () => {
    const request = deferred<PermissionSnapshot>()
    getPermissionMock.mockReturnValue(request.promise)
    const store = usePermissionStore()

    const pending = store.load()
    store.reset()
    request.resolve({ roleCodes: ['old_user'], menuTree: [], permissionCodes: ['old:permission'] })
    await pending

    expect(store.status).toBe('idle')
    expect(store.roleCodes).toEqual([])
    expect(store.menuTree).toEqual([])
    expect(store.permissionCodes).toEqual([])
  })

  it.each([
    { error: new ProtocolError('invalid DTO'), message: '访问权限响应格式无效' },
    { error: new ApiError(10006, '服务暂未就绪', 503), message: '服务暂未就绪' },
    { error: new Error('network down'), message: '加载访问权限失败' },
  ])('clears access and exposes the correct public failure message', async ({ error, message }) => {
    getPermissionMock.mockRejectedValue(error)
    const store = usePermissionStore()
    store.roleCodes.push('old')
    store.menuTree.push({
      code: 'old',
      menuType: 'directory',
      path: null,
      componentPath: null,
      i18nKey: 'navigation.system',
      icon: null,
      isHidden: YesNo.No,
      children: [],
    })
    store.permissionCodes.push('old:permission')

    await expect(store.load()).rejects.toBe(error)

    expect(store.status).toBe('error')
    expect(store.roleCodes).toEqual([])
    expect(store.menuTree).toEqual([])
    expect(store.permissionCodes).toEqual([])
    expect(store.errorMessage).toBe(message)
  })

  it('reset clears every user-specific value', () => {
    const store = usePermissionStore()
    store.applySnapshot({ roleCodes: ['role'], menuTree: [], permissionCodes: ['permission'] })
    store.reset()
    expect(store.status).toBe('idle')
    expect(store.errorMessage).toBe('')
    expect(store.roleCodes).toEqual([])
    expect(store.menuTree).toEqual([])
    expect(store.permissionCodes).toEqual([])
  })
})

function emptySnapshot(): PermissionSnapshot {
  return { roleCodes: [], menuTree: [], permissionCodes: [] }
}

function deferred<T>(): { promise: Promise<T>; resolve: (value: T) => void } {
  let resolvePromise: ((value: T) => void) | undefined
  const promise = new Promise<T>((resolve) => {
    resolvePromise = resolve
  })
  return {
    promise,
    resolve: (value: T) => {
      if (resolvePromise === undefined) throw new Error('deferred promise was not initialized')
      resolvePromise(value)
    },
  }
}
