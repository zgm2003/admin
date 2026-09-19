import { describe, expect, it } from 'vitest'

import {
  parseNotificationTask,
  parseNotificationTaskOptions,
  parseNotificationTaskPage,
} from '@/api/message/notificationTask'

const task = {
  id: 1,
  platformId: 2,
  platformName: 'Canvas',
  notificationId: null,
  title: 't',
  contentHtml: '<p>x</p>',
  summary: 'x',
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

describe('notification task DTO', () => {
  it('parses nullable task fields and options', () => {
    expect(parseNotificationTask(task).status).toBe('draft')
    expect(
      parseNotificationTaskOptions({ items: [{ id: 2, label: 'Admin' }], nextAfterId: null })
        .items[0]?.label,
    ).toBe('Admin')
  })
  it('rejects unknown and invalid enum values', () => {
    expect(() => parseNotificationTask({ ...task, revision: 1 })).toThrow()
    expect(() => parseNotificationTask({ ...task, status: 'unknown' })).toThrow()
  })
  it('parses a list projection without accepting detail fields', () => {
    const page = parseNotificationTaskPage({
      list: [
        {
          id: 1,
          platformId: 2,
          platformName: 'Canvas',
          title: 't',
          variant: 'info',
          priority: 'normal',
          audienceType: 'platform',
          scheduledAt: null,
          submittedAt: null,
          completedAt: '2026-09-18T12:01:00Z',
          status: 'completed',
          generatedCount: 1,
          updatedAt: '2026-09-18T12:01:00Z',
        },
      ],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    expect(page.list[0]?.completedAt).toBe('2026-09-18T12:01:00Z')
    expect(page.list[0]?.platformName).toBe('Canvas')
    expect(() =>
      parseNotificationTaskPage({
        ...page,
        list: [{ ...page.list[0], contentHtml: '<p>must not be in list</p>' }],
      }),
    ).toThrow()
  })
})
