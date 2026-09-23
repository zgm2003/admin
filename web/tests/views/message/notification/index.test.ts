import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'

import * as api from '@/api/message/notification'
import type { NotificationItem } from '@/api/message/notification'
import { appI18n, setLocale } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import Page from '@/views/message/notification/index.vue'
import { formatTime } from '@/utils/datetime'

vi.mock('@/api/message/notification', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/message/notification')>()
  return {
    ...actual,
    listNotifications: vi.fn(),
    readNotification: vi.fn().mockResolvedValue(undefined),
    readAllNotifications: vi.fn().mockResolvedValue(undefined),
    deleteNotification: vi.fn().mockResolvedValue(undefined),
  }
})

const listNotifications = vi.mocked(api.listNotifications)
const mountedWrappers: VueWrapper[] = []

describe('notification center', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('zh-CN')
    listNotifications.mockResolvedValue({ items: [], nextBeforeId: null })
  })
  afterEach(() => {
    for (const wrapper of mountedWrappers.splice(0)) wrapper.unmount()
    document.body.innerHTML = ''
  })

  it('does not load without list permission', async () => {
    const wrapper = mountPage([])
    await flushPromises()
    expect(listNotifications).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('暂无通知查看权限')
  })

  it('renders a standard management table and submits exact filters', async () => {
    listNotifications.mockResolvedValue({ items: [row()], nextBeforeId: null })
    const wrapper = mountPage([
      'message:notification:list',
      'message:notification:read',
      'message:notification:delete',
    ])
    await flushPromises()

    expect(listNotifications).toHaveBeenCalledWith({ limit: 20, filter: 'all' })
    expect(wrapper.find('h1').exists()).toBe(false)
    expect(wrapper.get('.notification-center').classes()).toContain('management-page')
    expect(wrapper.find('[data-testid="notification-table"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('年度体检通知')
    expect(wrapper.text()).toContain('请于本周五前完成预约')
    expect(wrapper.get('[data-testid="notification-variant-1"]').text()).toBe('信息')
    expect(wrapper.text()).toContain('未读')
    expect(wrapper.text()).toContain(formatTime('2026-09-18T12:00:00Z'))

    await selectByTestId(wrapper, 'notification-unread-filter').vm.$emit(
      'update:modelValue',
      'unread',
    )
    await selectByTestId(wrapper, 'notification-variant-filter').vm.$emit(
      'update:modelValue',
      'warning',
    )
    await selectByTestId(wrapper, 'notification-priority-filter').vm.$emit(
      'update:modelValue',
      'urgent',
    )
    await wrapper.get('[data-testid="notification-search"]').trigger('click')
    await flushPromises()
    expect(listNotifications).toHaveBeenLastCalledWith({
      limit: 20,
      filter: 'unread',
      variant: 'warning',
      priority: 'urgent',
    })

    await wrapper.get('[data-testid="notification-reset"]').trigger('click')
    await flushPromises()
    expect(listNotifications).toHaveBeenLastCalledWith({ limit: 20, filter: 'all' })
  })

  it('expands sanitized content and exposes the external link action', async () => {
    listNotifications.mockResolvedValue({
      items: [row({ contentHtml: '<p><strong>Body</strong></p>', linkType: 'external' })],
      nextBeforeId: null,
    })
    const wrapper = mountPage(['message:notification:list'])
    await flushPromises()

    await wrapper.get('button[aria-label="Expand this row"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Body')
    expect(wrapper.text()).toContain('查看内容')
  })

  it('gates read and delete without leaking actions', async () => {
    listNotifications.mockResolvedValue({ items: [row()], nextBeforeId: null })
    const readonly = mountPage(['message:notification:list'])
    await flushPromises()
    expect(readonly.find('[data-testid="notification-read-1"]').exists()).toBe(false)
    expect(readonly.find('[data-testid="notification-delete-1"]').exists()).toBe(false)
    expect(readonly.find('[data-testid="notification-read-all"]').exists()).toBe(false)

    const wrapper = mountPage([
      'message:notification:list',
      'message:notification:read',
      'message:notification:delete',
    ])
    await flushPromises()
    expect(wrapper.get('[data-testid="notification-read-1"]').text()).toBe('标记已读')
    expect(wrapper.get('[data-testid="notification-delete-1"]').text()).toBe('删除')

    await wrapper.get('[data-testid="notification-read-1"]').trigger('click')
    await flushPromises()
    expect(api.readNotification).toHaveBeenCalledWith(1)
    await vi.waitFor(() => expect(wrapper.text()).toContain('已读'))

    await wrapper.get('[data-testid="notification-delete-1"]').trigger('click')
    await flushPromises()
    expect(api.deleteNotification).toHaveBeenCalledWith(1)
    await vi.waitFor(() => expect(wrapper.text()).not.toContain('年度体检通知'))
  })

  it('marks every notification as read from the toolbar', async () => {
    listNotifications.mockResolvedValue({ items: [row(), row({ id: 2 })], nextBeforeId: null })
    const wrapper = mountPage(['message:notification:list', 'message:notification:read'])
    await flushPromises()

    await wrapper.get('[data-testid="notification-read-all"]').trigger('click')
    await flushPromises()
    expect(api.readAllNotifications).toHaveBeenCalledTimes(1)
    await vi.waitFor(() => expect(wrapper.text()).not.toContain('未读'))
  })

  it('keeps urgent priority visible and normal rows quiet', async () => {
    listNotifications.mockResolvedValue({
      items: [row({ id: 5, priority: 'urgent', isRead: true }), row({ id: 6, isRead: true })],
      nextBeforeId: null,
    })
    const wrapper = mountPage(['message:notification:list'])
    await flushPromises()
    expect(wrapper.get('[data-testid="notification-priority-5"]').text()).toBe('紧急')
    expect(wrapper.find('[data-testid="notification-priority-6"]').exists()).toBe(false)
  })

  it('loads the next cursor page on demand', async () => {
    listNotifications
      .mockResolvedValueOnce({ items: [row()], nextBeforeId: 21 })
      .mockResolvedValueOnce({ items: [row({ id: 21, title: '第二条通知' })], nextBeforeId: null })
    const wrapper = mountPage(['message:notification:list'])
    await flushPromises()

    await wrapper.get('[data-testid="notification-load-more"]').trigger('click')
    await flushPromises()
    expect(listNotifications).toHaveBeenLastCalledWith({ limit: 20, filter: 'all', beforeId: 21 })
    expect(wrapper.text()).toContain('第二条通知')
    expect(wrapper.find('[data-testid="notification-load-more"]').exists()).toBe(false)
  })

  it('renders explicit loading, empty, and error states', async () => {
    let resolveList:
      ((value: { items: NotificationItem[]; nextBeforeId: number | null }) => void) | undefined
    listNotifications.mockReturnValue(
      new Promise((resolve) => {
        resolveList = resolve
      }),
    )
    const wrapper = mountPage(['message:notification:list'])
    await nextTick()
    expect(wrapper.find('[data-testid="notification-loading"]').exists()).toBe(true)
    resolveList?.({ items: [], nextBeforeId: null })
    await flushPromises()
    expect(wrapper.text()).toContain('暂无通知')

    listNotifications.mockRejectedValue(new Error('查询失败'))
    const failed = mountPage(['message:notification:list'])
    await flushPromises()
    expect(failed.text()).toContain('查询失败')
  })
})

function mountPage(permissionCodes: string[]): VueWrapper {
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes })
  const router = createRouter({ history: createMemoryHistory(), routes: [] })
  const wrapper = mount(Page, {
    attachTo: document.body,
    global: { plugins: [pinia, router, appI18n, ElementPlus] },
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

function selectByTestId(wrapper: VueWrapper, testId: string) {
  const select = wrapper
    .findAllComponents({ name: 'ElSelectV2' })
    .find((component) => component.attributes('data-testid') === testId)
  if (select === undefined) throw new Error(`select not found: ${testId}`)
  return select
}

function row(overrides: Partial<NotificationItem> = {}): NotificationItem {
  return {
    id: 1,
    title: '年度体检通知',
    summary: '请于本周五前完成预约',
    contentHtml: '<p>正文</p>',
    variant: 'info',
    priority: 'normal',
    linkType: 'none',
    link: '',
    publishedAt: '2026-09-18T12:00:00Z',
    isRead: false,
    ...overrides,
  }
}
