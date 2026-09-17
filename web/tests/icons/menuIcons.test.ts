import { describe, expect, it } from 'vitest'

import { isMenuIconName, menuIcons } from '@/icons/menuIcons'

describe('menu icons', () => {
  it('resolves the message service icon used by the Admin menu migration', () => {
    expect(isMenuIconName('lucide:message-square-more')).toBe(true)
    expect(isMenuIconName('lucide:mail')).toBe(true)
    expect(menuIcons['lucide:message-square-more'].label).toBe('对话')
    expect(menuIcons['lucide:mail'].label).toBe('邮件')
  })

  it('resolves the system setting icon used by the settings migration', () => {
    expect(isMenuIconName('lucide:sliders-horizontal')).toBe(true)
    expect(menuIcons['lucide:sliders-horizontal'].label).toBe('系统设置')
  })

  it('resolves the task queue icon used by the queue monitor migration', () => {
    expect(isMenuIconName('lucide:list-checks')).toBe(true)
    expect(menuIcons['lucide:list-checks'].label).toBe('任务队列')
  })

  it('resolves the cache generation icon used by the cache migration', () => {
    expect(isMenuIconName('lucide:database-zap')).toBe(true)
    expect(menuIcons['lucide:database-zap'].label).toBe('配置缓存代际')
  })
})
