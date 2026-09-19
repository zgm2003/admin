import { describe, expect, it } from 'vitest'

import { parseNotificationList, parseNotificationSummary } from '@/api/message/notification'

const recent = {
  id: 1,
  title: 'Title',
  summary: 'Summary',
  variant: 'info',
  priority: 'normal',
  linkType: 'none',
  link: '',
  publishedAt: '2026-09-18T12:00:00Z',
  isRead: false,
}

describe('notification DTO', () => {
  it('parses list and summary exactly', () => {
    expect(parseNotificationSummary({ unreadCount: 1, recent: [recent] }).unreadCount).toBe(1)
    expect(
      parseNotificationList({
        items: [{ ...recent, contentHtml: '<p>Body</p>' }],
        nextBeforeId: null,
      }).items,
    ).toHaveLength(1)
  })

  it('rejects summary rich content and snake case', () => {
    expect(() =>
      parseNotificationSummary({
        unreadCount: 1,
        recent: [{ ...recent, contentHtml: '<p>x</p>' }],
      }),
    ).toThrow()
    expect(() => parseNotificationSummary({ unread_count: 1, recent: [] })).toThrow()
  })
})
