import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { describe, expect, it } from 'vitest'
import { ref } from 'vue'

import AppDialog from './index.vue'

describe('AppDialog', () => {
  it('renders slots and emits controlled close state', async () => {
    const visible = ref(true)
    const wrapper = mount(AppDialog, {
      attachTo: document.body,
      props: { modelValue: visible.value, title: 'Edit user', 'onUpdate:modelValue': (value: boolean) => { visible.value = value } },
      slots: { default: '<p class="dialog-content">Content</p>', header: '<strong>Custom header</strong>', footer: '<button>Save</button>' },
      global: { plugins: [ElementPlus] },
    })
    await flushPromises()
    expect(document.body.textContent).toContain('Custom header')
    expect(document.body.textContent).toContain('Content')
    expect(document.body.textContent).toContain('Save')
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
      props: { modelValue: false, height: 560, description: 'Edit details' },
      global: { plugins: [ElementPlus] },
    })
    await wrapper.setProps({ modelValue: true })
    await flushPromises()
    expect(document.body.querySelector('.el-scrollbar')).not.toBeNull()
    wrapper.findComponent({ name: 'ElDialog' }).vm.$emit('closed')
    await flushPromises()
    expect(document.activeElement).toBe(trigger)
    wrapper.unmount()
  })
})
