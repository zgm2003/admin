import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

import * as dictionaryAPI from '@/api/system/dictionary'
import type { Dictionary, DictionaryItem, DictionaryPage } from '@/api/system/dictionary'
import { YesNo } from '@/enums/yesNo'
import { appI18n, setLocale } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import DictionaryPageView from '@/views/system/dictionary/index.vue'

vi.mock('@/api/system/dictionary', () => ({
  createDictionary: vi.fn(),
  createDictionaryItem: vi.fn(),
  deleteDictionary: vi.fn(),
  deleteDictionaryItem: vi.fn(),
  getDictionaries: vi.fn(),
  getDictionary: vi.fn(),
  updateDictionary: vi.fn(),
  updateDictionaryItem: vi.fn(),
  updateDictionaryItemStatus: vi.fn(),
  updateDictionaryStatus: vi.fn(),
}))

const builtinDictionary = dictionaryRow({ id: 1, code: 'user.gender', isBuiltin: YesNo.Yes })
const customDictionary = dictionaryRow({ id: 2, code: 'user.level', isBuiltin: YesNo.No })
const builtinItem = dictionaryItem({ id: 11, dictionaryId: 1, value: '0', isBuiltin: YesNo.Yes })
const customItem = dictionaryItem({ id: 12, dictionaryId: 1, value: '3', isBuiltin: YesNo.No })

describe('system dictionary page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('zh-CN')
    vi.mocked(dictionaryAPI.getDictionaries).mockResolvedValue({
      list: [builtinDictionary, customDictionary],
      total: 2,
      page: 1,
      pageSize: 20,
    })
    vi.mocked(dictionaryAPI.getDictionary).mockResolvedValue({
      dictionary: builtinDictionary,
      items: [builtinItem, customItem],
    })
    vi.mocked(dictionaryAPI.createDictionary).mockResolvedValue(3)
    vi.mocked(dictionaryAPI.createDictionaryItem).mockResolvedValue(13)
    vi.mocked(dictionaryAPI.updateDictionary).mockResolvedValue(undefined)
    vi.mocked(dictionaryAPI.updateDictionaryItem).mockResolvedValue(undefined)
    vi.mocked(dictionaryAPI.updateDictionaryStatus).mockResolvedValue({
      id: 1,
      isEnabled: YesNo.No,
    })
    vi.mocked(dictionaryAPI.updateDictionaryItemStatus).mockResolvedValue({
      id: 11,
      isEnabled: YesNo.No,
    })
    vi.mocked(dictionaryAPI.deleteDictionary).mockResolvedValue(undefined)
    vi.mocked(dictionaryAPI.deleteDictionaryItem).mockResolvedValue(undefined)
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('loads the list and exposes loading, empty, and error states', async () => {
    const response = deferred<DictionaryPage>()
    vi.mocked(dictionaryAPI.getDictionaries).mockReturnValueOnce(response.promise)
    const wrapper = mountPage(['system:dictionary:list'])

    await nextTick()
    expect(wrapper.getComponent({ name: 'AppTable' }).props('resultState')).toBe('loading')

    response.resolve({ list: [], total: 0, page: 1, pageSize: 20 })
    await flushPromises()
    expect(wrapper.getComponent({ name: 'AppTable' }).props('resultState')).toBe('empty')

    wrapper.unmount()
    vi.mocked(dictionaryAPI.getDictionaries).mockRejectedValueOnce(new Error('unavailable'))
    const failed = mountPage(['system:dictionary:list'])
    await flushPromises()
    expect(failed.getComponent({ name: 'AppTable' }).props('resultState')).toBe('error')
    expect(failed.text()).toContain('字典加载失败')
  })

  it('submits trimmed filters, resets them, and preserves filters while paging', async () => {
    const wrapper = mountPage(['system:dictionary:list'])
    await flushPromises()

    await wrapper.get('[data-testid="dictionary-keyword"]').setValue(' user.gender ')
    wrapper
      .getComponent('[data-testid="dictionary-status"]')
      .vm.$emit('update:modelValue', YesNo.Yes)
    await wrapper.get('[data-testid="dictionary-search"]').trigger('click')
    await flushPromises()
    expect(dictionaryAPI.getDictionaries).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 20,
      keyword: 'user.gender',
      isEnabled: YesNo.Yes,
    })

    wrapper.getComponent({ name: 'ElPagination' }).vm.$emit('current-change', 2)
    await flushPromises()
    expect(dictionaryAPI.getDictionaries).toHaveBeenLastCalledWith({
      page: 2,
      pageSize: 20,
      keyword: 'user.gender',
      isEnabled: YesNo.Yes,
    })

    await wrapper.get('[data-testid="dictionary-reset"]').trigger('click')
    await flushPromises()
    expect(dictionaryAPI.getDictionaries).toHaveBeenLastCalledWith({ page: 1, pageSize: 20 })
  })

  it('gates every mutation independently and protects builtin deletion', async () => {
    const wrapper = mountPage([
      'system:dictionary:list',
      'system:dictionary:create',
      'system:dictionary:update',
      'system:dictionary:status',
      'system:dictionary:delete',
    ])
    await flushPromises()

    expect(wrapper.find('[data-testid="dictionary-create"]').exists()).toBe(true)
    expect(wrapper.findAll('[data-testid="dictionary-update"]')).toHaveLength(2)
    expect(wrapper.findAll('[data-testid="dictionary-status"]')).toHaveLength(2)
    expect(wrapper.findAll('[data-testid="dictionary-delete"]')).toHaveLength(1)

    wrapper.unmount()
    const readonly = mountPage(['system:dictionary:list'])
    await flushPromises()
    expect(readonly.find('[data-testid="dictionary-create"]').exists()).toBe(false)
    expect(readonly.find('[data-testid="dictionary-update"]').exists()).toBe(false)
    expect(readonly.find('[data-testid="dictionary-status"]').exists()).toBe(false)
    expect(readonly.find('[data-testid="dictionary-delete"]').exists()).toBe(false)
  })

  it('creates and edits a dictionary while keeping its code immutable', async () => {
    const wrapper = mountPage([
      'system:dictionary:list',
      'system:dictionary:create',
      'system:dictionary:update',
    ])
    await flushPromises()

    await wrapper.get('[data-testid="dictionary-create"]').trigger('click')
    await wrapper.get('[data-testid="dictionary-form-code"]').setValue(' user.status ')
    await wrapper.get('[data-testid="dictionary-form-name-zh"]').setValue(' 用户状态 ')
    await wrapper.get('[data-testid="dictionary-form-name-en"]').setValue(' User status ')
    await wrapper.get('[data-testid="dictionary-form-description"]').setValue(' profile status ')
    await wrapper.get('[data-testid="dictionary-save"]').trigger('click')
    await flushPromises()
    expect(dictionaryAPI.createDictionary).toHaveBeenCalledWith({
      code: ' user.status ',
      nameZh: ' 用户状态 ',
      nameEn: ' User status ',
      description: ' profile status ',
    })

    await wrapper.findAll('[data-testid="dictionary-update"]')[0]!.trigger('click')
    expect(wrapper.get('[data-testid="dictionary-form-code"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="dictionary-form-name-zh"]').setValue('性别名称')
    await wrapper.get('[data-testid="dictionary-save"]').trigger('click')
    await flushPromises()
    expect(dictionaryAPI.updateDictionary).toHaveBeenCalledWith(1, {
      code: 'user.gender',
      nameZh: '性别名称',
      nameEn: 'Gender',
      description: '',
    })
  })

  it('opens details and creates and edits items with immutable values', async () => {
    const wrapper = mountPage([
      'system:dictionary:list',
      'system:dictionary:detail',
      'system:dictionary:create',
      'system:dictionary:update',
      'system:dictionary:status',
      'system:dictionary:delete',
    ])
    await flushPromises()
    wrapper.getComponent({ name: 'AppTable' }).vm.$emit('row-click', builtinDictionary)
    await flushPromises()

    expect(dictionaryAPI.getDictionary).toHaveBeenCalledWith(1)
    expect(wrapper.findAll('[data-testid="dictionary-item-delete"]')).toHaveLength(1)
    expect(wrapper.findAll('[data-testid="dictionary-item-status"]')).toHaveLength(2)

    await wrapper.get('[data-testid="dictionary-item-create"]').trigger('click')
    await wrapper.get('[data-testid="dictionary-item-form-value"]').setValue(' 4 ')
    await wrapper.get('[data-testid="dictionary-item-form-label-zh"]').setValue('其他')
    await wrapper.get('[data-testid="dictionary-item-form-label-en"]').setValue('Other')
    wrapper
      .getComponent('[data-testid="dictionary-item-form-sort"]')
      .vm.$emit('update:modelValue', 4)
    await wrapper.get('[data-testid="dictionary-item-save"]').trigger('click')
    await flushPromises()
    expect(dictionaryAPI.createDictionaryItem).toHaveBeenCalledWith(1, {
      value: ' 4 ',
      labelZh: '其他',
      labelEn: 'Other',
      sort: 4,
    })

    await wrapper.findAll('[data-testid="dictionary-item-update"]')[1]!.trigger('click')
    expect(
      wrapper.get('[data-testid="dictionary-item-form-value"]').attributes('disabled'),
    ).toBeDefined()
    await wrapper.get('[data-testid="dictionary-item-form-label-en"]').setValue('Custom')
    await wrapper.get('[data-testid="dictionary-item-save"]').trigger('click')
    await flushPromises()
    expect(dictionaryAPI.updateDictionaryItem).toHaveBeenCalledWith(1, 12, {
      value: '3',
      labelZh: '自定义',
      labelEn: 'Custom',
      sort: 3,
    })
  })
})

