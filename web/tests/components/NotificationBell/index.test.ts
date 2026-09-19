import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import NotificationBell from '@/components/NotificationBell/index.vue'
import { usePermissionStore } from '@/store/permission'
import { useNotificationStore } from '@/store/notification'
import { createRouter, createMemoryHistory } from 'vue-router'
import { ElMessage } from 'element-plus'
import { appI18n } from '@/i18n'
import * as notificationApi from '@/api/message/notification'

vi.mock('@/api/message/notification', () => ({
  getNotificationSummary: vi.fn().mockResolvedValue({ unreadCount: 0, recent: [] }),
  readAllNotifications: vi.fn(),
  readNotification: vi.fn(),
  deleteNotification: vi.fn(),
}))

describe('NotificationBell', () => {
  beforeEach(() => setActivePinia(createPinia()))
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/message/notification', component: { template: '<div />' } },
      { path: '/target', component: { template: '<div />' } },
    ],
  })
  it('is hidden without list permission', () => {
    expect(
      mount(NotificationBell, {
        global: {
          plugins: [router, appI18n],
          stubs: { ElTooltip: true, ElPopover: true, ElBadge: true, ElButton: true, DIcon: true },
        },
      })
        .find('[data-testid="notification-bell"]')
        .exists(),
    ).toBe(false)
  })
  it('caps the badge at 99+', async () => {
    usePermissionStore().permissionCodes = ['message:notification:list']
    const store = useNotificationStore()
    store.unreadCount = 100
    const wrapper = mount(NotificationBell, {
      global: {
        plugins: [router, appI18n],
        stubs: {
          ElTooltip: { template: '<div><slot /></div>' },
          ElPopover: { template: '<div><slot name="reference" /></div>' },
          ElBadge: { props: ['value'], template: '<span :data-value="value"><slot /></span>' },
          ElButton: { template: '<button><slot /></button>' },
          DIcon: true,
        },
      },
    })
    expect(wrapper.find('[data-value="99+"]').exists()).toBe(true)
  })

  it('opens the recent notification popover when the bell is clicked', async () => {
    usePermissionStore().permissionCodes = [
      'message:notification:view',
      'message:notification:list',
      'message:notification:read',
    ]
    vi.mocked(notificationApi.getNotificationSummary).mockResolvedValueOnce({
      unreadCount: 1,
      recent: [
        {
          id: 9,
          title: 'Visible notice',
          summary: 'Visible body',
          variant: 'info',
          priority: 'normal',
          linkType: 'none',
          link: '',
          publishedAt: '2026-09-18T12:00:00Z',
          isRead: false,
        },
      ],
    })
    const wrapper = mount(NotificationBell, {
      attachTo: document.body,
      global: { plugins: [router, appI18n] },
    })

    try {
      await vi.waitFor(() =>
        expect(document.querySelector('[data-testid="notification-bell-item-9"]')).not.toBeNull(),
      )
      await wrapper.get('[data-testid="notification-bell"]').trigger('click')

      await vi.waitFor(() => {
        const popover = document
          .querySelector('[data-testid="notification-bell-item-9"]')
          ?.closest('.el-popover')
        expect(popover?.getAttribute('aria-hidden')).toBe('false')
      })
    } finally {
      wrapper.unmount()
    }
  })

  it('does not expose read actions without read permission and still opens a link', async () => {
    usePermissionStore().permissionCodes = ['message:notification:list']
    vi.mocked(notificationApi.getNotificationSummary).mockResolvedValueOnce({
      unreadCount: 1,
      recent: [
        {
          id: 7,
          title: 'Notice',
          summary: 'Body',
          variant: 'info',
          priority: 'normal',
          linkType: 'external',
          link: 'https://example.test/path',
          publishedAt: '2026-09-18T12:00:00Z',
          isRead: false,
        },
      ],
    })
    const open = vi.spyOn(window, 'open').mockImplementation(() => null)
    const wrapper = mount(NotificationBell, {
      global: {
        plugins: [router, appI18n],
        stubs: {
          ElTooltip: { template: '<div><slot /></div>' },
          ElPopover: {
            template: '<div><slot name="reference" /><slot /></div>',
          },
          ElBadge: { props: ['value'], template: '<span :data-value="value"><slot /></span>' },
          ElButton: { template: '<button v-bind="$attrs"><slot /></button>' },
          ElIcon: { template: '<i><slot /></i>' },
        },
      },
    })
    await vi.waitFor(() =>
      expect(wrapper.find('[data-testid="notification-bell-item-7"]').exists()).toBe(true),
    )
    expect(wrapper.find('[data-testid="notification-bell-read-all"]').exists()).toBe(false)
    await wrapper.get('[data-testid="notification-bell-item-7"]').trigger('click')
    expect(notificationApi.readNotification).not.toHaveBeenCalled()
    expect(open).toHaveBeenCalledWith('https://example.test/path', '_blank', 'noopener,noreferrer')
    open.mockRestore()
  })

  it('does not expose the notification center route to a list-only user', async () => {
    usePermissionStore().permissionCodes = ['message:notification:list']
    const wrapper = mount(NotificationBell, {
      global: {
        plugins: [router, appI18n],
        stubs: {
          ElTooltip: { template: '<div><slot /></div>' },
          ElPopover: { template: '<div><slot name="reference" /><slot /></div>' },
          ElBadge: { template: '<span><slot /></span>' },
          ElButton: { template: '<button v-bind="$attrs"><slot /></button>' },
        },
      },
    })
    await vi.waitFor(() => expect(notificationApi.getNotificationSummary).toHaveBeenCalled())
    expect(wrapper.find('[data-testid="notification-bell-view-all"]').exists()).toBe(false)
  })

  it('reports a rejected internal navigation without leaking the rejection', async () => {
    usePermissionStore().permissionCodes = [
      'message:notification:view',
      'message:notification:list',
    ]
    vi.mocked(notificationApi.getNotificationSummary).mockResolvedValueOnce({
      unreadCount: 0,
      recent: [
        {
          id: 8,
          title: 'Target',
          summary: 'Body',
          variant: 'info',
          priority: 'normal',
          linkType: 'internal',
          link: '/target',
          publishedAt: '2026-09-18T12:00:00Z',
          isRead: true,
        },
      ],
    })
    const push = vi.spyOn(router, 'push').mockRejectedValueOnce(new Error('navigation failed'))
    const notify = vi.spyOn(ElMessage, 'error')
    const wrapper = mount(NotificationBell, {
      global: {
        plugins: [router, appI18n],
        stubs: {
          ElTooltip: { template: '<div><slot /></div>' },
          ElPopover: { template: '<div><slot name="reference" /><slot /></div>' },
          ElBadge: { template: '<span><slot /></span>' },
          ElButton: { template: '<button v-bind="$attrs"><slot /></button>' },
          ElIcon: { template: '<i><slot /></i>' },
        },
      },
    })
    await vi.waitFor(() =>
      expect(wrapper.find('[data-testid="notification-bell-item-8"]').exists()).toBe(true),
    )
    await wrapper.get('[data-testid="notification-bell-item-8"]').trigger('click')
    await vi.waitFor(() => expect(notify).toHaveBeenCalled())
    expect(push).toHaveBeenCalledWith('/target')
  })
})
