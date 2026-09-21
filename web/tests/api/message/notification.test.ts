import { describe, expect, it } from 'vitest'

import {
  notificationPriorityMetadata,
  notificationVariantMetadata,
  parseNotificationList,
  parseNotificationSummary,
} from '@/api/message/notification'

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
  it('owns stable variant and priority values in domain metadata', () => {
    expect(notificationVariantMetadata.map((item) => item.value)).toEqual([
      'info',
      'success',
      'warning',
      'error',
    ])
    expect(notificationPriorityMetadata.map((item) => item.value)).toEqual(['normal', 'urgent'])
  })
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

  it('accepts only the sanitized notification HTML contract', () => {
    const safeHtml =
      '<h2>Title</h2><p>Hello <strong>bold</strong><em>em</em><u>u</u><br><a href="https://example.com/a?q=1" target="_blank" rel="noopener noreferrer">link</a></p><ul><li>one</li></ul><ol><li>two</li></ol>'
    expect(
      parseNotificationList({
        items: [{ ...recent, contentHtml: safeHtml }],
        nextBeforeId: null,
      }).items[0]?.contentHtml,
    ).toBe(safeHtml)
  })

  it.each([
    '<p onclick="alert(1)">event handler</p>',
    '<p><img src=x onerror="alert(1)">image</p>',
    '<p><a href="javascript:alert(1)">link</a></p>',
    '<p><a href="https://example.com" target="_self" rel="opener">link</a></p>',
    '<svg><script>alert(1)</script></svg>',
  ])('rejects notification HTML outside the backend sanitizer contract: %s', (contentHtml) => {
    expect(() =>
      parseNotificationList({
        items: [{ ...recent, contentHtml }],
        nextBeforeId: null,
      }),
    ).toThrow()
  })
})
