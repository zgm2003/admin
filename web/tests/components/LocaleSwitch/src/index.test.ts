import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { beforeEach, describe, expect, it } from 'vitest'

import { appI18n, setLocale } from '@src/i18n'
import LocaleSwitch from '@src/components/LocaleSwitch/src/index.vue'

describe('LocaleSwitch', () => {
  beforeEach(() => {
    localStorage.clear()
    setLocale('zh-CN')
    document.body.innerHTML = ''
  })

  it('switches the shared application locale from its dropdown command', async () => {
    const wrapper = mount(LocaleSwitch, {
      attachTo: document.body,
      global: { plugins: [ElementPlus, appI18n] },
    })

    await wrapper.get('[data-testid="locale-switch"]').trigger('click')
    await flushPromises()
    getPopupItem('locale-switch-en').click()
    await wrapper.vm.$nextTick()

    expect(localStorage.getItem('admin:locale')).toBe('en-US')
    expect(document.documentElement.lang).toBe('en-US')
  })
})

function getPopupItem(testId: string): HTMLElement {
  const items = Array.from(document.body.querySelectorAll<HTMLElement>(`[data-testid="${testId}"]`))
  const item = items.at(-1)
  if (item === undefined) throw new Error(`Missing dropdown item: ${testId}`)
  return item
}
