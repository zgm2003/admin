import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

import * as cacheGenerationAPI from '@/api/system/cacheGeneration'
import type { CacheGeneration, CacheGenerationPage } from '@/api/system/cacheGeneration'
import { appI18n, setLocale } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import CacheGenerationPageView from '@/views/system/cacheGeneration/index.vue'

vi.mock('@/api/system/cacheGeneration', () => ({
  getCacheGenerations: vi.fn(),
  getCacheGenerationOptions: vi.fn(),
}))

const readyRow: CacheGeneration = {
  namespaceLabel: '系统设置',
  scopeLabel: '全局',
  statusLabel: '就绪',
  statusTone: 'success',
  statusHint: '',
  publishedVersion: 12,
  namespace: 'system.setting',
  scopeKey: 'global',
  generation: 12,
  status: 'ready',
  pendingCount: 0,
  oldestPendingAt: null,
  latestAttempts: 1,
  lastError: '',
  latestPublishedGeneration: 12,
  latestPublishedAt: '2026-09-16T08:00:00Z',
  updatedAt: '2026-09-16T08:00:00Z',
}
const retryingRow: CacheGeneration = {
  namespaceLabel: '短信服务',
  scopeLabel: '全局',
  statusLabel: '重试中',
  statusTone: 'danger',
  statusHint: '',
  publishedVersion: null,
  namespace: 'message.sms',
  scopeKey: 'global',
  generation: 5,
  status: 'retrying',
  pendingCount: 2,
  oldestPendingAt: '2026-09-16T07:00:00Z',
  latestAttempts: 3,
  lastError: 'dependency-unavailable: redis unavailable',
  latestPublishedGeneration: 4,
  latestPublishedAt: '2026-09-16T06:00:00Z',
  updatedAt: '2026-09-16T07:30:00Z',
}
const missingRow: CacheGeneration = {
  ...readyRow,
  namespace: 'message.mail',
  status: 'missing',
  namespaceLabel: '邮件服务',
  statusLabel: '等待缓存重建',
  statusTone: 'warning',
  statusHint: '后端重建提示',
  publishedVersion: null,
  latestPublishedGeneration: null,
  latestPublishedAt: null,
}

const mountedWrappers: VueWrapper[] = []

