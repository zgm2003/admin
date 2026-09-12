import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

import * as settingAPI from '@/api/system/setting'
import type { SettingPage, SystemSetting } from '@/api/system/setting'
import { YesNo } from '@/enums/yesNo'
import { appI18n, setLocale } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import SettingPageView from '@/views/system/setting/index.vue'

vi.mock('@/api/system/setting', () => ({
  createSetting: vi.fn(),
  deleteSetting: vi.fn(),
  getSettings: vi.fn(),
  updateSetting: vi.fn(),
  updateSettingStatus: vi.fn(),
}))

const builtinSetting = settingRow({ id: 1, key: 'auth.captcha.ttl_minutes', isBuiltin: YesNo.Yes })
const customSetting = settingRow({ id: 2, key: 'auth.captcha.slide_padding', isBuiltin: YesNo.No })

describe('system setting page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('zh-CN')
    vi.mocked(settingAPI.getSettings).mockResolvedValue({
      list: [builtinSetting, customSetting],
      total: 2,
      page: 1,
      pageSize: 20,
    })
    vi.mocked(settingAPI.createSetting).mockResolvedValue(3)
    vi.mocked(settingAPI.updateSetting).mockResolvedValue(undefined)
    vi.mocked(settingAPI.updateSettingStatus).mockResolvedValue(undefined)
    vi.mocked(settingAPI.deleteSetting).mockResolvedValue(undefined)
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('exposes loading, empty, and error states while loading settings', async () => {
    const response = deferred<SettingPage>()
    vi.mocked(settingAPI.getSettings).mockReturnValueOnce(response.promise)
    const wrapper = mountPage(['system:setting:list'])
    await nextTick()
    expect(wrapper.getComponent({ name: 'AppTable' }).props('resultState')).toBe('loading')

    response.resolve({ list: [], total: 0, page: 1, pageSize: 20 })
    await flushPromises()
    expect(wrapper.getComponent({ name: 'AppTable' }).props('resultState')).toBe('empty')

    wrapper.unmount()
    vi.mocked(settingAPI.getSettings).mockRejectedValueOnce(new Error('unavailable'))
    const failed = mountPage(['system:setting:list'])
    await flushPromises()
    expect(failed.getComponent({ name: 'AppTable' }).props('resultState')).toBe('error')
    expect(failed.text()).toContain('系统设置加载失败')
  })

  it('follows the standard management page shell and shared component contracts', async () => {
    const wrapper = mountPage(['system:setting:list', 'system:setting:create'])
    await flushPromises()

    expect(wrapper.get('section.setting-page').classes()).toContain('management-page')
    expect(wrapper.find('h1').exists()).toBe(false)

    const search = wrapper.getComponent({ name: 'AppSearch' })
    expect(search.props('queryLabel')).toBe('查询')
    expect(search.props('resetLabel')).toBe('重置')
    expect(search.props('queryTestId')).toBe('setting-search')
    expect(search.props('resetTestId')).toBe('setting-reset')

    const table = wrapper.getComponent({ name: 'AppTable' })
    expect(table.props('rowKey')).toBe('id')
    expect(table.props('ariaLabel')).toBe('系统设置')
    expect(wrapper.find('.app-table__toolbar-left [data-testid="setting-create"]').exists()).toBe(
      true,
    )
    expect(wrapper.find('[data-testid="setting-empty"]').exists()).toBe(false)
  })

  it('submits normalized filters and keeps them while paging', async () => {
    const wrapper = mountPage(['system:setting:list'])
    await flushPromises()

    await wrapper.get('[data-testid="setting-keyword"]').setValue(' auth.captcha ')
    const statusFilter = wrapper
      .findAllComponents({ name: 'ElSelectV2' })
      .find((component) => component.attributes('data-testid') === 'setting-status-filter')
    if (statusFilter === undefined) throw new Error('setting status filter not found')
    statusFilter.vm.$emit('update:modelValue', YesNo.Yes)
    await wrapper.get('[data-testid="setting-search"]').trigger('click')
    await flushPromises()
    expect(settingAPI.getSettings).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 20,
      keyword: 'auth.captcha',
      isEnabled: YesNo.Yes,
    })

    wrapper.getComponent({ name: 'ElPagination' }).vm.$emit('current-change', 2)
    await flushPromises()
    expect(settingAPI.getSettings).toHaveBeenLastCalledWith({
      page: 2,
      pageSize: 20,
      keyword: 'auth.captcha',
      isEnabled: YesNo.Yes,
    })
  })

  it('gates every mutation and protects builtin deletion', async () => {
    const wrapper = mountPage([
      'system:setting:list',
      'system:setting:create',
      'system:setting:update',
      'system:setting:status',
      'system:setting:delete',
    ])
    await flushPromises()

    expect(wrapper.find('[data-testid="setting-create"]').exists()).toBe(true)
    expect(wrapper.findAll('[data-testid="setting-update"]')).toHaveLength(2)
    expect(wrapper.findAll('[data-testid="setting-status-toggle"]')).toHaveLength(2)
    expect(wrapper.findAll('[data-testid="setting-delete"]')).toHaveLength(1)

    wrapper.unmount()
    const readonly = mountPage(['system:setting:list'])
    await flushPromises()
    expect(readonly.find('[data-testid="setting-create"]').exists()).toBe(false)
    expect(readonly.find('[data-testid="setting-update"]').exists()).toBe(false)
    expect(readonly.find('[data-testid="setting-status-toggle"]').exists()).toBe(false)
    expect(readonly.find('[data-testid="setting-delete"]').exists()).toBe(false)
  })

  it('creates a setting and keeps its key immutable during edit', async () => {
    const wrapper = mountPage([
      'system:setting:list',
      'system:setting:create',
      'system:setting:update',
    ])
    await flushPromises()

    await wrapper.get('[data-testid="setting-create"]').trigger('click')
    expect(document.body.textContent).toContain('取消')
    await setBodyValue('setting-form-key', 'auth.session.ttl')
    await setBodyValue('setting-form-value', '30')
    await setBodyValue('setting-form-description', ' Session lifetime ')
    await clickBody('setting-save')
    await flushPromises()
    expect(settingAPI.createSetting).toHaveBeenCalledWith({
      key: 'auth.session.ttl',
      value: '30',
      valueType: 1,
      description: ' Session lifetime ',
    })

    await wrapper.findAll('[data-testid="setting-update"]')[0]!.trigger('click')
    const keyRoot = document.querySelector('[data-testid="setting-form-key"]')
    const keyElement =
      keyRoot instanceof HTMLInputElement ? keyRoot : keyRoot?.querySelector('input')
    if (!(keyElement instanceof HTMLInputElement) || !keyElement.disabled) {
      throw new Error('setting key must be disabled during edit')
    }
    await setBodyValue('setting-form-value', '3')
    await clickBody('setting-save')
    await flushPromises()
    expect(settingAPI.updateSetting).toHaveBeenCalledWith('auth.captcha.ttl_minutes', {
      value: '3',
      valueType: 2,
      description: '',
    })
  })
})

