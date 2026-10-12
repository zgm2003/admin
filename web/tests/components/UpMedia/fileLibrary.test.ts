import { DOMWrapper, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import * as fileAPI from '@/api/storage/file'
import { appI18n, setLocale } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import UpMedia from '@/components/UpMedia/index.vue'
import type { UpMediaProps } from '@/components/UpMedia/types'

vi.mock('@/api/storage/file', () => ({
  listFiles: vi.fn(),
  getFileOptions: vi.fn(),
  getFileURL: vi.fn(),
  completeFileUpload: vi.fn(),
}))
vi.mock('@/api/storage/upload', () => ({
  requestUploadCredentials: vi.fn(),
  requestObjectURL: vi
    .fn()
    .mockResolvedValue({ url: 'https://cdn.example/photo.png', expiresAt: null }),
}))

const photo: fileAPI.StorageFile = {
  id: 7,
  fileName: 'photo.png',
  objectKey: 'file/photo.png',
  mimeType: 'image/png',
  category: 'image',
  sizeBytes: 12,
  createdAt: '2026-10-12T00:00:00Z',
}
const wrappers: VueWrapper[] = []

beforeEach(() => {
  vi.clearAllMocks()
  setLocale('zh-CN')
  vi.mocked(fileAPI.getFileOptions).mockResolvedValue({ categories: [], uploadRules: [] })
  vi.mocked(fileAPI.listFiles).mockResolvedValue({ list: [photo], total: 1, page: 1, pageSize: 20 })
})
afterEach(() => {
  wrappers.splice(0).forEach((wrapper) => wrapper.unmount())
  document.body.innerHTML = ''
  setLocale('zh-CN')
})

function media(props: Partial<UpMediaProps> = {}) {
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore().applySnapshot({
    roleCodes: [],
    menuTree: [],
    permissionCodes: ['storage:file:list', 'storage:file:detail'],
  })
  const wrapper = mount(UpMedia, {
    attachTo: document.body,
    props: { modelValue: '', ruleCode: 'avatar', ...props },
    global: { plugins: [pinia, ElementPlus, appI18n] },
  })
  wrappers.push(wrapper)
  return wrapper
}

async function openLibrary(wrapper: VueWrapper) {
  await wrapper.get('[data-testid="up-media-library"]').trigger('click')
  await vi.dynamicImportSettled()
  await flushPromises()
}

async function selectPhoto() {
  await new DOMWrapper(document.body).get('[data-testid="file-select-7"]').trigger('click')
  await flushPromises()
}

it('keeps the library opt-in and does not load file options or lists before it opens', async () => {
  const wrapper = media()
  await flushPromises()
  expect(wrapper.find('[data-testid="up-media-library"]').exists()).toBe(false)
  await wrapper.setProps({ allowFileLibrary: true })
  expect(wrapper.get('[data-testid="up-media-library"]').text()).toBe('选择我的文件')
  expect(fileAPI.listFiles).not.toHaveBeenCalled()
  expect(fileAPI.getFileOptions).not.toHaveBeenCalled()
})

it('selects an existing file without uploading and returns both the key and file metadata', async () => {
  const wrapper = media({
    variant: 'image',
    allowFileLibrary: true,
    fileLibraryPurpose: 'avatar',
    uploadDisabled: true,
  })
  await openLibrary(wrapper)
  expect(fileAPI.listFiles).toHaveBeenCalledWith(
    expect.objectContaining({ purpose: 'avatar', page: 1, pageSize: 20 }),
    expect.any(AbortSignal),
  )
  await selectPhoto()
  expect(wrapper.emitted('selected')).toEqual([[photo]])
  expect(wrapper.emitted('update:modelValue')).toEqual([['file/photo.png']])
  expect(wrapper.emitted('uploaded')).toBeUndefined()
  await wrapper.setProps({ modelValue: 'file/photo.png' })
  await wrapper.get('.up-media__clear').trigger('click')
  expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([''])
})

it.each(['image', 'file', 'button'] as const)(
  'appends library selections for multiple %s media',
  async (variant) => {
    const wrapper = media({
      modelValue: ['old.png'],
      multiple: true,
      variant,
      allowFileLibrary: true,
    })
    await openLibrary(wrapper)
    await selectPhoto()
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([['old.png', 'file/photo.png']])
    await wrapper.setProps({ modelValue: ['old.png', 'file/photo.png'] })
    await openLibrary(wrapper)
    await selectPhoto()
    expect(wrapper.emitted('update:modelValue')).toHaveLength(1)
  },
)

it('rejects a library file that does not match the field accept constraint', async () => {
  const wrapper = media({ variant: 'file', allowFileLibrary: true, accept: '.xlsx' })
  await openLibrary(wrapper)
  await selectPhoto()
  expect(wrapper.emitted('selected')).toBeUndefined()
  expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  expect(wrapper.get('[role="alert"]').text()).toContain('文件类型不符合要求')
})

it('blocks selection when disabled and closes an open library if the capability is revoked', async () => {
  const wrapper = media({ allowFileLibrary: true, disabled: true })
  expect(wrapper.get('[data-testid="up-media-library"]').attributes('disabled')).toBeDefined()
  await wrapper.setProps({ disabled: false })
  await openLibrary(wrapper)
  await wrapper.setProps({ allowFileLibrary: false })
  await flushPromises()
  expect(document.body.querySelector('[data-testid="file-select-7"]')).toBeNull()
  expect(wrapper.emitted('selected')).toBeUndefined()
})

it.each(['image', 'video'] as const)(
  'keeps a square upload entry above the optional library button for %s',
  async (variant) => {
    const wrapper = media({ variant, allowFileLibrary: true })
    const upload = wrapper.get('[data-testid="up-media-upload"]')
    const library = wrapper.get('[data-testid="up-media-library"]')
    expect(
      upload.element.compareDocumentPosition(library.element) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy()
    expect(library.text()).toBe('选择我的文件')
    expect(library.find('svg').exists()).toBe(false)
    expect(wrapper.text()).not.toMatch(/上传头像|更换头像|选择头像/u)
    await wrapper.setProps({ modelValue: 'file/photo.png' })
    await flushPromises()
    expect(wrapper.find('[data-testid="up-media-upload"]').exists()).toBe(true)
    await wrapper.setProps({ disabled: true })
    expect(wrapper.get('[data-testid="up-media-upload"]').attributes('disabled')).toBeDefined()
  },
)

it.each(['file', 'button'] as const)(
  'uses compact upload and library buttons without a square for %s',
  async (variant) => {
    const wrapper = media({ variant, allowFileLibrary: true })
    expect(wrapper.get('[data-testid="up-media-upload"]').text()).toBe('上传文件')
    expect(wrapper.find('.up-media__trigger').exists()).toBe(false)
    expect(wrapper.find('.up-media__uploads').exists()).toBe(false)
    expect(wrapper.get('[data-testid="up-media-library"]').text()).toBe('选择我的文件')
    await openLibrary(wrapper)
    await selectPhoto()
    expect(wrapper.emitted('selected')).toEqual([[photo]])
  },
)

it('localizes the generic upload and library controls', async () => {
  const wrapper = media({ allowFileLibrary: true })
  expect(wrapper.get('[data-testid="up-media-upload"]').attributes('aria-label')).toBe('选择文件')
  setLocale('en-US')
  await flushPromises()
  expect(wrapper.get('[data-testid="up-media-library"]').text()).toBe('Choose My Files')
  expect(wrapper.get('[data-testid="up-media-upload"]').attributes('aria-label')).toBe(
    'Select file',
  )
})
