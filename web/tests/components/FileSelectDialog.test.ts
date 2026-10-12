import { DOMWrapper, flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus from 'element-plus'
import { expect, it, vi } from 'vitest'
import { appI18n } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import FileSelectDialog from '@/components/FileSelectDialog/index.vue'
import * as api from '@/api/storage/file'
vi.mock('@/api/storage/file', () => ({
  getFileOptions: vi.fn(),
  listFiles: vi.fn(),
  getFileURL: vi.fn(),
}))
it('requests avatar-compatible files and emits selection without copying or saving', async () => {
  const row = {
    id: 7,
    fileName: 'a.png',
    mimeType: 'image/png',
    sizeBytes: 2,
    category: 'image',
    createdAt: '2026-10-12T00:00:00Z',
    objectKey: 'key',
  }
  vi.mocked(api.getFileOptions).mockResolvedValue({
    categories: [{ value: 'image', label: '图片' }],
    uploadRules: [],
  })
  vi.mocked(api.listFiles).mockResolvedValue({ list: [row], total: 1, page: 1, pageSize: 20 })
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore().applySnapshot({
    roleCodes: [],
    menuTree: [],
    permissionCodes: ['storage:file:list', 'storage:file:detail'],
  })
  const wrapper = mount(FileSelectDialog, {
    attachTo: document.body,
    props: { modelValue: true, purpose: 'avatar' },
    global: { plugins: [pinia, ElementPlus, appI18n] },
  })
  await flushPromises()
  expect(api.listFiles).toHaveBeenCalledWith(
    expect.objectContaining({ purpose: 'avatar' }),
    expect.any(AbortSignal),
  )
  await new DOMWrapper(document.body).get('[data-testid="file-select-7"]').trigger('click')
  expect(wrapper.emitted('select')).toEqual([[row]])
  wrapper.unmount()
  document.body.innerHTML = ''
})
