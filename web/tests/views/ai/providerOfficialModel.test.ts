import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ElementPlus, { ElNotification } from 'element-plus'
import ProviderPage from '@/views/ai/provider/index.vue'
import OfficialPage from '@/views/ai/officialModel/index.vue'
import PriceDialog from '@/views/ai/provider/components/PriceDialog/index.vue'
import * as provider from '@/api/ai/provider'
import * as official from '@/api/ai/officialModel'
import { pinia } from '@/store'
import { usePermissionStore } from '@/store/permission'
import { appI18n, setLocale } from '@/i18n'

vi.mock('@/api/ai/provider', () => ({
  getProviders: vi.fn(),
  getProvider: vi.fn(),
  getProviderOptions: vi.fn(),
  createProvider: vi.fn(),
  updateProvider: vi.fn(),
  updateProviderStatus: vi.fn(),
  deleteProvider: vi.fn(),
  testProvider: vi.fn(),
  getProviderCandidates: vi.fn(),
  getProviderModels: vi.fn(),
  saveProviderModels: vi.fn(),
  updateProviderModelStatus: vi.fn(),
  previewProviderCatalog: vi.fn(),
  confirmProviderCatalog: vi.fn(),
  getProviderPrices: vi.fn(),
  syncProviderPrice: vi.fn(),
  previewProviderPrice: vi.fn(),
  publishProviderPrice: vi.fn(),
}))
vi.mock('@/api/ai/officialModel', () => ({
  getOfficialModels: vi.fn(),
  getOfficialModel: vi.fn(),
  getOfficialModelOptions: vi.fn(),
  createOfficialModel: vi.fn(),
  updateOfficialModel: vi.fn(),
  updateOfficialModelStatus: vi.fn(),
  getOfficialModelPrices: vi.fn(),
}))
const timestamp = '2026-10-29T00:00:00Z'
const providerRow: provider.Provider = {
  id: '1',
  code: 'fixture',
  name: 'Fixture provider',
  description: 'Provider description',
  status: 1,
  currentVersion: '1',
  apiKeyConfigured: true,
  createdAt: timestamp,
  updatedAt: timestamp,
}
const model: provider.ProviderModel = {
  id: '2',
  providerId: '1',
  upstreamModelId: 'fixture-model',
  name: 'Existing upstream model',
  officialModelId: '3',
  operations: ['text.generate'],
  limits: {},
  status: 1,
  createdAt: timestamp,
  updatedAt: timestamp,
}
const officialRow: official.OfficialModel = {
  id: '3',
  vendorCode: 'fixture',
  modelKey: 'standard-model',
  name: 'Reference model',
  operations: ['text.generate'],
  sourceUrls: ['https://example.test/reference'],
  status: 1,
  createdAt: timestamp,
  updatedAt: timestamp,
}
const published: provider.PriceVersion = {
  id: '4',
  providerModelId: '2',
  version: '1',
  status: 1,
  effectiveAt: timestamp,
  currency: 'CNY',
  sourceCurrency: 'USD',
  sourceDigest: 'source-digest',
  snapshotDigest: 'published-digest',
  syncedAt: timestamp,
  rates: [
    {
      id: '5',
      operation: 'text.generate',
      usageKey: 'input',
      selector: {},
      unit: 'token',
      unitQuantity: '1000000',
      sourceUnitPrice: '0.25000000',
      billingUnitPrice: '1.25000000',
    },
  ],
}
const draft: provider.PriceVersion = {
  ...published,
  id: '6',
  version: '2',
  status: 0,
  snapshotDigest: 'preview-digest',
}
const wrappers: VueWrapper[] = []
const globalOptions = {
  plugins: [pinia, appI18n, ElementPlus],
  stubs: {
    AppDialog: {
      props: ['modelValue'],
      template: '<section v-if="modelValue"><slot /><slot name="footer" /></section>',
    },
  },
}
function permissions(codes: string[]) {
  usePermissionStore(pinia).permissionCodes = codes
}
async function providerPage() {
  const w = mount(ProviderPage, { global: globalOptions })
  wrappers.push(w)
  await flushPromises()
  return w
}
async function pricePage() {
  const w = mount(PriceDialog, {
    props: { modelValue: true, providerId: '1', modelId: '2', modelName: 'Fixture model' },
    global: globalOptions,
  })
  wrappers.push(w)
  await flushPromises()
  return w
}
beforeEach(() => {
  vi.resetAllMocks()
  setLocale('zh-CN')
  usePermissionStore(pinia).reset()
  vi.mocked(provider.getProviderOptions).mockResolvedValue({
    statuses: [
      { value: 0, label: '停用' },
      { value: 1, label: '启用' },
    ],
    textProtocols: [],
    operations: [{ value: 'text.generate', label: '文字生成' }],
    defaults: { requestTimeoutMs: 30000, status: 0 },
    constraints: {
      requestTimeoutMs: { min: 1000, max: 300000 },
      maxNameLength: 100,
      maxDescriptionLength: 1000,
    },
  })
  vi.mocked(provider.getProviders).mockResolvedValue({
    list: [providerRow],
    page: 1,
    pageSize: 20,
    total: 1,
  })
  vi.mocked(provider.getProviderModels).mockResolvedValue({
    list: [model],
    page: 1,
    pageSize: 20,
    total: 1,
  })
  vi.mocked(provider.getProviderCandidates).mockResolvedValue({
    list: [],
    page: 1,
    pageSize: 20,
    total: 0,
  })
  vi.mocked(provider.getProviderPrices).mockResolvedValue({
    list: [published],
    page: 1,
    pageSize: 20,
    total: 1,
  })
  vi.mocked(provider.syncProviderPrice).mockResolvedValue(draft)
  vi.mocked(provider.previewProviderPrice).mockResolvedValue({
    ...draft,
    id: '7',
    snapshotDigest: 'edited-preview',
    rates: published.rates.map((rate) => ({ ...rate, billingUnitPrice: '2.12345678' })),
  })
  vi.mocked(official.getOfficialModelOptions).mockResolvedValue({
    statuses: [
      { value: 0, label: '停用' },
      { value: 1, label: '启用' },
    ],
    operations: [{ value: 'text.generate', label: '文字生成' }],
    defaults: { status: 0 },
    constraints: { maxNameLength: 100, maxModelKeyLength: 200, maxSourceUrls: 20 },
  })
  vi.mocked(official.getOfficialModels).mockResolvedValue({
    list: [officialRow],
    page: 1,
    pageSize: 20,
    total: 41,
  })
  vi.mocked(official.getOfficialModel).mockResolvedValue(officialRow)
  vi.mocked(official.getOfficialModelPrices).mockResolvedValue({
    list: [
      {
        providerModelId: '2',
        providerId: '1',
        providerName: 'Fixture provider',
        upstreamModelId: 'fixture-model',
        priceVersionId: '4',
        version: '1',
        effectiveAt: timestamp,
        currency: 'CNY',
        sourceCurrency: 'USD',
        rates: published.rates,
      },
    ],
    page: 1,
    pageSize: 20,
    total: 1,
  })
})
afterEach(() => {
  wrappers.forEach((w) => w.unmount())
  wrappers.length = 0
  ElNotification.closeAll()
  document.body.innerHTML = ''
})

