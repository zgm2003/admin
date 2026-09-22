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
import UpMedia from '@/components/UpMedia/index.vue'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'

vi.mock('element-plus/es/components/message-box/index', () => ({
  ElMessageBox: { confirm: vi.fn() },
}))

vi.mock('@/api/system/setting', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/system/setting')>()
  return {
    ...actual,
    createSetting: vi.fn(),
    deleteSetting: vi.fn(),
    getSettings: vi.fn(),
    getBrandSettings: vi.fn(),
    updateSetting: vi.fn(),
    updateBrandSettings: vi.fn(),
    updateSettingStatus: vi.fn(),
  }
})

const builtinSetting = settingRow({ id: 1, key: 'auth.captcha.ttl_minutes', isBuiltin: YesNo.Yes })
const customSetting = settingRow({ id: 2, key: 'auth.captcha.slide_padding', isBuiltin: YesNo.No })
const mountedWrappers: VueWrapper[] = []

describe('system setting page', () => {
  vi.setConfig({ testTimeout: 30_000 })

  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('zh-CN')
    vi.mocked(settingAPI.getSettings).mockResolvedValue({
      list: [builtinSetting, customSetting],
      total: 2,
      page: 1,
      pageSize: 20,
    })
    vi.mocked(settingAPI.getBrandSettings).mockResolvedValue({
      titleZhCN: '智澜',
      titleEnUS: 'ZHILAN',
      defaultAvatar: '',
    })
    vi.mocked(settingAPI.createSetting).mockResolvedValue(3)
    vi.mocked(settingAPI.updateSetting).mockResolvedValue(undefined)
    vi.mocked(settingAPI.updateBrandSettings).mockResolvedValue(undefined)
    vi.mocked(settingAPI.updateSettingStatus).mockResolvedValue(undefined)
    vi.mocked(settingAPI.deleteSetting).mockResolvedValue(undefined)
    vi.mocked(ElMessageBox.confirm).mockResolvedValue(
      'confirm' as unknown as Awaited<ReturnType<typeof ElMessageBox.confirm>>,
    )
  })

  afterEach(() => {
    for (const wrapper of mountedWrappers.splice(0)) wrapper.unmount()
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
  }, 30_000)

  it('follows the standard management page shell and shared component contracts', async () => {
    const wrapper = mountPage(['system:setting:list', 'system:setting:create'])
    await flushPromises()

    expect(wrapper.getComponent({ name: 'AppPage' }).classes()).toContain('management-page')
    expect(wrapper.findComponent({ name: 'SettingDialog' }).exists()).toBe(true)
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

  it('shows brand and advanced settings in separate tabs without losing unsaved brand input', async () => {
    const wrapper = mountPage(['system:setting:list', 'system:setting:update'])
    await flushPromises()

    expect(wrapper.get('.el-tabs__item.is-active').text()).toBe('品牌设置')
    expect(wrapper.getComponent({ name: 'BrandSettingsPanel' }).isVisible()).toBe(true)
    expect(wrapper.getComponent({ name: 'AppSearch' }).isVisible()).toBe(false)

    await wrapper.get('[data-testid="brand-title-zh-cn"]').setValue('新的品牌名')
    await wrapper.get('#tab-advanced').trigger('click')
    expect(wrapper.get('.el-tabs__item.is-active').text()).toBe('高级设置')
    expect(wrapper.getComponent({ name: 'AppSearch' }).isVisible()).toBe(true)
    expect(wrapper.getComponent({ name: 'BrandSettingsPanel' }).isVisible()).toBe(false)

    await wrapper.get('#tab-brand').trigger('click')
    expect(wrapper.get('[data-testid="brand-title-zh-cn"]').element).toHaveProperty(
      'value',
      '新的品牌名',
    )
    expect(settingAPI.getSettings).toHaveBeenCalledTimes(1)
    expect(settingAPI.getBrandSettings).toHaveBeenCalledTimes(1)
  })

  it('edits brand titles and reuses the single-image avatar upload rule', async () => {
    const wrapper = mountPage([
      'system:setting:list',
      'system:setting:update',
      'storage:object:upload',
    ])
    await flushPromises()

    const media = wrapper.getComponent(UpMedia)
    expect(media.props()).toMatchObject({
      modelValue: '',
      ruleCode: 'avatar',
      multiple: false,
      accept: 'image/*',
      variant: 'avatar',
      disabled: false,
    })
    await wrapper.get('[data-testid="brand-title-zh-cn"]').setValue(' 新标题 ')
    await wrapper.get('[data-testid="brand-title-en-us"]').setValue(' New title ')
    media.vm.$emit('update:modelValue', 'avatar/2026/09/15/default.png')
    await wrapper.get('[data-testid="brand-save"]').trigger('click')
    await flushPromises()

    expect(settingAPI.updateBrandSettings).toHaveBeenCalledWith({
      titleZhCN: '新标题',
      titleEnUS: 'New title',
      defaultAvatar: 'avatar/2026/09/15/default.png',
    })
  })

  it('keeps brand values visible but disables mutations without their permissions', async () => {
    const wrapper = mountPage(['system:setting:list'])
    await flushPromises()

    expect(wrapper.get('[data-testid="brand-title-zh-cn"]').attributes('disabled')).toBeDefined()
    expect(wrapper.getComponent(UpMedia).props('disabled')).toBe(true)
    expect(wrapper.find('[data-testid="brand-save"]').exists()).toBe(false)
  })

  it('formats valid JSON and blocks malformed JSON in the typed setting editor', async () => {
    const wrapper = mountPage(['system:setting:list', 'system:setting:create'])
    await flushPromises()
    await wrapper.get('[data-testid="setting-create"]').trigger('click')
    const typeSelect = wrapper
      .getComponent({ name: 'SettingDialog' })
      .getComponent({ name: 'ElSelectV2' })
    typeSelect.vm.$emit('update:modelValue', 4)
    await setBodyValue('setting-form-key', 'app.brand.payload')
    await setBodyValue('setting-form-value', '{"name":"智澜"}')
    await clickBody('setting-json-format')
    const valueRoot = document.querySelector('[data-testid="setting-form-value"]')
    const textarea =
      valueRoot instanceof HTMLTextAreaElement ? valueRoot : valueRoot?.querySelector('textarea')
    expect(textarea?.value).toContain('\n  "name": "智澜"\n')

    await setBodyValue('setting-form-value', '{')
    await clickBody('setting-save')
    expect(settingAPI.createSetting).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('JSON 格式不正确')
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
    const settingDialog = wrapper.getComponent({ name: 'SettingDialog' })
    settingDialog.vm.$emit('update:form', { ...settingDialog.props('form'), value: '3' })
    await nextTick()
    await clickBody('setting-save')
    await flushPromises()
    expect(settingAPI.updateSetting).toHaveBeenCalledWith('auth.captcha.ttl_minutes', {
      value: '3',
      valueType: 2,
      description: '',
    })
  })

  it('keeps required retention settings numeric and removes their disable action', async () => {
    const retention = settingRow({
      id: 10,
      key: settingAPI.messageNotificationRetentionDaysKey,
      value: '180',
      valueType: 2,
      isBuiltin: YesNo.Yes,
    })
    vi.mocked(settingAPI.getSettings).mockResolvedValue({
      list: [retention],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    const wrapper = mountPage([
      'system:setting:list',
      'system:setting:update',
      'system:setting:status',
    ])
    await flushPromises()
    expect(wrapper.find('[data-testid="setting-status-toggle"]').exists()).toBe(false)
    await wrapper.get('[data-testid="setting-update"]').trigger('click')
    const dialog = wrapper.getComponent({ name: 'SettingDialog' })
    expect(dialog.props('form')).toMatchObject({ key: retention.key, valueType: 2 })
    const valueRoot = document.querySelector('[data-testid="setting-form-value"]')
    const input =
      valueRoot instanceof HTMLInputElement ? valueRoot : valueRoot?.querySelector('input')
    expect(input?.getAttribute('min')).toBe('30')
    expect(input?.getAttribute('max')).toBe('3650')
  })

  it('confirms a retention decrease before updating', async () => {
    const retention = settingRow({
      id: 11,
      key: settingAPI.realtimeEventRetentionDaysKey,
      value: '7',
      valueType: 2,
      isBuiltin: YesNo.Yes,
    })
    vi.mocked(settingAPI.getSettings).mockResolvedValue({
      list: [retention],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    const wrapper = mountPage(['system:setting:list', 'system:setting:update'])
    await flushPromises()
    await wrapper.get('[data-testid="setting-update"]').trigger('click')
    const dialog = wrapper.getComponent({ name: 'SettingDialog' })
    dialog.vm.$emit('update:form', { ...dialog.props('form'), value: '6' })
    await nextTick()
    await clickBody('setting-save')
    await flushPromises()
    expect(ElMessageBox.confirm).toHaveBeenCalledOnce()
    expect(settingAPI.updateSetting).toHaveBeenCalledWith(
      retention.key,
      expect.objectContaining({ value: '6' }),
    )
  })

  it('does not update when a retention decrease is cancelled', async () => {
    const retention = settingRow({
      id: 12,
      key: settingAPI.realtimeEventRetentionDaysKey,
      value: '7',
      valueType: 2,
      isBuiltin: YesNo.Yes,
    })
    vi.mocked(settingAPI.getSettings).mockResolvedValue({
      list: [retention],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    vi.mocked(ElMessageBox.confirm).mockRejectedValue('cancel')
    const wrapper = mountPage(['system:setting:list', 'system:setting:update'])
    await flushPromises()
    await wrapper.get('[data-testid="setting-update"]').trigger('click')
    const dialog = wrapper.getComponent({ name: 'SettingDialog' })
    dialog.vm.$emit('update:form', { ...dialog.props('form'), value: '6' })
    await nextTick()
    await clickBody('setting-save')
    await flushPromises()
    expect(ElMessageBox.confirm).toHaveBeenCalledOnce()
    expect(settingAPI.updateSetting).not.toHaveBeenCalled()
  })

  it.each([
    ['equal', '7'],
    ['increase', '8'],
  ])('updates an %s retention value without confirmation', async (_case, value) => {
    const retention = settingRow({
      id: 13,
      key: settingAPI.realtimeEventRetentionDaysKey,
      value: '7',
      valueType: 2,
      isBuiltin: YesNo.Yes,
    })
    vi.mocked(settingAPI.getSettings).mockResolvedValue({
      list: [retention],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    const wrapper = mountPage(['system:setting:list', 'system:setting:update'])
    await flushPromises()
    await wrapper.get('[data-testid="setting-update"]').trigger('click')
    const dialog = wrapper.getComponent({ name: 'SettingDialog' })
    dialog.vm.$emit('update:form', { ...dialog.props('form'), value })
    await nextTick()
    await clickBody('setting-save')
    await flushPromises()
    expect(ElMessageBox.confirm).not.toHaveBeenCalled()
    expect(settingAPI.updateSetting).toHaveBeenCalledWith(
      retention.key,
      expect.objectContaining({ value }),
    )
  })

  it('keeps the retention dialog open when the update request fails', async () => {
    const retention = settingRow({
      id: 14,
      key: settingAPI.realtimeEventRetentionDaysKey,
      value: '7',
      valueType: 2,
      isBuiltin: YesNo.Yes,
    })
    vi.mocked(settingAPI.getSettings).mockResolvedValue({
      list: [retention],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    vi.mocked(settingAPI.updateSetting).mockRejectedValue(new Error('request failed'))
    const wrapper = mountPage(['system:setting:list', 'system:setting:update'])
    const errorHandler = vi.fn()
    wrapper.vm.$.appContext.config.errorHandler = errorHandler
    await flushPromises()
    await wrapper.get('[data-testid="setting-update"]').trigger('click')
    const dialog = wrapper.getComponent({ name: 'SettingDialog' })
    dialog.vm.$emit('update:form', { ...dialog.props('form'), value: '8' })
    await nextTick()
    await clickBody('setting-save')
    await flushPromises()
    expect(errorHandler).toHaveBeenCalledOnce()
    expect(document.querySelector('[data-testid="setting-save"]')).not.toBeNull()
  })
})

function mountPage(permissionCodes: string[]): VueWrapper {
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes })
  const wrapper = mount(SettingPageView, {
    attachTo: document.body,
    global: { plugins: [pinia, appI18n, ElementPlus] },
  })
  mountedWrappers.push(wrapper)
  return wrapper
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
