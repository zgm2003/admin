import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { ElMessageBox, ElNotification } from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { appI18n, setLocale } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import * as catalogApi from '@/api/message/notificationTaskOptions'
import * as api from '@/api/message/notificationTask'
import Page from '@/views/message/notificationTask/index.vue'
import {
  notificationEditorConfig,
  notificationToolbarKeys,
} from '@/views/message/notificationTask/components/NotificationEditor/index.vue'
import NotificationTaskSearch from '@/views/message/notificationTask/components/NotificationTaskSearch/index.vue'
import { taskCatalog, draftActions, completedActions } from './fixtures'

enableAutoUnmount(afterEach)

vi.mock('@/api/message/notificationTaskOptions', () => ({ getNotificationTaskOptions: vi.fn() }))

vi.mock('@/api/message/notificationTask', async (original) => ({
  ...(await original()),
  listNotificationTasks: vi.fn(),
  getNotificationTask: vi.fn(),
  getNotificationTaskForUpdate: vi.fn(),
  listNotificationTaskOptions: vi.fn(),
  createNotificationTask: vi.fn(),
  updateNotificationTask: vi.fn(),
  commandNotificationTask: vi.fn(),
  deleteNotificationTask: vi.fn(),
}))

const task: api.NotificationTask = {
  actions: draftActions,
  id: 1,
  platformId: 2,
  platformName: 'Canvas',
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
  status: 1,
  generatedCount: 0,
  createdBy: 3,
  createdAt: '2026-09-18T12:00:00Z',
  updatedAt: '2026-09-18T12:00:00Z',
}

function deferred<T>(): {
  promise: Promise<T>
  resolve: (value: T) => void
  reject: (reason: unknown) => void
} {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((next, fail) => {
    resolve = next
    reject = fail
  })
  return { promise, resolve, reject }
}

