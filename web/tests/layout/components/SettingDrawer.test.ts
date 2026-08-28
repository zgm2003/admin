import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { appI18n, setLocale } from '@src/i18n'
import { useUIPreferencesStore } from '@src/store/ui-preferences'
import { defaultUIPreferences, uiPreferencesStorageKey } from '@src/utils/ui-preferences'
import SettingDrawer from '@src/layout/components/SettingDrawer.vue'

describe('SettingDrawer', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    setLocale('zh-CN')
    document.body.innerHTML = ''
  })

  it('updates theme, primary color, and real layout switches', async () => {
    const store = useUIPreferencesStore()
    const wrapper = mountDrawer()
    await flushPromises()

    getControl('theme-dark').click()
    expect(store.preferences.theme).toBe('dark')

    wrapper.findComponent({ name: 'ElColorPicker' }).vm.$emit('change', '#059669')
    expect(store.preferences.primaryColor).toBe('#059669')

    getControl('show-footer').click()
    expect(store.preferences.showFooter).toBe(false)
    expect(wrapper.findComponent({ name: 'ElSpace' }).exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'ElRow' }).exists()).toBe(true)
  })

  it('shows persistent storage errors and resets only UI preferences', async () => {
    const store = useUIPreferencesStore()
    localStorage.setItem(uiPreferencesStorageKey, '{broken')
    store.initializeSafely()
    mountDrawer()
    await flushPromises()

    expect(getControl('ui-preferences-error').textContent).toContain('本地 UI 配置无效')
    getControl('reset-ui-preferences').click()
    expect(store.preferences).toEqual(defaultUIPreferences)
    expect(store.persistenceError).toBeNull()
  })

  it('keeps RouteTabs visible as the fullscreen exit path', async () => {
    mountDrawer({ contentFullscreen: true })
    await flushPromises()
    expect(getControl('show-route-tabs').classList.contains('is-disabled')).toBe(true)
  })
})

function mountDrawer(props: { modelValue?: boolean; contentFullscreen?: boolean } = {}) {
  return mount(SettingDrawer, {
    props: {
      modelValue: true,
      contentFullscreen: false,
      ...props,
    },
    attachTo: document.body,
    global: {
      plugins: [ElementPlus, appI18n],
    },
  })
}

function getControl(testId: string): HTMLElement {
  const control = document.body.querySelector<HTMLElement>(`[data-testid="${testId}"]`)
  if (control === null) throw new Error(`Missing control ${testId}`)
  return control
}
