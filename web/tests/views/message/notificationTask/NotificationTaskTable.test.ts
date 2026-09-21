import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import {
  NotificationTaskStatus,
  type NotificationTaskListItem,
} from '@/api/message/notificationTask'
import { appI18n } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import { formatTime } from '@/utils/datetime'
import NotificationTaskTable from '@/views/message/notificationTask/components/NotificationTaskTable/index.vue'

const draft: NotificationTaskListItem = {
  id: 9,
  platformId: 1,
  platformName: 'Admin',
  title: 'Draft notice',
  variant: 'info',
  priority: 'normal',
  audienceType: 'platform',
  scheduledAt: null,
  submittedAt: null,
  completedAt: null,
  status: NotificationTaskStatus.Draft,
  generatedCount: 0,
  updatedAt: '2026-09-19T08:00:00Z',
}

describe('NotificationTaskTable', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('renders draft commands through the AppTable cell slot contract', async () => {
    usePermissionStore().permissionCodes = [
      'message:notificationTask:detail',
      'message:notificationTask:update',
      'message:notificationTask:delete',
      'message:notificationTask:submit',
    ]
    const wrapper = mount(NotificationTaskTable, {
      props: {
        rows: [draft],
        loading: false,
        errorMessage: '',
        pagination: { currentPage: 1, pageSize: 20, total: 1 },
      },
      global: { plugins: [ElementPlus, appI18n] },
    })
    await flushPromises()

    expect(wrapper.get('[data-testid="notification-task-detail-9"]').text()).toBe('详情')
    expect(wrapper.get('[data-testid="notification-task-edit-9"]').text()).toBe('编辑')
    expect(wrapper.get('[data-testid="notification-task-delete-9"]').text()).toBe('删除')
    expect(wrapper.get('[data-testid="notification-task-submit-9"]').text()).toBe('提交')
    expect(wrapper.text()).toContain('Admin')
    expect(wrapper.get('[data-testid="notification-task-detail-9"]').classes()).toContain(
      'el-button--primary',
    )
    expect(wrapper.get('[data-testid="notification-task-edit-9"]').classes()).toContain(
      'el-button--warning',
    )
    expect(wrapper.get('[data-testid="notification-task-delete-9"]').classes()).toContain(
      'el-button--danger',
    )
    expect(wrapper.get('[data-testid="notification-task-submit-9"]').classes()).toContain(
      'el-button--success',
    )
  })

  it('uses semantic button colors for cancellation and copying', async () => {
    usePermissionStore().permissionCodes = [
      'message:notificationTask:cancel',
      'message:notificationTask:copy',
    ]
    const wrapper = mount(NotificationTaskTable, {
      props: {
        rows: [
          {
            ...draft,
            id: 10,
            status: NotificationTaskStatus.Scheduled,
            scheduledAt: '2026-09-20T08:00:00Z',
          },
          {
            ...draft,
            id: 11,
            status: NotificationTaskStatus.Completed,
            completedAt: '2026-09-19T09:00:00Z',
          },
        ],
        loading: false,
        errorMessage: '',
        pagination: { currentPage: 1, pageSize: 20, total: 2 },
      },
      global: { plugins: [ElementPlus, appI18n] },
    })
    await flushPromises()

    expect(wrapper.get('[data-testid="notification-task-cancel-10"]').classes()).toContain(
      'el-button--warning',
    )
    expect(wrapper.get('[data-testid="notification-task-copy-10"]').classes()).toContain(
      'el-button--primary',
    )
    expect(wrapper.get('[data-testid="notification-task-copy-11"]').classes()).toContain(
      'el-button--primary',
    )
  })

  it('renders protocol values as localized business values', async () => {
    const completed: NotificationTaskListItem = {
      ...draft,
      id: 11,
      audienceType: 'role',
      status: NotificationTaskStatus.Completed,
      generatedCount: 1,
      submittedAt: '2026-09-19T08:30:00Z',
      completedAt: '2026-09-19T09:00:00Z',
    }
    const wrapper = mount(NotificationTaskTable, {
      props: {
        rows: [draft, completed],
        loading: false,
        errorMessage: '',
        pagination: { currentPage: 1, pageSize: 20, total: 2 },
      },
      global: { plugins: [ElementPlus, appI18n] },
    })
    await flushPromises()

    expect(wrapper.get('[data-testid="notification-task-audience-9"]').text()).toBe('整个平台')
    expect(wrapper.get('[data-testid="notification-task-status-9"]').text()).toBe('草稿')
    expect(
      wrapper.get('[data-testid="notification-task-status-9"]').get('.el-tag').classes(),
    ).toContain('el-tag--info')
    expect(wrapper.get('[data-testid="notification-task-generated-9"]').text()).toBe('尚未生成')
    expect(wrapper.get('[data-testid="notification-task-scheduled-9"]').text()).toBe('-')

    expect(wrapper.get('[data-testid="notification-task-audience-11"]').text()).toBe('指定角色')
    expect(wrapper.get('[data-testid="notification-task-status-11"]').text()).toBe('已完成')
    expect(
      wrapper.get('[data-testid="notification-task-status-11"]').get('.el-tag').classes(),
    ).toContain('el-tag--success')
    expect(wrapper.get('[data-testid="notification-task-generated-11"]').text()).toBe('1 条')
    expect(wrapper.get('[data-testid="notification-task-submitted-11"]').text()).toBe(
      formatTime(completed.submittedAt!),
    )
    expect(wrapper.get('[data-testid="notification-task-completed-11"]').text()).toBe(
      formatTime(completed.completedAt!),
    )
    expect(wrapper.text()).not.toContain('T08:30:00Z')
  })
})
