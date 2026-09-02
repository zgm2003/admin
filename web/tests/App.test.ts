import { nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import ElementPlus, { ElConfigProvider } from 'element-plus'
import { beforeEach, describe, expect, it } from 'vitest'

import App from '@/App.vue'
import { appI18n, elementPlusLocaleFor, setLocale } from '@/i18n'
import { pinia } from '@/store'

describe('application locale provider', () => {
  beforeEach(() => {
    setLocale('zh-CN')
  })

  it('updates Element Plus locale and pagination text without remounting', async () => {
    const wrapper = mount(App, {
      global: {
        plugins: [pinia, ElementPlus, appI18n],
        stubs: {
          RouterView: {
            template: '<el-pagination :total="123" />',
          },
        },
      },
    })

    const provider = wrapper.findComponent(ElConfigProvider)
    expect(provider.exists()).toBe(true)
    expect(provider.props('locale')).toBe(elementPlusLocaleFor('zh-CN'))

    setLocale('en-US')
    await nextTick()

    expect(provider.props('locale')).toBe(elementPlusLocaleFor('en-US'))
    expect(wrapper.text()).toContain('Total 123')
  })
})
