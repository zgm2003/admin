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

vi.mock('@/api/system/cacheGeneration', () => ({ getCacheGenerations: vi.fn() }))

const readyRow: CacheGeneration = {
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
  namespace: 'system.dictionary',
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

const mountedWrappers: VueWrapper[] = []

describe('cache generation page', () => {
  vi.setConfig({ testTimeout: 30_000 })

  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('zh-CN')
    vi.mocked(cacheGenerationAPI.getCacheGenerations).mockResolvedValue({
      list: [readyRow, retryingRow],
      total: 2,
      page: 1,
      pageSize: 20,
    })
  })

  afterEach(() => {
    for (const wrapper of mountedWrappers.splice(0)) wrapper.unmount()
    document.body.innerHTML = ''
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
      '命名空间',
      'Scope',
      '当前代际',
      '状态',
      '待发布事件',
      '最早待发布时间',
      '最近尝试次数',
      '最近错误',
      '最近发布代际',
      '最近发布时间',
      '更新时间',
    ]) {
      expect(columnLabels).toContain(label)
    }
  })

  it('renders semantic status tags with text and formats nullable values', async () => {
    const wrapper = mountPage(['system:cacheGeneration:list'])
    await flushPromises()

    expect(wrapper.findAll('[data-testid="cache-generation-status"]')).toHaveLength(2)
    const tags = wrapper.findAll('.el-tag')
    expect(tags).toHaveLength(2)
    expect(tags[0]?.classes()).toContain('el-tag--success')
    expect(tags[1]?.classes()).toContain('el-tag--danger')
    expect(wrapper.text()).toContain('就绪')
    expect(wrapper.text()).toContain('重试中')
    expect(wrapper.text()).toContain('dependency-unavailable: redis unavailable')
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