function mountPage(permissionCodes: string[]): VueWrapper {
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes })
  return mount(SettingPageView, {
    attachTo: document.body,
    global: { plugins: [pinia, appI18n, ElementPlus] },
  })
}

function settingRow(overrides: Partial<SystemSetting>): SystemSetting {
  return {
    id: 1,
    key: 'auth.captcha.ttl_minutes',
    value: '2',
    valueType: 2,
    description: '',
    isEnabled: YesNo.Yes,
    isBuiltin: YesNo.No,
    createdAt: '2026-09-12T00:00:00Z',
    updatedAt: '2026-09-12T00:00:00Z',
    ...overrides,
  }
}

function deferred<T>(): { promise: Promise<T>; resolve: (value: T) => void } {
  let resolvePromise: ((value: T) => void) | undefined
  const promise = new Promise<T>((resolve) => {
    resolvePromise = resolve
  })
  return {
    promise,
    resolve: (value: T) => {
      if (resolvePromise === undefined) throw new Error('deferred promise was not initialized')
      resolvePromise(value)
    },
  }
}

async function setBodyValue(testId: string, value: string): Promise<void> {
  const wrapper = document.querySelector(`[data-testid="${testId}"]`)
  const input =
    wrapper instanceof HTMLInputElement || wrapper instanceof HTMLTextAreaElement
      ? wrapper
      : wrapper?.querySelector('input, textarea')
  if (!(input instanceof HTMLInputElement || input instanceof HTMLTextAreaElement)) {
    throw new Error(`input not found: ${testId}`)
  }
  input.value = value
  input.dispatchEvent(new Event('input', { bubbles: true }))
  await nextTick()
}

async function clickBody(testId: string): Promise<void> {
  const element = document.querySelector(`[data-testid="${testId}"]`)
  if (!(element instanceof HTMLElement)) throw new Error(`element not found: ${testId}`)
  element.click()
  await flushPromises()
}
