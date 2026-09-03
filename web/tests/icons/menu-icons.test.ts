import { describe, expect, it } from 'vitest'

import { isMenuIconName, menuIcons } from '@/icons/menu-icons'

describe('menu icons', () => {
  it('resolves the message service icon used by the Admin menu migration', () => {
    expect(isMenuIconName('lucide:message-square-more')).toBe(true)
    expect(isMenuIconName('lucide:mail')).toBe(true)
    expect(menuIcons['lucide:message-square-more'].label).toBe('对话')
    expect(menuIcons['lucide:mail'].label).toBe('邮件')
  })
})
