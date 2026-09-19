import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { appI18n } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import * as api from '@/api/message/notificationTask'
import Page from '@/views/message/notificationTask/index.vue'
import {
  notificationEditorConfig,
  notificationToolbarKeys,
} from '@/views/message/notificationTask/components/NotificationEditor/index.vue'

vi.mock('@/api/message/notificationTask', async (original) => ({
  ...(await original()),
  listNotificationTasks: vi.fn(),
  getNotificationTask: vi.fn(),
  listNotificationTaskOptions: vi.fn(),
}))

const task: api.NotificationTask = {
  id: 1,
  platformId: 2,
  notificationId: null,
  title: 'Notice',
  contentHtml: '<p>Body</p>',
  summary: 'Body',
  variant: 'info',
  priority: 'normal',
  linkType: 'none',
  link: '',
  audienceType: 'platform',
  targetIds: [],
  scheduledAt: null,
  audienceMaxUserId: null,
  submittedAt: null,
  publishedAt: null,
  completedAt: null,
  canceledAt: null,
  failedAt: null,
  failureMessage: null,
  status: 'draft',
  generatedCount: 0,
  createdBy: 3,
  createdAt: '2026-09-18T12:00:00Z',
  updatedAt: '2026-09-18T12:00:00Z',
}

describe('notification task management', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.mocked(api.listNotificationTasks).mockResolvedValue({
      list: [task],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    vi.mocked(api.listNotificationTaskOptions).mockResolvedValue({
      items: [{ id: 2, label: 'Admin' }],
      nextAfterId: 3,
    })
    vi.mocked(api.getNotificationTask).mockResolvedValue(task)
  })
  it('uses only the approved editor controls and HTTPS links', () => {
    expect(notificationToolbarKeys).toEqual([
      'bold',
      'italic',
      'underline',
      'headerSelect',
      'bulletedList',
      'numberedList',
      'insertLink',
      'undo',
      'redo',
      'clearStyle',
    ])
    expect(notificationEditorConfig.MENU_CONF.insertLink.checkLink('https://example.test')).toBe(
      true,
    )
    expect(notificationEditorConfig.MENU_CONF.insertLink.checkLink('http://example.test')).toBe(
      false,
    )
  })
  it('shows draft commands only with exact permissions', async () => {
    usePermissionStore().permissionCodes = [
      'message:notificationTask:list',
      'message:notificationTask:detail',
      'message:notificationTask:update',
      'message:notificationTask:delete',
      'message:notificationTask:submit',
    ]
    const wrapper = mount(Page, {
      global: {
        plugins: [appI18n],
        stubs: {
          AppPage: { template: '<section><slot /></section>' },
          AppTable: {
            props: ['data'],
            template:
              '<div><slot name="toolbar-right" /><slot v-if="data.length" name="actions" :row="data[0]" /></div>',
          },
          AppDialog: true,
          ElButton: { template: '<button v-bind="$attrs"><slot /></button>' },
          ElSelectV2: true,
          ElInput: true,
          ElForm: true,
          ElFormItem: true,
        },
      },
    })
    await vi.waitFor(() =>
      expect(wrapper.find('[data-testid="notification-task-edit-1"]').exists()).toBe(true),
    )
    expect(wrapper.find('[data-testid="notification-task-delete-1"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="notification-task-submit-1"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="notification-task-cancel-1"]').exists()).toBe(false)
  })
  it('loads options with an ID cursor', async () => {
    usePermissionStore().permissionCodes = [
      'message:notificationTask:list',
      'message:notificationTask:detail',
      'message:notificationTask:create',
    ]
    const wrapper = mount(Page, {
      global: {
        plugins: [appI18n],
        stubs: {
          AppPage: { template: '<section><slot /></section>' },
          AppTable: { props: ['data'], template: '<div><slot name="toolbar-right" /></div>' },
          AppDialog: { template: '<div><slot /></div><slot name="footer" />' },
          NotificationEditor: true,
          ElButton: {
            template: '<button v-bind="$attrs"><slot /></button>',
          },
          ElSelectV2: true,
          ElInput: true,
          ElForm: { template: '<form><slot /></form>' },
          ElFormItem: { template: '<div><slot /></div>' },
        },
      },
    })
    await wrapper.get('[data-testid="notification-task-create"]').trigger('click')
    await vi.waitFor(() =>
      expect(api.listNotificationTaskOptions).toHaveBeenCalledWith('platform', {
        afterId: 0,
        limit: 50,
      }),
    )
    await wrapper.get('[data-testid="notification-task-option-more"]').trigger('click')
    expect(api.listNotificationTaskOptions).toHaveBeenLastCalledWith('platform', {
      afterId: 3,
      limit: 50,
    })
  })
  it('loads exact detail before editing a draft', async () => {
    usePermissionStore().permissionCodes = [
      'message:notificationTask:list',
      'message:notificationTask:detail',
      'message:notificationTask:update',
    ]
    const wrapper = mount(Page, {
      global: {
        plugins: [appI18n],
        stubs: {
          AppPage: { template: '<section><slot /></section>' },
          AppTable: {
            props: ['data'],
            template: '<div><slot v-if="data.length" name="actions" :row="data[0]" /></div>',
          },
          AppDialog: { template: '<div><slot /></div><slot name="footer" />' },
          NotificationEditor: true,
          ElButton: {
            template: '<button v-bind="$attrs"><slot /></button>',
          },
          ElSelectV2: true,
          ElInput: true,
          ElForm: { template: '<form><slot /></form>' },
          ElFormItem: { template: '<div><slot /></div>' },
        },
      },
    })
    await vi.waitFor(() =>
      expect(wrapper.find('[data-testid="notification-task-edit-1"]').exists()).toBe(true),
    )
    await wrapper.get('[data-testid="notification-task-edit-1"]').trigger('click')
    await vi.waitFor(() => expect(api.getNotificationTask).toHaveBeenCalledWith(1))
    expect(wrapper.find('[data-testid="notification-task-variant"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="notification-task-priority"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="notification-task-link-type"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="notification-task-title"]').attributes('maxlength')).toBe(
      '128',
    )
  })

  it('shows a read-only detail command for submitted tasks', async () => {
    usePermissionStore().permissionCodes = [
      'message:notificationTask:list',
      'message:notificationTask:detail',
    ]
    vi.mocked(api.listNotificationTasks).mockResolvedValue({
      list: [{ ...task, status: 'completed', completedAt: '2026-09-18T12:01:00Z' }],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    const wrapper = mount(Page, {
      global: {
        plugins: [appI18n],
        stubs: {
          AppPage: { template: '<section><slot /></section>' },
          AppTable: {
            props: ['data'],
            template: '<div><slot v-if="data.length" name="actions" :row="data[0]" /></div>',
          },
          AppDialog: true,
          ElButton: { template: '<button v-bind="$attrs"><slot /></button>' },
          ElSelectV2: true,
        },
      },
    })
    await vi.waitFor(() =>
      expect(wrapper.find('[data-testid="notification-task-detail-1"]').exists()).toBe(true),
    )
    expect(wrapper.find('[data-testid="notification-task-edit-1"]').exists()).toBe(false)
  })
})
