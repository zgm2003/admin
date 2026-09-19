import { mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { describe, expect, it } from 'vitest'

import type { NotificationTask } from '@/api/message/notificationTask'
import { appI18n } from '@/i18n'
import NotificationTaskDetail from '@/views/message/notificationTask/components/NotificationTaskDetail/index.vue'

const draft = {
  id: 1,
  platformId: 1,
  platformName: 'Admin',
  notificationId: null,
  title: '全平台维护通知',
  contentHtml: '<p>今晚进行系统维护。</p>',
  summary: '今晚进行系统维护。',
  variant: 'warning',
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
} as NotificationTask

describe('NotificationTaskDetail', () => {
  it('shows a compact semantic summary with the real platform name', () => {
    const wrapper = mount(NotificationTaskDetail, {
      props: { task: draft },
      global: { plugins: [ElementPlus, appI18n] },
    })

    expect(wrapper.get('[data-testid="notification-task-detail-title"]').text()).toBe(
      '全平台维护通知',
    )
    expect(wrapper.get('[data-testid="notification-task-detail-platform"]').text()).toBe('Admin')
    expect(wrapper.get('[data-testid="notification-task-detail-status"]').text()).toBe('草稿')
    expect(wrapper.getComponent({ name: 'ElTag' }).props('type')).toBe('info')
    expect(wrapper.get('[data-testid="notification-task-detail-generated"]').text()).toBe(
      '尚未生成',
    )
    expect(wrapper.get('[data-testid="notification-task-detail-content"]').text()).toContain(
      '今晚进行系统维护。',
    )
  })

  it('keeps a submitted zero count visible as a real business value', () => {
    const wrapper = mount(NotificationTaskDetail, {
      props: { task: { ...draft, status: 'queued', submittedAt: '2026-09-18T12:01:00Z' } },
      global: { plugins: [ElementPlus, appI18n] },
    })

    expect(wrapper.get('[data-testid="notification-task-detail-generated"]').text()).toBe('0 条')
  })
})
