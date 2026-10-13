import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus from 'element-plus'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { beforeEach, afterEach, expect, it, vi } from 'vitest'
import { appI18n } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import * as api from '@/api/storage/file'
import FilePage from '@/views/storage/file/index.vue'
import FileBrowser from '@/components/FileBrowser/index.vue'
vi.mock('@/api/storage/file', () => ({
  listFiles: vi.fn(),
  getFileOptions: vi.fn(),
  getFileURL: vi.fn(),
  deleteFile: vi.fn(),
  completeFileUpload: vi.fn(),
}))
const row: api.StorageFile = {
  id: 7,
  fileName: 'photo.png',
  mimeType: 'image/png',
  sizeBytes: 12,
  category: 'image',
  createdAt: '2026-10-12T00:00:00Z',
  objectKey: 'key',
}
const wrappers: VueWrapper[] = []
function page(codes = ['storage:file:list', 'storage:file:detail', 'storage:file:delete']) {
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore().applySnapshot({ roleCodes: [], menuTree: [], permissionCodes: codes })
  const wrapper = mount(FilePage, { global: { plugins: [pinia, ElementPlus, appI18n] } })
  wrappers.push(wrapper)
  return wrapper
}
beforeEach(() => {
  vi.clearAllMocks()
  appI18n.global.locale.value = 'zh-CN'
  vi.mocked(api.getFileOptions).mockResolvedValue({
    categories: [{ value: 'image', label: '图片' }],
    uploadRules: [],
  })
  vi.mocked(api.listFiles).mockResolvedValue({ list: [row], total: 42, page: 1, pageSize: 20 })
  vi.mocked(api.getFileURL).mockResolvedValue({
    url: 'https://files.example.test/photo.png',
    expiresAt: '2099-01-01T00:00:00Z',
  })
})
afterEach(() => wrappers.splice(0).forEach((w) => w.unmount()))
it('requests paginated own files and resets filters to the first page', async () => {
  const wrapper = page()
  await flushPromises()
  expect(api.listFiles).toHaveBeenLastCalledWith(
    { page: 1, pageSize: 20, keyword: '', category: '', purpose: '' },
    expect.any(AbortSignal),
  )
  const pagination = wrapper.findComponent({ name: 'ElPagination' })
  pagination.vm.$emit('current-change', 2)
  await flushPromises()
  expect(api.listFiles).toHaveBeenLastCalledWith(
    expect.objectContaining({ page: 2 }),
    expect.any(AbortSignal),
  )
  wrapper
    .findComponent({ name: 'AppSearch' })
    .vm.$emit('query', { keyword: 'photo', category: 'image' })
  await flushPromises()
  expect(api.listFiles).toHaveBeenLastCalledWith(
    expect.objectContaining({ page: 1, keyword: 'photo', category: 'image' }),
    expect.any(AbortSignal),
  )
})
it('uses shared search and renders file cards with localized search placeholders', async () => {
  const wrapper = page()
  await flushPromises()
  expect(wrapper.findComponent({ name: 'AppSearch' }).exists()).toBe(true)
  expect(wrapper.findComponent({ name: 'AppTable' }).exists()).toBe(false)
  expect(wrapper.find('.file-browser__grid').exists()).toBe(true)
  expect(wrapper.get('[data-testid="file-card-7"]').text()).toContain('photo.png')
  expect(wrapper.get('[data-testid="file-card-7"]').text()).toContain('图片')
  expect(wrapper.get('input[data-testid="file-keyword"]').attributes('placeholder')).toBe(
    '请输入文件名',
  )
  expect(wrapper.findComponent({ name: 'AppSearch' }).props('fields')).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ key: 'category', placeholder: '请选择文件类型' }),
    ]),
  )
  appI18n.global.locale.value = 'en-US'
  await flushPromises()
  expect(wrapper.get('input[data-testid="file-keyword"]').attributes('placeholder')).toBe(
    'Enter a file name',
  )
  expect(wrapper.findComponent({ name: 'AppSearch' }).props('fields')).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ key: 'category', placeholder: 'Select a file type' }),
    ]),
  )
})
it('shows media-aware card previews and keeps actions on the card', async () => {
  vi.mocked(api.listFiles).mockResolvedValueOnce({
    list: [
      row,
      { ...row, id: 8, fileName: 'clip.mp4', mimeType: 'video/mp4', category: 'video' },
      { ...row, id: 9, fileName: 'report.pdf', mimeType: 'application/pdf', category: 'document' },
    ],
    total: 3,
    page: 1,
    pageSize: 20,
  })
  const wrapper = page()
  await flushPromises()
  expect(
    wrapper.get('[data-testid="file-card-7"] .file-browser__thumbnail').attributes('data-kind'),
  ).toBe('image')
  expect(
    wrapper.get('[data-testid="file-card-8"] .file-browser__thumbnail').attributes('data-kind'),
  ).toBe('video')
  expect(
    wrapper.get('[data-testid="file-card-9"] .file-browser__thumbnail').attributes('data-kind'),
  ).toBe('file')
  expect(wrapper.get('[data-testid="file-preview-7"]')).toBeTruthy()
  expect(wrapper.get('[data-testid="file-download-7"]')).toBeTruthy()
  expect(api.getFileURL).toHaveBeenCalledWith(7, 'inline', expect.any(AbortSignal))
  expect(api.getFileURL).toHaveBeenCalledWith(8, 'inline', expect.any(AbortSignal))
})
it('opens image previews in the fullscreen image viewer', async () => {
  vi.mocked(api.getFileURL).mockResolvedValue({
    url: 'https://files.example.test/photo.png',
    expiresAt: '2099-01-01T00:00:00Z',
  })
  const wrapper = page()
  await flushPromises()
  await wrapper.get('[data-testid="file-preview-7"]').trigger('click')
  await flushPromises()
  expect(wrapper.findComponent({ name: 'ElImageViewer' }).exists()).toBe(true)
  expect(wrapper.findComponent({ name: 'AppDialog' }).exists()).toBe(false)
  expect(wrapper.findComponent({ name: 'ElImageViewer' }).props('urlList')).toEqual([
    'https://files.example.test/photo.png',
  ])
})
it('does not offer preview for non-media files and keeps download available', async () => {
  const file = {
    ...row,
    id: 10,
    fileName: 'manual.pdf',
    mimeType: 'application/pdf',
    category: 'document',
  }
  vi.mocked(api.listFiles).mockResolvedValueOnce({ list: [file], total: 1, page: 1, pageSize: 20 })
  vi.mocked(api.getFileURL).mockResolvedValue({
    url: 'https://files.example.test/manual.pdf',
    expiresAt: '2099-01-01T00:00:00Z',
  })
  const wrapper = page()
  await flushPromises()
  expect(wrapper.find('[data-testid="file-preview-10"]').exists()).toBe(false)
  expect(wrapper.find('[data-testid="file-download-10"]').exists()).toBe(true)
  expect(api.getFileURL).not.toHaveBeenCalled()
  expect(wrapper.findComponent({ name: 'ElImageViewer' }).exists()).toBe(false)
})
it('uploads with the backend-selected rule without a technical purpose selector', async () => {
  vi.mocked(api.getFileOptions).mockResolvedValue({
    categories: [{ value: 'image', label: '图片' }],
    uploadRules: [
      { value: 'file', label: '默认100MB', accept: '.png,image/png', maxFileSizeBytes: 104857600 },
    ],
  })
  const wrapper = page(['storage:file:list', 'storage:object:upload'])
  await flushPromises()
  expect(wrapper.find('.file-browser__purpose').exists()).toBe(false)
  expect(wrapper.findComponent({ name: 'UpMedia' }).props('ruleCode')).toBe('file')
  expect(wrapper.findComponent({ name: 'UpMedia' }).props('variant')).toBe('button')
  expect(wrapper.get('[data-testid="up-media-upload"]').text()).toBe('上传文件')
  expect(wrapper.find('.up-media__trigger').exists()).toBe(false)
  expect(wrapper.get('[data-testid="file-upload-limit"]').text()).toBe('单个文件最大 100 MB')
})
it('shows a clear unavailable hint instead of guessing another upload rule', async () => {
  const wrapper = page(['storage:file:list', 'storage:object:upload'])
  await flushPromises()
  expect(wrapper.findComponent({ name: 'UpMedia' }).exists()).toBe(false)
  expect(wrapper.text()).toContain('当前平台尚未配置可用上传规则')
})
it('displays small upload limits without rounding them down to zero MB', async () => {
  vi.mocked(api.getFileOptions).mockResolvedValue({
    categories: [],
    uploadRules: [{ value: 'file', label: 'Small files', accept: '', maxFileSizeBytes: 512 }],
  })
  const wrapper = page(['storage:file:list', 'storage:object:upload'])
  await flushPromises()
  expect(wrapper.get('[data-testid="file-upload-limit"]').text()).toBe('单个文件最大 512 B')
})
it('does not request own files without independent list permission', async () => {
  page(['storage:file:view'])
  await flushPromises()
  expect(api.listFiles).not.toHaveBeenCalled()
})
it('canceled delete sends no request and operations have explicit types and no icons', async () => {
  vi.spyOn(ElMessageBox, 'confirm').mockRejectedValueOnce('cancel')
  const wrapper = page()
  await flushPromises()
  await wrapper.get('[data-testid="file-delete-7"]').trigger('click')
  await flushPromises()
  expect(api.deleteFile).not.toHaveBeenCalled()
  const button = wrapper
    .findAllComponents({ name: 'ElButton' })
    .find((b) => b.attributes('data-testid') === 'file-delete-7')
  expect(button?.props('type')).toBe('danger')
  expect(button?.props('icon')).toBeUndefined()
})
it('ignores a late page response after a newer page request', async () => {
  let resolve: (value: Awaited<ReturnType<typeof api.listFiles>>) => void = () => {}
  vi.mocked(api.listFiles).mockResolvedValueOnce({ list: [row], total: 42, page: 1, pageSize: 20 })
  vi.mocked(api.listFiles).mockReturnValueOnce(
    new Promise((r) => {
      resolve = r
    }),
  )
  const wrapper = page()
  await flushPromises()
  wrapper.findComponent({ name: 'ElPagination' }).vm.$emit('current-change', 2)
  await flushPromises()
  wrapper
    .findComponent({ name: 'AppSearch' })
    .vm.$emit('query', { keyword: 'latest', category: '' })
  await flushPromises()
  resolve({ list: [{ ...row, fileName: 'stale' }], total: 42, page: 1, pageSize: 20 })
  await flushPromises()
  expect(wrapper.findComponent(FileBrowser).findAll('[data-testid^="file-card-"]')).toHaveLength(1)
  expect(wrapper.find('[data-testid="file-card-7"]').exists()).toBe(true)
  expect(wrapper.get('[data-testid="file-card-7"]').text()).not.toContain('stale')
})