describe('notification task management', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    setLocale('zh-CN')
    vi.mocked(catalogApi.getNotificationTaskOptions).mockResolvedValue(taskCatalog)
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
    vi.mocked(api.createNotificationTask).mockResolvedValue(task)
    vi.mocked(api.updateNotificationTask).mockResolvedValue(task)
    vi.mocked(api.commandNotificationTask).mockResolvedValue(task)
    vi.mocked(api.deleteNotificationTask).mockResolvedValue()
  })
  afterEach(() => vi.restoreAllMocks())

  it('uses backend defaults and title constraints rather than the first option', async () => {
    vi.mocked(catalogApi.getNotificationTaskOptions).mockResolvedValue({
      ...taskCatalog,
      audiences: [...taskCatalog.audiences].reverse(),
      defaults: {
        audienceType: 'platform',
        variant: 'future-variant',
        priority: 'future-priority',
        linkType: 'future-link',
      },
      constraints: { titleMaxLength: 77 },
    })
    const wrapper = mountControls(['message:notificationTask:create'])
    await flushPromises()
    wrapper.getComponent({ name: 'NotificationTaskTable' }).vm.$emit('create')
    await flushPromises()
    const dialog = wrapper.getComponent({ name: 'NotificationTaskDialog' })
    expect(dialog.props('form')).toMatchObject({
      audienceType: 'platform',
      variant: 'future-variant',
      priority: 'future-priority',
      linkType: 'future-link',
    })
    expect(dialog.props('titleMaxLength')).toBe(77)
  })

  it('blocks draft creation until backend options succeed and exposes a retry', async () => {
    vi.mocked(catalogApi.getNotificationTaskOptions).mockRejectedValueOnce(new Error('目录失败'))
    const wrapper = mountControls(['message:notificationTask:create'])
    await flushPromises()
    const table = wrapper.getComponent({ name: 'NotificationTaskTable' })
    table.vm.$emit('create')
    await flushPromises()
    expect(table.props('createDisabled')).toBe(true)
    expect(wrapper.getComponent({ name: 'NotificationTaskDialog' }).props('modelValue')).toBe(false)
    expect(wrapper.text()).toContain('目录失败')
    await wrapper.get('[data-testid="notification-task-options-retry"]').trigger('click')
    await flushPromises()
    expect(table.props('createDisabled')).toBe(false)
  })

  it('does not submit a draft twice while the first save is pending', async () => {
    const pending = deferred<api.NotificationTask>()
    vi.mocked(api.createNotificationTask).mockReturnValueOnce(pending.promise)
    vi.spyOn(ElNotification, 'success').mockImplementation(() => ({ close: () => undefined }))
    const wrapper = mountControls(['message:notificationTask:create'])
    await flushPromises()
    wrapper.getComponent({ name: 'NotificationTaskTable' }).vm.$emit('create')
    const dialog = wrapper.getComponent({ name: 'NotificationTaskDialog' })
    dialog.vm.$emit('update:form', { platformId: 2 })
    dialog.vm.$emit('save')
    dialog.vm.$emit('save')
    await flushPromises()
    expect(api.createNotificationTask).toHaveBeenCalledOnce()
    expect(dialog.props('saving')).toBe(true)
    pending.resolve(task)
    await flushPromises()
    expect(dialog.props('modelValue')).toBe(false)
  })

  it.each(['success', 'failure'] as const)(
    'does not let a stale save %s overwrite a newly opened draft',
    async (outcome) => {
      const pending = deferred<api.NotificationTask>()
      vi.mocked(api.createNotificationTask).mockReturnValueOnce(pending.promise)
      vi.spyOn(ElNotification, 'success').mockImplementation(() => ({ close: () => undefined }))
      const wrapper = mountControls(['message:notificationTask:create'])
      await flushPromises()
      const table = wrapper.getComponent({ name: 'NotificationTaskTable' })
      const dialog = wrapper.getComponent({ name: 'NotificationTaskDialog' })
      table.vm.$emit('create')
      dialog.vm.$emit('update:form', { platformId: 2 })
      dialog.vm.$emit('save')
      await flushPromises()
      dialog.vm.$emit('update:modelValue', false)
      table.vm.$emit('create')
      dialog.vm.$emit('update:form', { title: 'Current draft' })
      await flushPromises()
      if (outcome === 'success') pending.resolve(task)
      else pending.reject(new Error('Obsolete save failure'))
      await flushPromises()
      expect(dialog.props('modelValue')).toBe(true)
      expect(dialog.props('form')).toMatchObject({ title: 'Current draft', platformId: null })
      expect(dialog.props('errorMessage')).toBe('')
    },
  )

  it('locks row commands during confirmation without checking a frontend status list', async () => {
    const confirmation = deferred<Awaited<ReturnType<typeof ElMessageBox.confirm>>>()
    vi.spyOn(ElMessageBox, 'confirm').mockReturnValueOnce(confirmation.promise)
    vi.spyOn(ElNotification, 'success').mockImplementation(() => ({ close: () => undefined }))
    const wrapper = mountControls(['message:notificationTask:submit'])
    await flushPromises()
    const table = wrapper.getComponent({ name: 'NotificationTaskTable' })
    table.vm.$emit('command', { ...task, status: 99 }, 'submit')
    table.vm.$emit('command', { ...task, status: 99 }, 'submit')
    await flushPromises()
    expect(ElMessageBox.confirm).toHaveBeenCalledOnce()
    expect(table.props('busyIds')).toEqual([1])
    confirmation.resolve(
      Object.assign('confirm' as const, { value: '', action: 'confirm' as const }),
    )
    await flushPromises()
    expect(api.commandNotificationTask).toHaveBeenCalledExactlyOnceWith(1, 'submit')
    expect(table.props('busyIds')).toEqual([])
  })

  it('checks backend actions and exact action permission before executing a row command', async () => {
    const wrapper = mountControls(['message:notificationTask:submit'])
    await flushPromises()
    const table = wrapper.getComponent({ name: 'NotificationTaskTable' })
    table.vm.$emit('command', { ...task, actions: { ...draftActions, submit: false } }, 'submit')
    table.vm.$emit('command', { ...task, actions: { ...draftActions, copy: true } }, 'copy')
    table.vm.$emit('remove', task)
    table.vm.$emit('edit', task)
    await flushPromises()
    expect(api.commandNotificationTask).not.toHaveBeenCalled()
    expect(api.deleteNotificationTask).not.toHaveBeenCalled()
    expect(api.getNotificationTaskForUpdate).not.toHaveBeenCalled()
  })

  it('shows command failures inline, unlocks the row, and allows retry without a second notification', async () => {
    vi.mocked(api.commandNotificationTask).mockRejectedValueOnce(new Error('复制失败'))
    const error = vi
      .spyOn(ElNotification, 'error')
      .mockImplementation(() => ({ close: () => undefined }))
    vi.spyOn(ElNotification, 'success').mockImplementation(() => ({ close: () => undefined }))
    const wrapper = mountControls(['message:notificationTask:copy'])
    await flushPromises()
    const table = wrapper.getComponent({ name: 'NotificationTaskTable' })
    table.vm.$emit('command', { ...task, actions: completedActions }, 'copy')
    await flushPromises()
    expect(table.props('errorMessage')).toBe('复制失败')
    expect(table.props('busyIds')).toEqual([])
    expect(error).not.toHaveBeenCalled()
    table.vm.$emit('command', { ...task, actions: completedActions }, 'copy')
    await flushPromises()
    expect(api.commandNotificationTask).toHaveBeenCalledTimes(2)
  })

  it('does not open a copied task for editing when the returned action forbids it', async () => {
    vi.mocked(api.commandNotificationTask).mockResolvedValue({
      ...task,
      id: 2,
      actions: completedActions,
    })
    vi.spyOn(ElNotification, 'success').mockImplementation(() => ({ close: () => undefined }))
    const wrapper = mountControls([
      'message:notificationTask:copy',
      'message:notificationTask:update',
    ])
    await flushPromises()
    wrapper
      .getComponent({ name: 'NotificationTaskTable' })
      .vm.$emit('command', { ...task, actions: completedActions }, 'copy')
    await flushPromises()
    expect(api.getNotificationTaskForUpdate).not.toHaveBeenCalled()
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
  it('keeps status out of the search form because status is filtered by tabs', () => {
    const wrapper = mount(NotificationTaskSearch, {
      props: {
        modelValue: { keyword: '', platformId: '', audienceType: '', timeRange: [] },
        audienceOptions: taskCatalog.audiences,
      },
      global: {
        plugins: [appI18n],
        stubs: {
          AppSearch: {
            name: 'AppSearch',
            props: ['fields'],
            template: '<div />',
          },
        },
      },
    })

    expect(
      (wrapper.getComponent({ name: 'AppSearch' }).props('fields') as Array<{ key: string }>).map(
        (field) => field.key,
      ),
    ).toEqual(['keyword', 'platformId', 'audienceType', 'timeRange'])
  })
  it('filters the real list by status tabs and resets pagination', async () => {
    usePermissionStore().permissionCodes = ['message:notificationTask:list']
    const wrapper = mount(Page, {
      global: {
        plugins: [appI18n],
        stubs: {
          AppPage: { template: '<section><slot /></section>' },
          NotificationTaskSearch: true,
          NotificationTaskTable: {
            name: 'NotificationTaskTable',
            props: ['pagination'],
            emits: ['update:pagination'],
            template:
              '<button data-testid="notification-task-page-three" @click="$emit(\'update:pagination\', { currentPage: 3, pageSize: 20, total: 1 })" />',
          },
          AppDialog: true,
          ElTabs: {
            name: 'ElTabs',
            props: ['modelValue'],
            emits: ['update:modelValue', 'tabChange'],
            template: '<div><slot /></div>',
          },
          ElTabPane: {
            name: 'ElTabPane',
            props: ['name', 'label'],
            template: '<div />',
          },
        },
      },
    })
    await vi.waitFor(() => expect(api.listNotificationTasks).toHaveBeenCalledOnce())

    expect(wrapper.findAllComponents({ name: 'ElTabPane' }).map((tab) => tab.props())).toEqual([
      expect.objectContaining({ name: '', label: '全部' }),
      expect.objectContaining({ name: 1, label: '草稿' }),
      expect.objectContaining({ name: 2, label: '待调度' }),
      expect.objectContaining({ name: 3, label: '已入队' }),
      expect.objectContaining({ name: 4, label: '处理中' }),
      expect.objectContaining({ name: 5, label: '已完成' }),
      expect.objectContaining({ name: 6, label: '失败' }),
      expect.objectContaining({ name: 7, label: '已取消' }),
      expect.objectContaining({ name: 99, label: '未来任务状态' }),
    ])

    await wrapper.get('[data-testid="notification-task-page-three"]').trigger('click')
    await vi.waitFor(() =>
      expect(api.listNotificationTasks).toHaveBeenLastCalledWith({ page: 3, pageSize: 20 }),
    )

    const tabs = wrapper.getComponent({ name: 'ElTabs' })
    tabs.vm.$emit('update:modelValue', 1)
    tabs.vm.$emit('tabChange', 1)
    await vi.waitFor(() =>
      expect(api.listNotificationTasks).toHaveBeenLastCalledWith({
        page: 1,
        pageSize: 20,
        status: 1,
      }),
    )

    tabs.vm.$emit('update:modelValue', '')
    tabs.vm.$emit('tabChange', '')
    await vi.waitFor(() =>
      expect(api.listNotificationTasks).toHaveBeenLastCalledWith({ page: 1, pageSize: 20 }),
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
              '<div><slot name="toolbar-right" /><slot v-if="data.length" name="cell-actions" :row="data[0]" /></div>',
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
          AppDialog: { template: '<div><slot /><slot name="footer" /></div>' },
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
    await flushPromises()
    await wrapper.get('[data-testid="notification-task-create"]').trigger('click')
    const platformSelect = wrapper
      .findAllComponents({ name: 'ElSelectV2' })
      .find((select) => select.attributes('data-testid') === 'notification-task-platform')
    if (platformSelect === undefined) throw new Error('platform select is missing')
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
        platformId: 2,
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
        platformId: 2,
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
          AppDialog: { template: '<div><slot /><slot name="footer" /></div>' },
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
    await flushPromises()
    const create = wrapper.get('[data-testid="notification-task-create"]')
    await create.trigger('click')
    expect(wrapper.find('[data-testid="notification-task-option-more"]').exists()).toBe(false)
    await create.trigger('click')
    await vi.waitFor(() =>
      expect(
        wrapper
          .findAllComponents({ name: 'ElSelectV2' })
          .find((select) => select.attributes('data-testid') === 'notification-task-platform')
          ?.props('options'),
      ).toEqual([{ value: 3, label: 'Current' }]),
    )

    stale.resolve({ items: [{ id: 2, label: 'Stale' }], nextAfterId: null })
    await stale.promise
    await Promise.resolve()
    await wrapper.vm.$nextTick()
    expect(
      wrapper
        .findAllComponents({ name: 'ElSelectV2' })
        .find((select) => select.attributes('data-testid') === 'notification-task-platform')
        ?.props('options'),
    ).toEqual([{ value: 3, label: 'Current' }])
  })

  it('does not let a closed edit request replace a newly opened draft', async () => {
    usePermissionStore().permissionCodes = [
      'message:notificationTask:list',
      'message:notificationTask:create',
      'message:notificationTask:update',
    ]
    const stale = deferred<api.NotificationTask>()
    vi.mocked(api.getNotificationTaskForUpdate).mockReturnValueOnce(stale.promise)
    const wrapper = mount(Page, {
      global: {
        plugins: [appI18n],
        stubs: {
          AppPage: { template: '<section><slot /></section>' },
          AppTable: {
            props: ['data'],
            template:
              '<div><slot name="toolbar-right" /><slot v-if="data.length" name="cell-actions" :row="data[0]" /></div>',
          },
          AppDialog: {
            name: 'AppDialog',
            props: ['modelValue'],
            emits: ['update:modelValue'],
            template: '<div><slot /><slot name="footer" /></div>',
          },
          NotificationEditor: true,
          ElButton: { template: '<button v-bind="$attrs"><slot /></button>' },
          ElSelectV2: {
            name: 'ElSelectV2',
            props: ['modelValue', 'options'],
            template: '<div v-bind="$attrs" />',
          },
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

    wrapper.getComponent({ name: 'AppDialog' }).vm.$emit('update:modelValue', false)
    await wrapper.vm.$nextTick()
    await wrapper.get('[data-testid="notification-task-create"]').trigger('click')

    const platformSelect = wrapper
      .findAllComponents({ name: 'ElSelectV2' })
      .find((select) => select.attributes('data-testid') === 'notification-task-platform')
    if (platformSelect === undefined) throw new Error('platform select is missing')
    expect(platformSelect.props('modelValue')).toBeNull()

    stale.resolve({ ...task, platformId: 99, title: 'Stale edit' })
    await stale.promise
    await Promise.resolve()
    await wrapper.vm.$nextTick()
    expect(platformSelect.props('modelValue')).toBeNull()
  })

  it('submits platform, audience, keyword, and time filters together', async () => {
    usePermissionStore().permissionCodes = ['message:notificationTask:list']
    const wrapper = mount(Page, {
      global: {
        plugins: [appI18n],
        stubs: {
          AppPage: { template: '<section><slot /></section>' },
          AppSearch: {
            emits: ['update:modelValue', 'query'],
            template: `<button data-testid="notification-task-search" @click="$emit('update:modelValue', { platformId: '2', audienceType: 'role', keyword: ' maintenance ', timeRange: ['2026-09-18T00:00:00Z', '2026-09-19T00:00:00Z'] }); $emit('query')">search</button>`,
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
        audienceType: 'role',
        keyword: 'maintenance',
        from: '2026-09-18T00:00:00Z',
        to: '2026-09-19T00:00:00Z',
      }),
    )
  })

  it('keeps the newest task list when an older request finishes later', async () => {
    usePermissionStore().permissionCodes = ['message:notificationTask:list']
    const stale = deferred<api.NotificationTaskPage>()
    vi.mocked(api.listNotificationTasks)
      .mockReturnValueOnce(stale.promise)
      .mockResolvedValueOnce({
        list: [{ ...task, id: 2, title: 'Current result' }],
        total: 1,
        page: 1,
        pageSize: 20,
      })
    const wrapper = mount(Page, {
      global: {
        plugins: [appI18n],
        stubs: {
          AppPage: { template: '<section><slot /></section>' },
          AppSearch: {
            emits: ['update:modelValue', 'query'],
            template: `<button data-testid="notification-task-search" @click="$emit('update:modelValue', { keyword: 'current' }); $emit('query')">search</button>`,
          },
          AppTable: {
            props: ['data'],
            template: '<div>{{ data.map((row) => row.title).join(",") }}</div>',
          },
          AppDialog: true,
        },
      },
    })

    await wrapper.get('[data-testid="notification-task-search"]').trigger('click')
    await vi.waitFor(() => expect(wrapper.text()).toContain('Current result'))
    stale.resolve({
      list: [{ ...task, title: 'Stale result' }],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    await stale.promise
    await Promise.resolve()
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Current result')
    expect(wrapper.text()).not.toContain('Stale result')
  })

  it('notifies, closes the dialog, and reloads after saving a draft', async () => {
    usePermissionStore().permissionCodes = [
      'message:notificationTask:list',
      'message:notificationTask:create',
    ]
    const success = vi.spyOn(ElNotification, 'success').mockImplementation(() => ({
      close: () => undefined,
    }))
    const wrapper = mount(Page, {
      global: {
        plugins: [appI18n],
        stubs: {
          AppPage: { template: '<section><slot /></section>' },
          AppTable: { props: ['data'], template: '<div><slot name="toolbar-right" /></div>' },
          AppDialog: {
            name: 'AppDialog',
            props: ['modelValue'],
            emits: ['update:modelValue'],
            template: '<div><slot /><slot name="footer" /></div>',
          },
          NotificationEditor: true,
          ElButton: { template: '<button v-bind="$attrs"><slot /></button>' },
          ElSelectV2: {
            name: 'ElSelectV2',
            props: ['modelValue'],
            emits: ['update:modelValue'],
            template: '<div v-bind="$attrs" />',
          },
          ElInput: true,
          ElForm: { template: '<form><slot /></form>' },
          ElFormItem: { template: '<div><slot /></div>' },
        },
      },
    })
    await vi.waitFor(() => expect(api.listNotificationTasks).toHaveBeenCalledTimes(1))
    await wrapper.get('[data-testid="notification-task-create"]').trigger('click')
    const platform = wrapper
      .findAllComponents({ name: 'ElSelectV2' })
      .find((select) => select.attributes('data-testid') === 'notification-task-platform')
    if (platform === undefined) throw new Error('platform select is missing')
    platform.vm.$emit('update:modelValue', 2)
    await wrapper.vm.$nextTick()

    const save = wrapper.findAll('button').find((button) => button.text().includes('保存'))
    if (save === undefined) throw new Error('save button is missing')
    await save.trigger('click')
    await vi.waitFor(() => expect(api.createNotificationTask).toHaveBeenCalledOnce())
    await vi.waitFor(() => expect(success).toHaveBeenCalledOnce())

    expect(wrapper.getComponent({ name: 'AppDialog' }).props('modelValue')).toBe(false)
    expect(api.listNotificationTasks).toHaveBeenCalledTimes(2)
  })

  it('keeps the dialog open and shows a backend save failure without notifying it twice', async () => {
    usePermissionStore().permissionCodes = [
      'message:notificationTask:list',
      'message:notificationTask:create',
    ]
    vi.mocked(api.createNotificationTask).mockRejectedValue(new Error('后端保存失败'))
    const success = vi.spyOn(ElNotification, 'success').mockImplementation(() => ({
      close: () => undefined,
    }))
    const error = vi.spyOn(ElNotification, 'error').mockImplementation(() => ({
      close: () => undefined,
    }))
    const wrapper = mount(Page, {
      global: {
        plugins: [appI18n],
        stubs: {
          AppPage: { template: '<section><slot /></section>' },
          AppTable: { props: ['data'], template: '<div><slot name="toolbar-right" /></div>' },
          AppDialog: {
            name: 'AppDialog',
            props: ['modelValue'],
            emits: ['update:modelValue'],
            template: '<div><slot /><slot name="footer" /></div>',
          },
          NotificationEditor: true,
          ElButton: { template: '<button v-bind="$attrs"><slot /></button>' },
          ElSelectV2: {
            name: 'ElSelectV2',
            props: ['modelValue'],
            emits: ['update:modelValue'],
            template: '<div v-bind="$attrs" />',
          },
          ElInput: true,
          ElForm: { template: '<form><slot /></form>' },
          ElFormItem: { template: '<div><slot /></div>' },
        },
      },
    })
    await vi.waitFor(() => expect(api.listNotificationTasks).toHaveBeenCalledTimes(1))
    await wrapper.get('[data-testid="notification-task-create"]').trigger('click')
    const platform = wrapper
      .findAllComponents({ name: 'ElSelectV2' })
      .find((select) => select.attributes('data-testid') === 'notification-task-platform')
    if (platform === undefined) throw new Error('platform select is missing')
    platform.vm.$emit('update:modelValue', 2)
    await wrapper.vm.$nextTick()

    const save = wrapper.findAll('button').find((button) => button.text().includes('保存'))
    if (save === undefined) throw new Error('save button is missing')
    await save.trigger('click')
    await vi.waitFor(() => expect(wrapper.text()).toContain('后端保存失败'))
    expect(error).not.toHaveBeenCalled()
    expect(success).not.toHaveBeenCalled()
    expect(wrapper.getComponent({ name: 'AppDialog' }).props('modelValue')).toBe(true)
    expect(wrapper.find('[data-testid="notification-task-title"]').exists()).toBe(true)
    expect(api.listNotificationTasks).toHaveBeenCalledTimes(1)
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
            template: '<div><slot v-if="data.length" name="cell-actions" :row="data[0]" /></div>',
          },
          AppDialog: { template: '<div><slot /><slot name="footer" /></div>' },
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
      list: [
        {
          ...task,
          status: 5,
          actions: completedActions,
          completedAt: '2026-09-18T12:01:00Z',
        },
      ],
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
            template: '<div><slot v-if="data.length" name="cell-actions" :row="data[0]" /></div>',
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

function mountControls(permissions: string[]) {
  usePermissionStore().permissionCodes = permissions
  return mount(Page, {
    global: {
      plugins: [appI18n],
      stubs: {
        AppPage: { template: '<section><slot /></section>' },
        NotificationTaskSearch: true,
        NotificationTaskStatusTabs: true,
        NotificationTaskTable: {
          name: 'NotificationTaskTable',
          props: ['createDisabled', 'busyIds', 'errorMessage'],
          emits: ['create', 'command', 'remove', 'edit'],
          template: '<div />',
        },
        NotificationTaskDialog: {
          name: 'NotificationTaskDialog',
          props: ['modelValue', 'form', 'saving', 'titleMaxLength', 'errorMessage'],
          emits: ['save', 'update:form', 'update:modelValue'],
          template: '<div />',
        },
        ElAlert: { props: ['title'], template: '<div>{{ title }}<slot /></div>' },
        ElButton: { template: '<button v-bind="$attrs"><slot /></button>' },
      },
    },
  })
}
