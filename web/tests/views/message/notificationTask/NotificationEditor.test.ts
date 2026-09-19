import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import NotificationEditor from '@/views/message/notificationTask/components/NotificationEditor/index.vue'

describe('NotificationEditor', () => {
  it('forwards consecutive editor updates through v-model', async () => {
    const wrapper = mount(NotificationEditor, {
      props: { modelValue: '' },
      global: {
        stubs: {
          Editor: {
            name: 'Editor',
            props: ['modelValue'],
            emits: ['update:modelValue', 'onCreated'],
            template: '<div />',
          },
          Toolbar: true,
        },
      },
    })
    const editor = wrapper.getComponent({ name: 'Editor' })

    editor.vm.$emit('update:modelValue', '<p>First</p>')
    await wrapper.setProps({ modelValue: '<p>First</p>' })
    editor.vm.$emit('update:modelValue', '<p>First second</p>')

    expect(wrapper.emitted('update:modelValue')).toEqual([
      ['<p>First</p>'],
      ['<p>First second</p>'],
    ])
  })
})
