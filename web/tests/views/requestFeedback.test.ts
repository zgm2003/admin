import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import type { Component } from 'vue'
import ElementPlus, { ElNotification } from 'element-plus'
import type { CreateAxiosDefaults } from 'axios'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { appI18n, setLocale } from '@/i18n'
import { pinia } from '@/store'
import { usePermissionStore } from '@/store/permission'

const transport = vi.hoisted(() => ({ requests: [] as string[], message: '接口失败详情只显示一次' }))

// Exercise the real API modules, request interceptors and notifications; replace network I/O only.
vi.mock('axios', async (importOriginal) => {
  const actual = await importOriginal<typeof import('axios')>()
  return {
    ...actual,
    default: {
      ...actual.default,
      create: (config?: CreateAxiosDefaults) => actual.default.create({
        ...config,
        adapter: async (request) => {
          transport.requests.push(request.url ?? '')
          return {
            config: request, status: 200, statusText: 'OK', headers: {},
            data: { code: 10001, data: null, message: transport.message },
          }
        },
      }),
    },
  }
})

const pages = [
  'permission/menu', 'permission/role', 'permission/authPlatform',
  'user/account', 'user/session', 'user/loginLog', 'user/profile',
  'payment/config', 'payment/rechargePackage', 'payment/redeemCode', 'payment/recharge',
  'payment/order', 'payment/wallet', 'payment/transaction', 'payment/myWallet',
  'system/setting', 'system/configCacheStatus', 'system/operationLog', 'system/scheduler',
  'message/mail', 'message/sms', 'message/notification', 'message/notificationTask',
] as const
const modules = import.meta.glob('../../src/views/**/index.vue')
const permissionCodes = [
  'permission:menu:list', 'permission:role:list', 'permission:authPlatform:list',
  'user:account:list', 'user:session:list', 'user:loginLog:list', 'user:profile:detail',
  'payment:config:list', 'payment:rechargePackage:list', 'payment:redeemCode:list',
  'payment:recharge:create', 'payment:recharge:list', 'payment:order:list',
  'payment:wallet:list', 'payment:transaction:list', 'payment:myWallet:detail',
  'payment:myWallet:list', 'system:setting:list', 'system:configCacheStatus:list',
  'system:operationLog:list', 'system:scheduler:list', 'message:mail:list',
  'message:sms:list', 'message:notification:list', 'message:notificationTask:list',
]
let wrapper: VueWrapper | undefined
beforeEach(() => {
  setLocale('zh-CN')
  transport.requests = []
  usePermissionStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes })
})
afterEach(async () => {
  wrapper?.unmount()
  wrapper = undefined
  ElNotification.closeAll()
  await flushPromises()
  document.body.innerHTML = ''
})

it.each(pages)('%s leaves API error details exclusively to request notifications', async (path) => {
  const load = modules[`../../src/views/${path}/index.vue`]
  if (load === undefined) throw new Error(`Missing page: ${path}`)
  const loaded = await load()
  if (typeof loaded !== 'object' || loaded === null || !('default' in loaded)) {
    throw new Error(`Invalid page module: ${path}`)
  }
  if (typeof loaded.default !== 'object' && typeof loaded.default !== 'function') {
    throw new Error(`Invalid page component: ${path}`)
  }
  const page = loaded.default as Component
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: page }] })
  await router.push('/')
  await router.isReady()
  wrapper = mount(page, { global: { plugins: [ElementPlus, pinia, appI18n, router] } })
  await flushPromises()

  expect(transport.requests.length).toBeGreaterThan(0)
  const notifications = document.body.querySelectorAll('.el-notification')
  expect(notifications).toHaveLength(transport.requests.length)
  for (const notification of notifications) expect(notification.textContent).toContain(transport.message)
  expect(wrapper.text()).not.toContain(transport.message)
})
