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
    getLegalDocument: vi.fn(),
    updateSetting: vi.fn(),
    updateBrandSettings: vi.fn(),
    updateLegalDocument: vi.fn(),
    updateSettingStatus: vi.fn(),
  }
})

const builtinSetting = settingRow({ id: 1, key: 'auth.captcha.ttl_minutes', isBuiltin: YesNo.Yes })
const customSetting = settingRow({ id: 2, key: 'auth.captcha.slide_padding', isBuiltin: YesNo.No })
const mediaAvatar = settingRow({
  id: 3,
  key: 'app.brand.default_avatar',
  value: '',
  valueType: 5,
  isBuiltin: YesNo.Yes,
})
const mediaXlsx = settingRow({
  id: 4,
  key: 'message.mail.recipient_rule.import_template_object_key',
  value: '',
  valueType: 5,
  isBuiltin: YesNo.Yes,
})
const uploadedImage =
  'setting/.admin-storage/v2/p1/r1/c1/v1/2026/09/30/0123456789abcdef0123456789abcdef.png'
const uploadedXlsx = uploadedImage.replace('.png', '.xlsx')
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
    vi.mocked(settingAPI.getLegalDocument).mockImplementation(async (kind) => ({
      kind,
      contentHtml: kind === 'userAgreement' ? '<p>Agreement</p>' : '<p>Privacy</p>',
    }))
    vi.mocked(settingAPI.createSetting).mockResolvedValue(3)
    vi.mocked(settingAPI.updateSetting).mockResolvedValue(undefined)
    vi.mocked(settingAPI.updateBrandSettings).mockResolvedValue(undefined)
    vi.mocked(settingAPI.updateLegalDocument).mockResolvedValue(undefined)
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

  it('keeps only advanced and legal tabs, with advanced selected first', async () => {
    const wrapper = mountPage(['system:setting:list', 'system:setting:update'])
    await flushPromises()
    expect(
      wrapper
        .findAll('.setting-page__tabs > .el-tabs__header [role="tab"]')
        .map((tab) => tab.text()),
    ).toEqual(['高级设置', '协议与隐私'])
    expect(wrapper.get('.el-tabs__item.is-active').text()).toBe('高级设置')
    expect(wrapper.findComponent({ name: 'MediaSettingsPanel' }).exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'BrandSettingsPanel' }).exists()).toBe(false)
  })

  it('loads both single-language legal documents and saves the selected document', async () => {
    const wrapper = mountPage(['system:setting:list', 'system:setting:update'])
    await flushPromises()
    await wrapper.get('#tab-legal').trigger('click')

    const panel = wrapper.getComponent({ name: 'LegalSettingsPanel' })
    expect(panel.props('documents')).toEqual({
      userAgreement: '<p>Agreement</p>',
      privacyPolicy: '<p>Privacy</p>',
    })
    panel.vm.$emit('update:documents', {
      userAgreement: '<p>Updated agreement</p>',
      privacyPolicy: '<p>Privacy</p>',
    })
    panel.vm.$emit('save', 'userAgreement')
    await flushPromises()

    expect(settingAPI.getLegalDocument).toHaveBeenCalledTimes(2)
    expect(settingAPI.getLegalDocument).toHaveBeenCalledWith('userAgreement')
    expect(settingAPI.getLegalDocument).toHaveBeenCalledWith('privacyPolicy')
    expect(settingAPI.updateLegalDocument).toHaveBeenCalledWith(
      'userAgreement',
      '<p>Updated agreement</p>',
    )
  })

  it('offers media value type in the common create dialog and stores only its object key', async () => {
    const wrapper = mountPage([
      'system:setting:list',
      'system:setting:create',
      'storage:object:upload',
    ])
    await flushPromises()
    await wrapper.get('[data-testid="setting-create"]').trigger('click')
    const dialog = wrapper.getComponent({ name: 'SettingDialog' })
    const select = dialog.getComponent({ name: 'ElSelectV2' })
    expect(select.props('options')).toContainEqual({ label: '媒体', value: 5 })
    await setBodyValue('setting-form-key', 'app.assets.custom')
    select.vm.$emit('update:modelValue', 5)
    await nextTick()
    const media = dialog.getComponent(UpMedia)
    expect(media.props('ruleCode')).toBe('setting')
    expect(document.querySelector('[data-testid="setting-form-value"]')).toBeNull()
    media.vm.$emit('uploading-change', true)
    media.vm.$emit('update:modelValue', uploadedXlsx)
    await nextTick()
    expect(document.querySelector('[data-testid="setting-save"]')).toHaveProperty('disabled', true)
    media.vm.$emit('uploading-change', false)
    await nextTick()
    await clickBody('setting-save')
    await flushPromises()
    expect(settingAPI.createSetting).toHaveBeenCalledWith({
      key: 'app.assets.custom',
      value: uploadedXlsx,
      valueType: 5,
      description: '',
    })
  })

  it('edits builtin media in the common dialog and allows clearing without upload permission', async () => {
    vi.mocked(settingAPI.getSettings).mockResolvedValue({
      list: [{ ...mediaAvatar, value: uploadedImage }],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    const wrapper = mountPage(['system:setting:list', 'system:setting:update'])
    await flushPromises()
    await wrapper.get('[data-testid="setting-update"]').trigger('click')
    const dialog = wrapper.getComponent({ name: 'SettingDialog' })
    expect(dialog.getComponent({ name: 'ElSelectV2' }).props('disabled')).toBe(true)
    const media = dialog.getComponent(UpMedia)
    expect(media.props()).toMatchObject({
      ruleCode: 'setting',
      variant: 'avatar',
      uploadDisabled: true,
      disabled: false,
    })
    media.vm.$emit('update:modelValue', '')
    await nextTick()
    await clickBody('setting-save')
    await flushPromises()
    expect(settingAPI.updateSetting).toHaveBeenCalledWith(mediaAvatar.key, {
      value: '',
      valueType: 5,
      description: mediaAvatar.description,
    })
    expect(settingAPI.deleteSetting).not.toHaveBeenCalled()
  })

  it('edits titles in advanced settings and refreshes runtime brand values', async () => {
    const title = settingRow({
      key: 'app.brand.title_zh_cn',
      value: '旧标题',
      valueType: 1,
      isBuiltin: YesNo.Yes,
    })
    vi.mocked(settingAPI.getSettings).mockResolvedValue({
      list: [title],
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
    expect(dialog.getComponent({ name: 'ElSelectV2' }).props('disabled')).toBe(true)
    await setBodyValue('setting-form-value', '新标题')
    await clickBody('setting-save')
    await flushPromises()
    expect(settingAPI.updateSetting).toHaveBeenCalledWith(
      title.key,
      expect.objectContaining({ value: '新标题', valueType: 1 }),
    )
    expect(settingAPI.getBrandSettings).toHaveBeenCalledOnce()
    expect(settingAPI.updateBrandSettings).not.toHaveBeenCalled()
  })

  it('does not render fake empty settings or request media without read access', async () => {
    const wrapper = mountPage(['system:setting:view'])
    await flushPromises()
    expect(wrapper.findComponent({ name: 'AppTable' }).exists()).toBe(false)
    expect(wrapper.text()).toContain('未授予系统设置读取权限')
    expect(wrapper.findComponent(UpMedia).exists()).toBe(false)
    expect(settingAPI.getSettings).not.toHaveBeenCalled()
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
    expect(errorHandler).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('保存失败')
    expect(document.querySelector('[data-testid="setting-save"]')).not.toBeNull()
  })

  it('keeps an uploaded media draft when saving fails', async () => {
    vi.mocked(settingAPI.getSettings).mockResolvedValue({
      list: [mediaXlsx],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    vi.mocked(settingAPI.updateSetting).mockRejectedValueOnce(new Error('save failed'))
    const wrapper = mountPage([
      'system:setting:list',
      'system:setting:update',
      'storage:object:upload',
    ])
    await flushPromises()
    await wrapper.get('[data-testid="setting-update"]').trigger('click')
    const dialog = wrapper.getComponent({ name: 'SettingDialog' })
    const media = dialog.getComponent(UpMedia)
    expect(media.props('accept')).toBe(
      '.xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    )
    media.vm.$emit('update:modelValue', uploadedXlsx)
    await nextTick()
    await clickBody('setting-save')
    await flushPromises()
    expect(media.props('modelValue')).toBe(uploadedXlsx)
    expect(document.body.textContent).toContain('保存失败')
    await clickBody('setting-save')
    await flushPromises()
    expect(settingAPI.updateSetting).toHaveBeenLastCalledWith(mediaXlsx.key, {
      value: uploadedXlsx,
      valueType: 5,
      description: mediaXlsx.description,
    })
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
