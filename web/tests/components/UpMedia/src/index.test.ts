import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { appI18n } from '@src/i18n'
import { requestUploadCredentials } from '@src/api/storage/upload'
import UpMedia from '@src/components/UpMedia/src/index.vue'

vi.mock('@src/api/storage/upload', () => ({ requestUploadCredentials: vi.fn() }))
const credentialsMock = vi.mocked(requestUploadCredentials)

describe('UpMedia', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true }))
  })

  it('uploads one file and emits its object key instead of the upload URL', async () => {
    credentialsMock.mockResolvedValue({ items: [{ uploadUrl: 'https://cos.example/upload', objectKey: 'avatar/2026/08/30/a.png', method: 'PUT', headers: { 'Content-Type': 'image/png' }, expiresAt: '2026-08-30T00:10:00Z', publicUrl: 'https://cdn.example/avatar/2026/08/30/a.png' }] })
    const wrapper = mountComponent({ modelValue: '', ruleCode: 'avatar' })

    await chooseFiles(wrapper, [new File(['image'], 'a.png', { type: 'image/png' })])

    expect(fetch).toHaveBeenCalledWith('https://cos.example/upload', expect.objectContaining({ method: 'PUT' }))
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual(['avatar/2026/08/30/a.png'])
  })

  it('appends all uploaded object keys when multiple is enabled', async () => {
    credentialsMock.mockResolvedValue({ items: [
      { uploadUrl: 'https://cos.example/a', objectKey: 'gallery/a.png', method: 'PUT', headers: {}, expiresAt: '2026-08-30T00:10:00Z' },
      { uploadUrl: 'https://cos.example/b', objectKey: 'gallery/b.png', method: 'PUT', headers: {}, expiresAt: '2026-08-30T00:10:00Z' },
    ] })
    const wrapper = mountComponent({ modelValue: ['gallery/old.png'], ruleCode: 'gallery', multiple: true })

    await chooseFiles(wrapper, [new File(['a'], 'a.png', { type: 'image/png' }), new File(['b'], 'b.png', { type: 'image/png' })])

    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([['gallery/old.png', 'gallery/a.png', 'gallery/b.png']])
  })
})

function mountComponent(props: { modelValue: string | string[]; ruleCode: string; multiple?: boolean }) {
  return mount(UpMedia, { props, global: { plugins: [ElementPlus, appI18n] } })
}

async function chooseFiles(wrapper: ReturnType<typeof mount>, files: File[]): Promise<void> {
  const input = wrapper.get('[data-testid="up-media-input"]')
  Object.defineProperty(input.element, 'files', { configurable: true, value: files })
  await input.trigger('change')
  await flushPromises()
}
