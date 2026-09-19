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
  getNotificationTaskForUpdate: vi.fn(),
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

function deferred<T>(): { promise: Promise<T>; resolve: (value: T) => void } {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((next) => {
    resolve = next
  })
  return { promise, resolve }
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
    vi.mocked(api.getNotificationTaskForUpdate).mockResolvedValue(task)
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
          ElSelectV2: {
            name: 'ElSelectV2',
            props: ['modelValue', 'options', 'remoteMethod'],
            emits: ['update:modelValue', 'change'],
            template: '<div v-bind="$attrs" />',
          },
          ElInput: true,
          ElForm: { template: '<form><slot /></form>' },
          ElFormItem: { template: '<div><slot /></div>' },
        },
      },
    })
    await wrapper.get('[data-testid="notification-task-create"]').trigger('click')
    const platformSelect = wrapper.getComponent({ name: 'ElSelectV2' })
    expect(platformSelect.props('modelValue')).toBeNull()
    await vi.waitFor(() =>
      expect(api.listNotificationTaskOptions).toHaveBeenCalledWith('create', 'platform', {
        afterId: 0,
        limit: 50,
      }),
    )
    await vi.waitFor(() =>
      expect(platformSelect.props('options')).toEqual([{ value: 2, label: 'Admin' }]),
    )
    platformSelect.vm.$emit('update:modelValue', 2)
    await wrapper.vm.$nextTick()
    expect(platformSelect.props('modelValue')).toBe(2)

    vi.mocked(api.listNotificationTaskOptions).mockClear()
    const remoteMethod = platformSelect.props('remoteMethod') as (keyword: string) => void
    remoteMethod('')
    await wrapper.vm.$nextTick()
    expect(api.listNotificationTaskOptions).not.toHaveBeenCalled()

    await wrapper.get('[data-testid="notification-task-option-more"]').trigger('click')
    expect(api.listNotificationTaskOptions).toHaveBeenLastCalledWith('create', 'platform', {
      afterId: 3,
      limit: 50,
    })

    const audienceSelect = wrapper
      .findAllComponents({ name: 'ElSelectV2' })
      .find((select) => select.attributes('data-testid') === 'notification-task-audience')
    if (audienceSelect === undefined) throw new Error('audience select is missing')
    audienceSelect.vm.$emit('update:modelValue', 'user')
    audienceSelect.vm.$emit('change', 'user')
    await vi.waitFor(() =>
      expect(api.listNotificationTaskOptions).toHaveBeenCalledWith('create', 'user', {
        afterId: 0,
        limit: 50,
      }),
    )
    const targetSelect = wrapper
      .findAllComponents({ name: 'ElSelectV2' })
      .find((select) => select.attributes('data-testid') === 'notification-task-targets')
    if (targetSelect === undefined) throw new Error('target select is missing')
    await vi.waitFor(() =>
      expect(targetSelect.props('options')).toEqual([{ value: 2, label: 'Admin' }]),
    )

    audienceSelect.vm.$emit('update:modelValue', 'role')
    audienceSelect.vm.$emit('change', 'role')
    await vi.waitFor(() =>
      expect(api.listNotificationTaskOptions).toHaveBeenCalledWith('create', 'role', {
        afterId: 0,
        limit: 50,
      }),
    )
    await vi.waitFor(() =>
      expect(targetSelect.props('options')).toEqual([{ value: 2, label: 'Admin' }]),
    )
  })

  it('ignores stale platform options when the draft dialog is reopened', async () => {
    usePermissionStore().permissionCodes = [
      'message:notificationTask:list',
      'message:notificationTask:create',
    ]
    const stale = deferred<api.NotificationTaskOptions>()
    vi.mocked(api.listNotificationTaskOptions)
      .mockReturnValueOnce(stale.promise)
      .mockResolvedValueOnce({ items: [{ id: 3, label: 'Current' }], nextAfterId: null })
    const wrapper = mount(Page, {
      global: {
        plugins: [appI18n],
        stubs: {
          AppPage: { template: '<section><slot /></section>' },
          AppTable: { props: ['data'], template: '<div><slot name="toolbar-right" /></div>' },
          AppDialog: { template: '<div><slot /></div><slot name="footer" />' },
          NotificationEditor: true,
          ElButton: { template: '<button v-bind="$attrs"><slot /></button>' },
          ElSelectV2: {
            name: 'ElSelectV2',
            props: ['options'],
            template: '<div v-bind="$attrs" />',
          },
          ElInput: true,
          ElForm: { template: '<form><slot /></form>' },
          ElFormItem: { template: '<div><slot /></div>' },
        },
      },
    })
    const create = wrapper.get('[data-testid="notification-task-create"]')
    await create.trigger('click')
    expect(wrapper.find('[data-testid="notification-task-option-more"]').exists()).toBe(false)
    await create.trigger('click')
    await vi.waitFor(() =>
      expect(wrapper.getComponent({ name: 'ElSelectV2' }).props('options')).toEqual([
        { value: 3, label: 'Current' },
      ]),
    )

    stale.resolve({ items: [{ id: 2, label: 'Stale' }], nextAfterId: null })
    await stale.promise
    await Promise.resolve()
    await wrapper.vm.$nextTick()
    expect(wrapper.getComponent({ name: 'ElSelectV2' }).props('options')).toEqual([
      { value: 3, label: 'Current' },
    ])
  })

  it('submits platform, status, audience, keyword, and time filters together', async () => {
    usePermissionStore().permissionCodes = ['message:notificationTask:list']
    const wrapper = mount(Page, {
      global: {
        plugins: [appI18n],
        stubs: {
          AppPage: { template: '<section><slot /></section>' },
          AppSearch: {
            emits: ['update:modelValue', 'query'],
            template: `<button data-testid="notification-task-search" @click="$emit('update:modelValue', { platformId: '2', status: 'processing', audienceType: 'role', keyword: ' maintenance ', timeRange: ['2026-09-18T00:00:00Z', '2026-09-19T00:00:00Z'] }); $emit('query')">search</button>`,
          },
          AppTable: { props: ['data'], template: '<div />' },
          AppDialog: true,
        },
      },
    })
    await vi.waitFor(() => expect(api.listNotificationTasks).toHaveBeenCalled())
    vi.mocked(api.listNotificationTasks).mockClear()
    await wrapper.get('[data-testid="notification-task-search"]').trigger('click')
    await vi.waitFor(() =>
      expect(api.listNotificationTasks).toHaveBeenCalledWith({
        page: 1,
        pageSize: 20,
        platformId: 2,
        status: 'processing',
        audienceType: 'role',
        keyword: 'maintenance',
        from: '2026-09-18T00:00:00Z',
        to: '2026-09-19T00:00:00Z',
      }),
    )
  })
  it('loads exact detail before editing a draft', async () => {
    usePermissionStore().permissionCodes = [
      'message:notificationTask:list',
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
    await vi.waitFor(() => expect(api.getNotificationTaskForUpdate).toHaveBeenCalledWith(1))
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
