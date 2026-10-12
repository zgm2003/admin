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
})
afterEach(() => wrappers.splice(0).forEach((w) => w.unmount()))
it('requests paginated own files and resets filters to the first page', async () => {
  const wrapper = page()
  await flushPromises()
  expect(api.listFiles).toHaveBeenLastCalledWith(
    { page: 1, pageSize: 20, keyword: '', category: '', purpose: '' },
    expect.any(AbortSignal),
  )
  const table = wrapper.findComponent({ name: 'AppTable' })
  table.vm.$emit('update:pagination', { currentPage: 2, pageSize: 20, total: 42 })
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
it('uses shared search and table components with localized search placeholders', async () => {
  const wrapper = page()
  await flushPromises()
  expect(wrapper.findComponent({ name: 'AppSearch' }).exists()).toBe(true)
  expect(wrapper.findComponent({ name: 'AppTable' }).exists()).toBe(true)
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
  vi.mocked(api.listFiles).mockReturnValueOnce(
    new Promise((r) => {
      resolve = r
    }),
  )
  const wrapper = page()
  await flushPromises()
  wrapper
    .findComponent({ name: 'AppTable' })
    .vm.$emit('update:pagination', { currentPage: 2, pageSize: 20, total: 42 })
  await flushPromises()
  resolve({ list: [{ ...row, fileName: 'stale' }], total: 42, page: 1, pageSize: 20 })
  await flushPromises()
  expect(
    wrapper.findComponent(FileBrowser).findComponent({ name: 'AppTable' }).props('data'),
  ).toEqual([row])
})
