import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

import * as operationLogAPI from '@/api/system/operationLog'
import type { OperationLogItem, OperationLogPage } from '@/api/system/operationLog'
import { YesNo } from '@/enums/yesNo'
import { appI18n, setLocale } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import OperationLogs from '@/views/system/operationLog/index.vue'

vi.mock('@/api/system/operationLog', () => ({ getOperationLogs: vi.fn() }))
const getOperationLogs = vi.mocked(operationLogAPI.getOperationLogs)
const mountedWrappers: VueWrapper[] = []

describe('operation logs', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    setLocale('zh-CN')
    getOperationLogs.mockResolvedValue({ list: [row()], total: 1, page: 1, pageSize: 20 })
  })
  afterEach(() => {
    for (const wrapper of mountedWrappers.splice(0)) wrapper.unmount()
    document.body.innerHTML = ''
  })

  it('loads logs and submits exact filters and pagination', async () => {
    const wrapper = mountPage()
    await flushPromises()
    expect(getOperationLogs).toHaveBeenCalledWith({ page: 1, pageSize: 20 })
    expect(wrapper.find('h1').exists()).toBe(false)
    expect(wrapper.get('.operation-log-page').classes()).toContain('management-page')
    await wrapper.get('[data-testid="operation-log-user-id"]').setValue('7')
    await wrapper.get('[data-testid="operation-log-action"]').setValue('user.update')
    await wrapper.get('[data-testid="operation-log-route"]').setValue('/api/v1/users')
    wrapper.findComponent({ name: 'ElSelectV2' }).vm.$emit('update:modelValue', YesNo.No)
    wrapper
      .findComponent({ name: 'ElDatePicker' })
      .vm.$emit('update:modelValue', ['2026-08-20T00:00:00+08:00', '2026-08-21T00:00:00+08:00'])
    await wrapper.get('[data-testid="operation-log-search"]').trigger('click')
    await flushPromises()
    expect(getOperationLogs).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 20,
      userId: 7,
      action: 'user.update',
      route: '/api/v1/users',
      isSuccess: YesNo.No,
      from: '2026-08-20T00:00:00+08:00',
      to: '2026-08-21T00:00:00+08:00',
    })

    wrapper.findComponent({ name: 'ElPagination' }).vm.$emit('current-change', 2)
    await flushPromises()
    expect(getOperationLogs).toHaveBeenLastCalledWith(
      expect.objectContaining({ page: 2, pageSize: 20 }),
    )
  })

  it('expands sanitized details and never renders a delete command', async () => {
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.findComponent({ name: 'ElSpace' }).exists()).toBe(true)
    expect(wrapper.find('[data-testid="operation-log-delete"]').exists()).toBe(false)
    await wrapper.get('button[aria-label="Expand this row"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('***')
    expect(wrapper.text()).toContain('request-id-1')
  })

  it('renders explicit loading, empty, and error states', async () => {
    let resolveLogs:
      | ((value: {
          list: OperationLogItem[]
          total: number
          page: number
          pageSize: number
        }) => void)
      | undefined
    getOperationLogs.mockReturnValue(
      new Promise((resolve) => {
        resolveLogs = resolve
      }),
    )
    const wrapper = mountPage()
    await nextTick()
    expect(wrapper.find('[data-testid="operation-log-loading"]').exists()).toBe(true)
    resolveLogs?.({ list: [], total: 0, page: 1, pageSize: 20 })
    await flushPromises()
    expect(wrapper.text()).toContain('暂无操作日志')

    getOperationLogs.mockRejectedValue(new Error('查询失败'))
    const failed = mountPage()
    await flushPromises()
    expect(failed.text()).toContain('查询失败')
  })

  it('renders backend action labels verbatim, including new actions unknown to the frontend', async () => {
    getOperationLogs.mockResolvedValueOnce({
      list: [{ ...row(), actionLabel: '服务端操作原样标签' }],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('服务端操作原样标签')
    expect(wrapper.text()).not.toContain('编辑用户')
    expect(wrapper.text()).toContain('admin')

    getOperationLogs.mockResolvedValue({
      list: [
        { ...row(), action: 'future.action', actionLabel: 'future.action' },
        { ...row(), id: 2, action: 'mail.config.update', actionLabel: '编辑邮件配置' },
        { ...row(), id: 3, action: 'user.password.update', actionLabel: '修改密码' },
        { ...row(), id: 4, action: 'future.serverAdded', actionLabel: '新后端动作' },
      ],
      total: 4,
      page: 1,
      pageSize: 20,
    })
    const fallback = mountPage()
    await flushPromises()
    expect(fallback.text()).toContain('future.action')
    expect(fallback.text()).toContain('编辑邮件配置')
    expect(fallback.text()).toContain('修改密码')
    expect(fallback.text()).toContain('新后端动作')
  })

  it('reloads localized labels and ignores late data from the previous locale', async () => {
    const old = deferred<OperationLogPage>()
    getOperationLogs.mockReturnValueOnce(old.promise)
    const wrapper = mountPage()
    await nextTick()
    getOperationLogs.mockResolvedValueOnce(page('Backend English action'))
    setLocale('en-US')
    await flushPromises()
    expect(getOperationLogs).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('Backend English action')
    old.resolve(page('迟到中文动作'))
    await flushPromises()
    expect(wrapper.text()).toContain('Backend English action')
    expect(wrapper.text()).not.toContain('迟到中文动作')
  })

  it('ignores errors and loading finalizers from the previous locale', async () => {
    const old = deferred<OperationLogPage>()
    const english = deferred<OperationLogPage>()
    getOperationLogs.mockReturnValueOnce(old.promise)
    const wrapper = mountPage()
    await nextTick()
    getOperationLogs.mockReturnValueOnce(english.promise)
    setLocale('en-US')
    await flushPromises()
    old.reject(new Error('过期错误'))
    await flushPromises()
    expect(wrapper.text()).not.toContain('过期错误')
    expect(wrapper.find('[data-testid="operation-log-loading"]').exists()).toBe(true)
    english.resolve(page('Backend English action'))
    await flushPromises()
    expect(wrapper.text()).toContain('Backend English action')
    expect(wrapper.find('[data-testid="operation-log-loading"]').exists()).toBe(false)
  })

  it('does not request logs without the independent list action permission', async () => {
    const wrapper = mountPage([])
    await flushPromises()
    expect(getOperationLogs).not.toHaveBeenCalled()
    expect(wrapper.text()).not.toContain('后端编辑用户')
  })

  it('clears data on list permission loss and discards an in-flight response', async () => {
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('后端编辑用户')
    const pending = deferred<OperationLogPage>()
    getOperationLogs.mockReturnValueOnce(pending.promise)
    await wrapper.get('[data-testid="operation-log-search"]').trigger('click')
    const access = usePermissionStore()
    access.permissionCodes = []
    await nextTick()
    expect(wrapper.text()).not.toContain('后端编辑用户')
    pending.resolve(page('失权后迟到的审计数据'))
    await flushPromises()
    expect(wrapper.text()).not.toContain('失权后迟到的审计数据')
    expect(wrapper.find('[data-testid="operation-log-loading"]').exists()).toBe(false)
    const calls = getOperationLogs.mock.calls.length
    await wrapper.get('[data-testid="operation-log-search"]').trigger('click')
    await flushPromises()
    expect(getOperationLogs).toHaveBeenCalledTimes(calls)
    access.permissionCodes = ['system:operationLog:list']
    getOperationLogs.mockResolvedValueOnce(page('重新获得权限后的数据'))
    await flushPromises()
    expect(wrapper.text()).toContain('重新获得权限后的数据')
  })
})

