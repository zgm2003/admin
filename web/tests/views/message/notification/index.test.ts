import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import { appI18n } from '@/i18n'
import Page from '@/views/message/notification/index.vue'
import { usePermissionStore } from '@/store/permission'
import { formatTime } from '@/utils/datetime'
import * as api from '@/api/message/notification'
vi.mock('@/api/message/notification', async (o) => ({
  ...(await o()),
  listNotifications: vi.fn().mockResolvedValue({ items: [], nextBeforeId: null }),
  readNotification: vi.fn().mockResolvedValue(undefined),
  readAllNotifications: vi.fn().mockResolvedValue(undefined),
  deleteNotification: vi.fn().mockResolvedValue(undefined),
}))
describe('notification center', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })
  it('does not load without list permission', () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [] })
    mount(Page, {
      global: {
        plugins: [router, appI18n],
        stubs: {
          AppPage: { template: '<section><slot /></section>' },
          ElEmpty: true,
          ElButton: true,
          ElSelectV2: true,
        },
      },
    })
    expect(api.listNotifications).not.toHaveBeenCalled()
  })
  it('renders sanitized body with precise action permissions', async () => {
    usePermissionStore().permissionCodes = [
      'message:notification:list',
      'message:notification:read',
      'message:notification:delete',
    ]
    vi.mocked(api.listNotifications).mockResolvedValue({
      items: [
        {
          id: 1,
          title: 'T',
          contentHtml: '<p><strong>Body</strong></p>',
          summary: 'Body',
          variant: 'info',
          priority: 'normal',
          linkType: 'none',
          link: '',
          publishedAt: '2026-09-18T12:00:00Z',
          isRead: false,
        },
      ],
      nextBeforeId: null,
    })
    const router = createRouter({ history: createMemoryHistory(), routes: [] })
    const w = mount(Page, {
      global: {
        plugins: [router, appI18n],
        stubs: {
          AppPage: { template: '<section><slot /></section>' },
          ElEmpty: true,
          ElButton: { template: '<button><slot /></button>' },
          ElSelectV2: true,
          ElSegmented: {
            name: 'ElSegmented',
            props: ['modelValue', 'options'],
            template: '<div data-testid="notification-filter-mode" />',
          },
          ElTag: { template: '<span><slot /></span>' },
          ElTooltip: { template: '<span><slot /></span>' },
        },
      },
    })
    await vi.waitFor(() => expect(w.html()).toContain('<strong>Body</strong>'))
    expect(w.getComponent({ name: 'ElSegmented' }).props('options')).toEqual([
      { value: 'all', label: '全部通知' },
      { value: 'unread', label: '未读通知' },
    ])
    expect(w.findAll('.notification-center__filter-label').map((label) => label.text())).toEqual([
      '类型',
      '优先级',
    ])
    expect(w.get('[data-testid="notification-item-1"]').classes()).toContain('is-unread')
    expect(w.get('[data-testid="notification-variant-1"]').text()).toBe('信息')
    expect(w.get('[data-testid="notification-priority-1"]').text()).toBe('普通')
    expect(w.get('[data-testid="notification-published-1"]').text()).toBe(
      formatTime('2026-09-18T12:00:00Z'),
    )
    expect(w.find('[data-testid="notification-read-1"]').exists()).toBe(true)
    expect(w.find('[data-testid="notification-delete-1"]').exists()).toBe(true)
    expect(w.get('[data-testid="notification-read-1"]').attributes('aria-label')).toBe('标记已读')
    expect(w.get('[data-testid="notification-delete-1"]').attributes('aria-label')).toBe('删除')

    await w.get('[data-testid="notification-read-1"]').trigger('click')
    await vi.waitFor(() =>
      expect(w.find('[data-testid="notification-read-1"]').exists()).toBe(false),
    )
    expect(w.get('[data-testid="notification-item-1"]').classes()).not.toContain('is-unread')

    await w.get('[data-testid="notification-delete-1"]').trigger('click')
    await vi.waitFor(() => expect(w.text()).not.toContain('Body'))
  })
})