it('cancels old thumbnails on paging without letting late responses consume the new queue', async () => {
  vi.mocked(api.listFiles)
    .mockResolvedValueOnce({
      list: Array.from({ length: 6 }, (_, index) => ({ ...row, id: index + 1 })),
      total: 42,
      page: 1,
      pageSize: 20,
    })
    .mockResolvedValueOnce({
      list: Array.from({ length: 6 }, (_, index) => ({ ...row, id: index + 20 })),
      total: 42,
      page: 2,
      pageSize: 20,
    })
  const pending = new Map<number, (value: api.FileURL) => void>()
  vi.mocked(api.getFileURL).mockImplementation(
    (id) => new Promise((resolve) => pending.set(id, resolve)),
  )
  const wrapper = page()
  await flushPromises()
  expect(api.getFileURL).toHaveBeenCalledTimes(4)
  const oldSignals = vi.mocked(api.getFileURL).mock.calls.map((call) => call[2])
  wrapper.findComponent({ name: 'ElPagination' }).vm.$emit('current-change', 2)
  await flushPromises()
  expect(oldSignals.every((signal) => signal?.aborted)).toBe(true)
  expect(api.getFileURL).toHaveBeenCalledTimes(8)
  const signed = (id: number): api.FileURL => ({
    url: `https://files.example.test/${id}.png`,
    expiresAt: '2099-01-01T00:00:00Z',
  })
  for (const id of [1, 2, 3, 4]) pending.get(id)?.(signed(id))
  await flushPromises()
  expect(api.getFileURL).toHaveBeenCalledTimes(8)
  pending.get(20)?.(signed(20))
  await flushPromises()
  expect(api.getFileURL).toHaveBeenCalledWith(24, 'inline', expect.any(AbortSignal))
  pending.get(24)?.(signed(24))
  await flushPromises()
  expect(wrapper.get('[data-testid="file-card-24"] img').attributes('src')).toBe(
    'https://files.example.test/24.png',
  )
  expect(wrapper.find('[data-testid="file-card-1"]').exists()).toBe(false)
})

it('keeps keyboard preview separate from card selection', async () => {
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore().applySnapshot({
    roleCodes: [],
    menuTree: [],
    permissionCodes: ['storage:file:list', 'storage:file:detail'],
  })
  const wrapper = mount(FileBrowser, {
    props: { selection: true },
    global: { plugins: [pinia, ElementPlus, appI18n] },
  })
  wrappers.push(wrapper)
  await flushPromises()
  await wrapper.get('[data-testid="file-preview-7"]').trigger('keydown', { key: 'Enter' })
  expect(wrapper.emitted('select')).toBeUndefined()
  await wrapper.get('[data-testid="file-card-7"]').trigger('keydown', { key: 'Enter' })
  expect(wrapper.emitted('select')).toEqual([[row]])
})