function mountPage(permissions = ['system:operationLog:list']): VueWrapper {
  const pinia = createPinia()
  setActivePinia(pinia)
  const access = usePermissionStore()
  access.status = 'ready'
  access.permissionCodes = permissions
  const wrapper = mount(OperationLogs, {
    attachTo: document.body,
    global: { plugins: [pinia, appI18n, ElementPlus] },
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

function row(): OperationLogItem {
  return {
    id: 1,
    requestId: 'request-id-1',
    userId: 7,
    userName: 'admin',
    sessionId: 9,
    platform: 'admin',
    method: 'PUT',
    route: '/api/admin/v1/user/account/:id',
    module: 'user',
    action: 'user.update',
    actionLabel: '后端编辑用户',
    clientIp: '127.0.0.1',
    userAgent: 'Chrome',
    statusCode: 200,
    isSuccess: YesNo.Yes,
    latencyMs: 12,
    requestData: { password: '***' },
    responseData: { code: 0 },
    createdAt: '2026-08-21T00:00:00Z',
    updatedAt: '2026-08-21T00:00:00Z',
  }
}

function page(actionLabel: string): OperationLogPage {
  return { list: [{ ...row(), actionLabel }], total: 1, page: 1, pageSize: 20 }
}

function deferred<T>(): {
  promise: Promise<T>
  resolve: (value: T) => void
  reject: (error: Error) => void
} {
  let resolve!: (value: T) => void
  let reject!: (error: Error) => void
  const promise = new Promise<T>((done, fail) => {
    resolve = done
    reject = fail
  })
  return { promise, resolve, reject }
}
