import { mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { describe, expect, it } from 'vitest'

import type { NotificationTask } from '@/api/message/notificationTask'
import { appI18n } from '@/i18n'
import NotificationTaskDetail from '@/views/message/notificationTask/components/NotificationTaskDetail/index.vue'
import { taskCatalog, draftActions } from './fixtures'

const draft = {
  actions: draftActions,
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
  status: 1,
  generatedCount: 0,
  createdBy: 3,
  createdAt: '2026-09-18T12:00:00Z',
  updatedAt: '2026-09-18T12:00:00Z',
} as NotificationTask

describe('NotificationTaskDetail', () => {
  it('shows a compact semantic summary with the real platform name', () => {
    const wrapper = mount(NotificationTaskDetail, {
      props: {
        task: draft,
        statuses: taskCatalog.statuses,
        audiences: taskCatalog.audiences,
        variants: taskCatalog.variants,
        priorities: taskCatalog.priorities,
      },
      global: { plugins: [ElementPlus, appI18n] },
    })

    expect(wrapper.get('[data-testid="notification-task-detail-title"]').text()).toBe(
      '全平台维护通知',
    )
    expect(wrapper.get('[data-testid="notification-task-detail-platform"]').text()).toBe('Admin')
    expect(wrapper.get('[data-testid="notification-task-detail-status"]').text()).toBe('草稿')
    expect(wrapper.getComponent({ name: 'ElTag' }).props('type')).toBe('info')
    expect(wrapper.get('[data-testid="notification-task-detail-generated"]').text()).toBe('0 条')
    const descriptions = wrapper.getComponent({ name: 'ElDescriptions' })
    expect(descriptions.props('column')).toBe(2)
    expect(descriptions.props('border')).toBe(true)
    expect(wrapper.find('dl').exists()).toBe(false)
    expect(wrapper.findAll('.el-descriptions__label').map((item) => item.text())).toContain('平台')
    expect(wrapper.get('[data-testid="notification-task-detail-content"]').text()).toContain(
      '今晚进行系统维护。',
    )
  })

  it('keeps a submitted zero count visible as a real business value', () => {
    const wrapper = mount(NotificationTaskDetail, {
      props: {
        statuses: taskCatalog.statuses,
        audiences: taskCatalog.audiences,
        variants: taskCatalog.variants,
        priorities: taskCatalog.priorities,
        task: {
          ...draft,
          status: 3,
          submittedAt: '2026-09-18T12:01:00Z',
        },
      },
      global: { plugins: [ElementPlus, appI18n] },
    })

    expect(wrapper.get('[data-testid="notification-task-detail-generated"]').text()).toBe('0 条')
  })

  it('renders unknown and backend-added display values without guessing other entries', () => {
    const wrapper = mount(NotificationTaskDetail, {
      props: {
        task: { ...draft, status: 99, variant: 'future-variant', priority: 'future-priority' },
        statuses: taskCatalog.statuses,
        audiences: taskCatalog.audiences,
        variants: [{ value: 'future-variant', label: '未来样式' }],
        priorities: taskCatalog.priorities,
      },
      global: { plugins: [ElementPlus, appI18n] },
    })
    expect(wrapper.get('[data-testid="notification-task-detail-status"]').text()).toBe(
      '未来任务状态',
    )
    expect(wrapper.text()).toContain('未来样式')
    expect(wrapper.text()).toContain('future-priority')
  })
})