function mountPage(permissionCodes: string[]): VueWrapper {
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes })
  return mount(DictionaryPageView, {
    attachTo: document.body,
    global: { plugins: [pinia, appI18n, ElementPlus] },
  })
}

function dictionaryRow(overrides: Partial<Dictionary>): Dictionary {
  return {
    id: 1,
    code: 'user.gender',
    nameZh: '性别',
    nameEn: 'Gender',
    description: '',
    isEnabled: YesNo.Yes,
    isBuiltin: YesNo.No,
    itemCount: 3,
    createdAt: '2026-09-10T00:00:00Z',
    updatedAt: '2026-09-10T00:00:00Z',
    ...overrides,
  }
}

function dictionaryItem(overrides: Partial<DictionaryItem>): DictionaryItem {
  return {
    id: 11,
    dictionaryId: 1,
    value: '0',
    labelZh: '未知',
    labelEn: 'Unknown',
    sort: 0,
    isEnabled: YesNo.Yes,
    isBuiltin: YesNo.No,
    createdAt: '2026-09-10T00:00:00Z',
    updatedAt: '2026-09-10T00:00:00Z',
    ...overrides,
    labelZh: overrides.value === '3' ? '自定义' : (overrides.labelZh ?? '未知'),
    labelEn: overrides.value === '3' ? 'Custom value' : (overrides.labelEn ?? 'Unknown'),
    sort: overrides.value === '3' ? 3 : (overrides.sort ?? 0),
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
