import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { afterEach, describe, expect, it } from 'vitest'
import { ref } from 'vue'

import AppDialog from '@/components/AppDialog/index.vue'

describe('AppDialog', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('renders slots and emits controlled close state', async () => {
    const visible = ref(true)
    const wrapper = mount(AppDialog, {
      attachTo: document.body,
      props: {
        modelValue: visible.value,
        title: 'Edit user',
        'onUpdate:modelValue': (value: boolean) => {
          visible.value = value
        },
      },
      slots: {
        default: '<p class="dialog-content">Content</p>',
        header: '<strong>Custom header</strong>',
        footer: '<button>Save</button>',
      },
      global: { plugins: [ElementPlus] },
    })
    await flushPromises()
    expect(document.body.textContent).toContain('Custom header')
    expect(document.body.textContent).toContain('Content')
    expect(document.body.textContent).toContain('Save')
    expect(document.body.querySelector<HTMLElement>('.el-scrollbar__wrap')?.style.height).toBe('')
    expect(wrapper.findComponent({ name: 'ElDialog' }).props('alignCenter')).toBe(true)
    expect(document.body.querySelector<HTMLElement>('.app-dialog__content')?.style.boxSizing).toBe(
      'border-box',
    )
    expect(document.body.querySelector('.el-dialog__footer')).not.toBeNull()
    wrapper.findComponent({ name: 'ElDialog' }).vm.$emit('update:modelValue', false)
    expect(visible.value).toBe(false)
    wrapper.unmount()
  })

  it('renders a scroll body and restores the trigger focus after close', async () => {
    const trigger = document.createElement('button')
    document.body.append(trigger)
    trigger.focus()
    const wrapper = mount(AppDialog, {
      attachTo: document.body,
      props: { modelValue: false, description: 'Edit details' },
      global: { plugins: [ElementPlus] },
    })
    await wrapper.setProps({ modelValue: true })
    await flushPromises()
    expect(document.body.querySelector('.el-scrollbar')).not.toBeNull()
    await wrapper.setProps({ modelValue: false })
    await flushPromises()
    wrapper.findComponent({ name: 'ElDialog' }).vm.$emit('closed')
    await flushPromises()
    expect(document.activeElement).toBe(trigger)
    wrapper.unmount()
    trigger.remove()
  })

  it('can opt out of viewport centering without centering header and footer text', async () => {
    const wrapper = mount(AppDialog, {
      props: { modelValue: true, center: false, top: '8vh' },
      global: { plugins: [ElementPlus] },
    })
    await flushPromises()
    const dialog = wrapper.findComponent({ name: 'ElDialog' })
    expect(dialog.props('alignCenter')).toBe(false)
    expect(dialog.props('center')).toBe(false)
    expect(dialog.props('top')).toBe('8vh')
    await wrapper.setProps({ center: true })
    expect(dialog.props('alignCenter')).toBe(true)
    wrapper.unmount()
  })

  it('centers on narrow viewports too', async () => {
    const width = window.innerWidth
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 })
    const wrapper = mount(AppDialog, {
      props: { modelValue: true },
      global: { plugins: [ElementPlus] },
    })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'ElDialog' }).props('alignCenter')).toBe(true)
    wrapper.unmount()
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: width })
  })
})
