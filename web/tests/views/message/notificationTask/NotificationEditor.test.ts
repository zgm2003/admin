import { shallowMount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { Editor } from '@wangeditor-next/editor-for-vue'

import NotificationEditor from '@/views/message/notificationTask/components/NotificationEditor/index.vue'

describe('NotificationEditor', () => {
  it('forwards consecutive editor updates through v-model', async () => {
    const wrapper = shallowMount(NotificationEditor, { props: { modelValue: '' } })
    const editor = wrapper.getComponent(Editor)

    editor.vm.$emit('update:modelValue', '<p>First</p>')
    await wrapper.setProps({ modelValue: '<p>First</p>' })
    editor.vm.$emit('update:modelValue', '<p>First second</p>')

    expect(wrapper.emitted('update:modelValue')).toEqual([
      ['<p>First</p>'],
      ['<p>First second</p>'],
    ])
  })
})
