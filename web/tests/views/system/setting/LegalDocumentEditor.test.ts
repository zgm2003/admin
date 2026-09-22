import { shallowMount } from '@vue/test-utils'
import { Editor, Toolbar } from '@wangeditor-next/editor-for-vue'
import { describe, expect, it } from 'vitest'

import LegalDocumentEditor, {
  legalDocumentEditorConfig,
  legalDocumentToolbarKeys,
} from '@/views/system/setting/components/LegalDocumentEditor/index.vue'

describe('LegalDocumentEditor', () => {
  it('uses the restricted legal-document toolbar and HTTPS-only links', () => {
    const wrapper = shallowMount(LegalDocumentEditor, { props: { modelValue: '' } })

    expect(wrapper.getComponent(Toolbar).props('defaultConfig')).toEqual({
      toolbarKeys: [...legalDocumentToolbarKeys],
    })
    expect(legalDocumentEditorConfig.MENU_CONF.insertLink.checkLink('https://example.test')).toBe(
      true,
    )
    expect(legalDocumentEditorConfig.MENU_CONF.insertLink.checkLink('http://example.test')).toBe(
      false,
    )
  })

  it('forwards editor changes through v-model', async () => {
    const wrapper = shallowMount(LegalDocumentEditor, { props: { modelValue: '' } })
    wrapper.getComponent(Editor).vm.$emit('update:modelValue', '<p>Policy</p>')
    expect(wrapper.emitted('update:modelValue')).toEqual([['<p>Policy</p>']])
  })
})
