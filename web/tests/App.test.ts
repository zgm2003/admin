import { nextTick } from 'vue'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { mount } from '@vue/test-utils'
import ElementPlus, { ElConfigProvider } from 'element-plus'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import App from '@/App.vue'
import { appI18n, elementPlusLocaleFor, setLocale } from '@/i18n'
import { pinia } from '@/store'

describe('application locale provider', () => {
  beforeEach(() => {
    setLocale('zh-CN')
    setNavigatorOnline(true)
  })

  afterEach(() => setNavigatorOnline(true))

  it('localizes programmatic confirmation buttons and follows language changes', async () => {
    const wrapper = mount(App, { global: { plugins: [pinia, appI18n], stubs: { RouterView: true } } })
    for (const [locale, confirm, cancel] of [['zh-CN', '确定', '取消'], ['en-US', 'OK', 'Cancel']] as const) {
      setLocale(locale)
      await nextTick()
      const pending = ElMessageBox.confirm('Message', 'Title').catch(() => undefined)
      await nextTick()
      expect(document.querySelector('.el-message-box__btns')?.textContent).toContain(confirm)
      expect(document.querySelector('.el-message-box__btns')?.textContent).toContain(cancel)
      document.querySelector<HTMLButtonElement>('.el-message-box__btns .el-button:not(.el-button--primary)')?.click()
      await pending
      ElMessageBox.close()
    }
    wrapper.unmount()
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
    wrapper.unmount()
  })

  it('mounts the network status notice globally', () => {
    setNavigatorOnline(false)
    const wrapper = mount(App, {
      global: {
        plugins: [pinia, ElementPlus, appI18n],
        stubs: { RouterView: true, transition: false },
      },
    })

    expect(wrapper.get('[data-testid="network-status-notice"]').text()).toContain('网络连接已断开')
    wrapper.unmount()
  })
})

function setNavigatorOnline(value: boolean): void {
  Object.defineProperty(window.navigator, 'onLine', {
    configurable: true,
    value,
  })
}