it('keeps both catalog pages independent from list and write permissions', async () => {
  permissions(['ai:provider:view', 'ai:officialModel:view'])
  const p = await providerPage()
  const o = mount(OfficialPage, { global: globalOptions })
  wrappers.push(o)
  await flushPromises()
  expect(provider.getProviders).not.toHaveBeenCalled()
  expect(official.getOfficialModels).not.toHaveBeenCalled()
  expect(p.find('[data-testid="ai-provider-create"]').exists()).toBe(false)
  expect(o.find('[data-testid="ai-official-model-create"]').exists()).toBe(false)
  expect(p.text()).not.toContain(providerRow.name)
  expect(o.text()).not.toContain(officialRow.name)
})
it('opens real provider models and keeps synchronized catalog additions in an unselected preview', async () => {
  permissions(['ai:provider:list', 'ai:provider:detail', 'ai:provider:sync'])
  vi.mocked(provider.previewProviderCatalog).mockResolvedValue({
    providerId: '1',
    providerVersionId: '1',
    digest: 'catalog-preview-digest',
    added: [{ id: 'new-upstream', ownedBy: 'fixture', created: 1 }],
    existing: [],
  })
  const w = await providerPage()
  expect(w.text()).toContain('Fixture provider')
  await w.get('[data-testid="ai-provider-models"]').trigger('click')
  await flushPromises()
  expect(w.text()).toContain('Existing upstream model')
  await w.get('[data-testid="ai-provider-sync"]').trigger('click')
  await flushPromises()
  expect(w.find('.catalog-preview').text()).toContain('new-upstream')
  expect(w.find('.catalog-digest').text()).toContain('catalog-preview-digest')
  expect(w.get('[data-testid="ai-provider-confirm-sync"]').attributes('disabled')).toBeDefined()
  expect(w.find('.catalog-preview input[type="checkbox"]').element).toHaveProperty('checked', false)
  expect(w.text()).toContain('Existing upstream model')
  expect(provider.confirmProviderCatalog).not.toHaveBeenCalled()
})
it('shows synchronized USD references and an unpublishable read-only RMB preview without price-update permission', async () => {
  permissions(['ai:provider:detail', 'ai:provider:syncPrice'])
  const w = await pricePage()
  await w.get('[data-testid="ai-price-sync"]').trigger('click')
  await flushPromises()
  expect(w.get('.price-editor output').text()).toBe('0.25000000')
  const input = w.get<HTMLInputElement>('input[data-testid="ai-price-rmb"]')
  expect(input.element.value).toBe('1.25000000')
  expect(input.element.disabled).toBe(true)
  expect(w.get('.price-digest').text()).toContain('preview-digest')
  expect(w.find('[data-testid="ai-price-publish"]').exists()).toBe(false)
  expect(w.find('[data-testid="ai-price-preview"]').exists()).toBe(false)
  expect(provider.publishProviderPrice).not.toHaveBeenCalled()
})
it('edits only RMB, requires a fresh preview, and publishes the exact reviewed version', async () => {
  permissions(['ai:provider:detail', 'ai:provider:updatePrice'])
  const w = await pricePage()
  await w.get('[data-testid="ai-price-open"]').trigger('click')
  await flushPromises()
  const input = w.get<HTMLInputElement>('input[data-testid="ai-price-rmb"]')
  expect(input.element.disabled).toBe(false)
  expect(w.get('.price-editor output').text()).toBe('0.25000000')
  expect(w.get('[data-testid="ai-price-publish"]').attributes('disabled')).toBeDefined()
  await input.setValue('2.12345678')
  await w.get('[data-testid="ai-price-preview"]').trigger('click')
  await flushPromises()
  expect(w.get('.price-editor output').text()).toBe('0.25000000')
  expect(w.get<HTMLInputElement>('input[data-testid="ai-price-rmb"]').element.value).toBe(
    '2.12345678',
  )
  expect(w.get('.price-digest').text()).toContain('edited-preview')
  expect(w.get('[data-testid="ai-price-publish"]').attributes('disabled')).toBeUndefined()
  vi.mocked(provider.publishProviderPrice).mockImplementation(async () => {
    vi.mocked(provider.getProviderPrices).mockResolvedValue({
      list: [{ ...draft, id: '7', status: 1 }],
      page: 1,
      pageSize: 20,
      total: 1,
    })
    return {}
  })
  await w.get('[data-testid="ai-price-publish"]').trigger('click')
  await flushPromises()
  expect(w.find('.price-editor').exists()).toBe(false)
  expect(w.text()).toContain('已发布')
  expect(provider.publishProviderPrice).toHaveBeenCalledWith('1', '2', '7', 'edited-preview')
})
it('paginates the official catalog and exposes detail and reference prices as read-only views', async () => {
  permissions(['ai:officialModel:list', 'ai:officialModel:detail'])
  const w = mount(OfficialPage, { global: globalOptions })
  wrappers.push(w)
  await flushPromises()
  expect(w.text()).toContain('Reference model')
  expect(w.find('[data-testid="ai-official-model-edit"]').exists()).toBe(false)
  const next = { ...officialRow, id: '8', name: 'Next reference model' }
  vi.mocked(official.getOfficialModels).mockResolvedValue({
    list: [next],
    page: 2,
    pageSize: 20,
    total: 41,
  })
  w.findComponent({ name: 'AppTable' }).vm.$emit('update:pagination', {
    currentPage: 2,
    pageSize: 20,
    total: 41,
  })
  await flushPromises()
  expect(w.text()).toContain('Next reference model')
  expect(w.text()).not.toContain('Reference model')
  await w
    .findAll('button')
    .find((button) => button.text() === appI18n.global.t('aiOfficialModel.detail'))!
    .trigger('click')
  await flushPromises()
  const name = w.get<HTMLInputElement>(
    `input[placeholder="${appI18n.global.t('aiOfficialModel.namePlaceholder')}"]`,
  )
  expect(name.element.value).toBe('Reference model')
  expect(name.element.disabled).toBe(true)
  expect(w.find('[data-testid="ai-official-model-save"]').exists()).toBe(false)
  await w.get('[data-testid="ai-official-model-prices"]').trigger('click')
  await flushPromises()
  expect(w.text()).toContain('USD 0.25000000')
  expect(w.text()).toContain('CNY 1.25000000')
  expect(w.find('[data-testid="ai-price-rmb"]').exists()).toBe(false)
})
