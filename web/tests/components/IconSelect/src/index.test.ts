import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { describe, expect, it } from 'vitest'
import IconSelect from '@src/components/IconSelect/src/index.vue'

describe('IconSelect', () => {
  const icons = [{ name: 'Folder', label: 'Folder' }, { name: 'mdi:home', label: 'Home' }]

  it('filters icons and emits the selected name', async () => {
    const wrapper = mount(IconSelect, {
      props: { modelValue: true, icons },
      attachTo: document.body,
      global: { plugins: [ElementPlus] },
    })
    await flushPromises()
    const input = document.body.querySelector('input')
    expect(input).not.toBeNull()
    input?.focus()
    if (input !== null) {
      input.value = 'home'
      input.dispatchEvent(new Event('input'))
    }
    await flushPromises()
    const item = document.body.querySelectorAll('.icon-select-item')
    expect(item).toHaveLength(1)
    item[0]?.dispatchEvent(new Event('click'))
    await flushPromises()
    expect(wrapper.emitted('select-icon')?.[0]?.[0]).toBe('mdi:home')
    expect(wrapper.emitted('update:modelValue')?.[0]?.[0]).toBe(false)
  })

  it('shows an empty state when no icon matches', async () => {
    mount(IconSelect, {
      props: { modelValue: true, icons },
      attachTo: document.body,
      global: { plugins: [ElementPlus] },
    })
    await flushPromises()
    const input = document.body.querySelector('input')
    expect(input).not.toBeNull()
    if (input !== null) {
      input.value = 'missing'
      input.dispatchEvent(new Event('input'))
    }
    await flushPromises()
    expect(document.body.textContent).toContain('暂无匹配图标')
  })
})
