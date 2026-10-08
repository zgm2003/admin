import { AxiosError, type AxiosAdapter, type CreateAxiosDefaults } from 'axios'
import { ElNotification } from 'element-plus/es/components/notification/index'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { pinia } from '@/store'
import { useAuthStore } from '@/store/auth'
import { usePermissionStore } from '@/store/permission'
import { refreshAccessCredential } from '@/utils/request'

const { transport } = vi.hoisted(() => ({ transport: vi.fn<AxiosAdapter>() }))

vi.mock('axios', async (importOriginal) => {
  const actual = await importOriginal<typeof import('axios')>()
  return {
    ...actual,
    default: {
      ...actual.default,
      create: (config: CreateAxiosDefaults = {}) =>
        actual.default.create({ ...config, adapter: transport }),
    },
  }
})

describe('Unauthorized recovery from the login page', () => {
  beforeEach(() => {
    useAuthStore(pinia).$reset()
    usePermissionStore(pinia).reset()
    vi.spyOn(ElNotification, 'error').mockImplementation(() => ({ close: () => undefined }))
    transport.mockReset().mockImplementation(async (config) => {
      throw new AxiosError('HTTP 401', AxiosError.ERR_BAD_REQUEST, config, undefined, {
        data: { code: 10002, data: null, message: '登录已失效' },
        status: 401,
        statusText: 'Unauthorized',
        headers: {},
        config,
      })
    })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it.each(['/login', '/login/'])(
    'keeps an expired session on %s without reloading or nesting its original redirect',
    async (pathname) => {
      const assign = vi.fn()
      vi.stubGlobal('window', {
        localStorage: window.localStorage,
        location: { pathname, search: '?redirect=%2Fuser%2Fprofile', hash: '', assign },
      })
      useAuthStore(pinia).setError('Backend unavailable')
      usePermissionStore(pinia).applySnapshot({
        roleCodes: [],
        menuTree: [],
        permissionCodes: ['user:profile:view'],
      })

      await expect(refreshAccessCredential()).rejects.toMatchObject({
        code: 10002,
        httpStatus: 401,
      })

      expect(assign).not.toHaveBeenCalled()
      expect(useAuthStore(pinia).status).toBe('anonymous')
      expect(useAuthStore(pinia).errorMessage).toBe('')
      expect(usePermissionStore(pinia).permissionCodes).toEqual([])
      expect(ElNotification.error).toHaveBeenCalledOnce()
    },
  )

  it('still redirects an expired protected page and preserves its query and hash', async () => {
    const assign = vi.fn()
    vi.stubGlobal('window', {
      localStorage: window.localStorage,
      location: { pathname: '/user/profile', search: '?tab=email', hash: '#security', assign },
    })

    await expect(refreshAccessCredential()).rejects.toMatchObject({ code: 10002, httpStatus: 401 })

    expect(assign).toHaveBeenCalledExactlyOnceWith(
      '/login?redirect=%2Fuser%2Fprofile%3Ftab%3Demail%23security',
    )
    expect(useAuthStore(pinia).status).toBe('anonymous')
  })
})
