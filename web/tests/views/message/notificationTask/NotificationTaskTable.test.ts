import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import type { NotificationTaskListItem } from '@/api/message/notificationTask'
import { appI18n } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import NotificationTaskTable from '@/views/message/notificationTask/components/NotificationTaskTable/index.vue'

const draft: NotificationTaskListItem = {
  id: 9,
  platformId: 1,
  title: 'Draft notice',
  variant: 'info',
  priority: 'normal',
  audienceType: 'platform',
  scheduledAt: null,
  submittedAt: null,
  completedAt: null,
  status: 'draft',
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
          { ...draft, id: 10, status: 'scheduled', scheduledAt: '2026-09-20T08:00:00Z' },
          { ...draft, id: 11, status: 'completed', completedAt: '2026-09-19T09:00:00Z' },
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
})