describe('cache generation page', () => {
  vi.setConfig({ testTimeout: 30_000 })

  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('zh-CN')
    vi.mocked(cacheGenerationAPI.getCacheGenerationOptions).mockResolvedValue({
      publishStates: [
        { value: 'ready', label: '已发布' },
        { value: 'pending', label: '待发布' },
        { value: 'retrying', label: '重试中' },
      ],
    })
    vi.mocked(cacheGenerationAPI.getCacheGenerations).mockResolvedValue({
      list: [readyRow, retryingRow, missingRow],
      total: 3,
      page: 1,
      pageSize: 20,
    })
  })

  afterEach(() => {
    for (const wrapper of mountedWrappers.splice(0)) wrapper.unmount()
    document.body.innerHTML = ''
  })

  it('renders backend labels and forwards future filter values without business guessing', async () => {
    vi.mocked(cacheGenerationAPI.getCacheGenerations).mockResolvedValue({
      list: [
        {
          ...readyRow,
          status: 'future',
          namespaceLabel: '后端模块',
          scopeLabel: '后端作用域',
          statusLabel: '后端新状态',
          statusTone: 'info',
          publishedVersion: null,
        },
      ],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    vi.mocked(cacheGenerationAPI.getCacheGenerationOptions).mockResolvedValue({
      publishStates: [{ value: 'future', label: '后端新筛选' }],
    })
    const wrapper = mountPage(['system:cacheGeneration:list'])
    await flushPromises()
    expect(wrapper.text()).toContain('后端模块')
    expect(wrapper.text()).toContain('后端作用域')
    expect(wrapper.text()).toContain('后端新状态')
    const filter = wrapper
      .findAllComponents({ name: 'ElSelectV2' })
      .find((item) => item.attributes('data-testid') === 'cache-generation-publish-state')
    if (!filter) throw new Error('filter missing')
    expect(filter.props('options')).toEqual([{ value: 'future', label: '后端新筛选' }])
    filter.vm.$emit('update:modelValue', 'future')
    await wrapper.get('[data-testid="cache-generation-search"]').trigger('click')
    await flushPromises()
    expect(cacheGenerationAPI.getCacheGenerations).toHaveBeenLastCalledWith(
      expect.objectContaining({ publishState: 'future' }),
    )
  })

  it('clears rows and details on lost list access and rejects a late response', async () => {
    const wrapper = mountPage(['system:cacheGeneration:list'])
    await flushPromises()
    await wrapper.get('[data-testid="cache-generation-detail"]').trigger('click')
    const late = deferred<CacheGenerationPage>()
    vi.mocked(cacheGenerationAPI.getCacheGenerations).mockReturnValueOnce(late.promise)
    wrapper.getComponent({ name: 'AppTable' }).vm.$emit('refresh')
    await nextTick()
    usePermissionStore().applySnapshot({ roleCodes: [], menuTree: [], permissionCodes: [] })
    await flushPromises()
    expect(wrapper.getComponent({ name: 'AppTable' }).props('data')).toEqual([])
    expect(wrapper.getComponent({ name: 'AppTable' }).props('loading')).toBe(false)
    expect(wrapper.getComponent({ name: 'CacheGenerationDetailDialog' }).props('row')).toBeNull()
    late.resolve({ list: [readyRow], total: 1, page: 1, pageSize: 20 })
    await flushPromises()
    expect(wrapper.getComponent({ name: 'AppTable' }).props('data')).toEqual([])
    usePermissionStore().applySnapshot({
      roleCodes: [],
      menuTree: [],
      permissionCodes: ['system:cacheGeneration:list'],
    })
    await flushPromises()
    expect(cacheGenerationAPI.getCacheGenerations).toHaveBeenCalledTimes(3)
  })

  it('renders the read-only management shell without mutation controls', async () => {
    const wrapper = mountPage(['system:cacheGeneration:list'])
    await flushPromises()

    expect(wrapper.getComponent({ name: 'AppPage' }).classes()).toContain('management-page')
    expect(wrapper.get('section.cache-generation-page').classes()).toContain('management-page')

    const search = wrapper.getComponent({ name: 'AppSearch' })
    expect(search.props('queryTestId')).toBe('cache-generation-search')
    expect(search.props('resetTestId')).toBe('cache-generation-reset')

    const table = wrapper.getComponent({ name: 'AppTable' })
    expect(table.props('rowKey')).toEqual(expect.any(Function))
    expect(table.props('ariaLabel')).toBe('配置缓存代际')
    expect(wrapper.findAll('.app-table__toolbar-left button')).toHaveLength(0)
    expect(wrapper.find('[data-testid="cache-generation-empty"]').exists()).toBe(false)

    const columnLabels = table
      .props('columns')
      .map((column: { label: string }) => column.label)
      .join('|')
    for (const label of [
      '配置项',
      '作用范围',
      '状态',
      '待同步数量',
      'Redis 版本',
      '最近错误',
      '最近同步时间',
      '同步详情',
    ]) {
      expect(columnLabels).toContain(label)
    }
    const detailColumn = table
      .props('columns')
      .find((column: { key?: string }) => column.key === 'actions')
    expect(detailColumn?.width).toBe(140)
  })

  it('renders semantic status tags with text and formats nullable values', async () => {
    const wrapper = mountPage(['system:cacheGeneration:list'])
    await flushPromises()

    expect(wrapper.findAll('[data-testid="cache-generation-status"]')).toHaveLength(3)
    const tags = wrapper.findAll('.el-tag')
    expect(tags).toHaveLength(3)
    expect(tags[0]?.classes()).toContain('el-tag--success')
    expect(tags[1]?.classes()).toContain('el-tag--danger')
    expect(tags[2]?.classes()).toContain('el-tag--warning')
    expect(wrapper.text()).toContain('就绪')
    expect(wrapper.text()).toContain('重试中')
    expect(wrapper.text()).toContain('等待缓存重建')
    expect(wrapper.text()).toContain('dependency-unavailable: redis unavailable')
  })

  it('opens technical details without changing the page identity', async () => {
    const wrapper = mountPage(['system:cacheGeneration:list'])
    await flushPromises()

    await wrapper.find('[data-testid="cache-generation-detail"]').trigger('click')
    await flushPromises()
    expect(document.body.textContent).toContain('内部命名空间')
    expect(document.body.textContent).toContain('system.setting')
    expect(wrapper.getComponent({ name: 'AppTable' }).props('ariaLabel')).toBe('配置缓存代际')
  })

  it('shows loading, empty and error states', async () => {
    const response = deferred<CacheGenerationPage>()
    vi.mocked(cacheGenerationAPI.getCacheGenerations).mockReturnValueOnce(response.promise)
    const wrapper = mountPage(['system:cacheGeneration:list'])
    await nextTick()
    expect(wrapper.getComponent({ name: 'AppTable' }).props('resultState')).toBe('loading')

    response.resolve({ list: [], total: 0, page: 1, pageSize: 20 })
    await flushPromises()
    expect(wrapper.getComponent({ name: 'AppTable' }).props('resultState')).toBe('empty')

    wrapper.unmount()
    vi.mocked(cacheGenerationAPI.getCacheGenerations).mockRejectedValueOnce(
      new Error('unavailable'),
    )
    const failed = mountPage(['system:cacheGeneration:list'])
    await flushPromises()
    expect(failed.getComponent({ name: 'AppTable' }).props('resultState')).toBe('error')
    expect(failed.text()).toContain('配置缓存代际加载失败')
  }, 30_000)

  it('does not request without the list permission', async () => {
    const wrapper = mountPage([])
    await flushPromises()

    expect(cacheGenerationAPI.getCacheGenerations).not.toHaveBeenCalled()
    expect(wrapper.getComponent({ name: 'AppTable' }).props('resultState')).toBe('empty')
  })

  it('submits normalized filters and keeps them while paging', async () => {
    const wrapper = mountPage(['system:cacheGeneration:list'])
    await flushPromises()

    await wrapper.get('[data-testid="cache-generation-keyword"]').setValue(' system ')
    const publishStateFilter = wrapper
      .findAllComponents({ name: 'ElSelectV2' })
      .find((component) => component.attributes('data-testid') === 'cache-generation-publish-state')
    if (publishStateFilter === undefined) throw new Error('publish state filter not found')
    publishStateFilter.vm.$emit('update:modelValue', 'retrying')
    await wrapper.get('[data-testid="cache-generation-search"]').trigger('click')
    await flushPromises()
    expect(cacheGenerationAPI.getCacheGenerations).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 20,
      keyword: 'system',
      publishState: 'retrying',
    })

    wrapper.getComponent({ name: 'ElPagination' }).vm.$emit('current-change', 2)
    await flushPromises()
    expect(cacheGenerationAPI.getCacheGenerations).toHaveBeenLastCalledWith({
      page: 2,
      pageSize: 20,
      keyword: 'system',
      publishState: 'retrying',
    })
  })
})

function mountPage(permissionCodes: string[]): VueWrapper {
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes })
  const wrapper = mount(CacheGenerationPageView, {
    attachTo: document.body,
    global: { plugins: [pinia, appI18n, ElementPlus] },
  })
  mountedWrappers.push(wrapper)
  return wrapper
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
