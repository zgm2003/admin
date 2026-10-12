import { DOMWrapper, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { appI18n } from '@/i18n'
import { requestObjectURL, requestUploadCredentials } from '@/api/storage/upload'
import { completeFileUpload } from '@/api/storage/file'
import UpMedia from '@/components/UpMedia/index.vue'
import type { UpMediaProps } from '@/components/UpMedia/types'

vi.mock('@/api/storage/upload', () => ({
  requestUploadCredentials: vi.fn(),
  requestObjectURL: vi.fn(),
}))
vi.mock('@/api/storage/file', () => ({ completeFileUpload: vi.fn() }))
const completeMock = vi.mocked(completeFileUpload)
const credentialsMock = vi.mocked(requestUploadCredentials)
const objectURLMock = vi.mocked(requestObjectURL)
const wrappers: VueWrapper[] = []

describe('UpMedia', () => {
  afterEach(() => {
    for (const wrapper of wrappers.splice(0)) wrapper.unmount()
    vi.unstubAllGlobals()
  })
  beforeEach(() => {
    vi.clearAllMocks()
    completeMock.mockImplementation(async (id) => {
      const response = await credentialsMock.mock.results.at(-1)?.value
      const item = response.items.find((entry: { fileId: number }) => entry.fileId === id)
      return { fileId: id, objectKey: item.objectKey, publicUrl: item.publicUrl }
    })
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true }))
    objectURLMock.mockResolvedValue({ url: 'https://cdn.example/existing.png', expiresAt: null })
  })

  it('uploads one file and emits its object key instead of the upload URL', async () => {
    credentialsMock.mockResolvedValue({
      items: [
        {
          fileId: 7,
          uploadUrl: 'https://cos.example/upload',
          objectKey: 'avatar/2026/08/30/a.png',
          method: 'PUT',
          headers: { 'Content-Type': 'image/png' },
          expiresAt: '2026-08-30T00:10:00Z',
          publicUrl: 'https://cdn.example/avatar/2026/08/30/a.png',
        },
      ],
    })
    const wrapper = mountComponent({ modelValue: '', ruleCode: 'avatar' })

    await chooseFiles(wrapper, [new File(['image'], 'a.png', { type: 'image/png' })])

    expect(fetch).toHaveBeenCalledWith(
      'https://cos.example/upload',
      expect.objectContaining({ method: 'PUT' }),
    )
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual(['avatar/2026/08/30/a.png'])
  })

  it('does not publish a successful PUT until backend completion succeeds', async () => {
    credentialsMock.mockResolvedValue({
      items: [
        {
          fileId: 9,
          uploadUrl: 'https://cos.example/put',
          objectKey: 'final.png',
          method: 'PUT',
          headers: {},
          expiresAt: '2030-01-01T00:00:00Z',
        },
      ],
    })
    completeMock.mockRejectedValueOnce(new Error('HEAD mismatch'))
    const wrapper = mountComponent({ modelValue: '', ruleCode: 'avatar' })
    await chooseFiles(wrapper, [new File(['image'], 'a.png', { type: 'image/png' })])
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(wrapper.emitted('uploaded')).toBeUndefined()
  })

  it('renders XLSX as a file attachment instead of requesting an image preview', async () => {
    const wrapper = mountComponent({
      modelValue: 'setting/template.xlsx',
      ruleCode: 'setting',
      variant: 'file',
      fileLabel: '收件规则导入模板.xlsx',
      accept: '.xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    })
    await flushPromises()
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.text()).toContain('收件规则导入模板.xlsx')
    expect(wrapper.find('[data-testid="up-media-download"]').exists()).toBe(true)
  })

  it('normalizes browser XLSX MIME aliases and exposes the pending upload state', async () => {
    credentialsMock.mockResolvedValue({
      items: [
        {
          fileId: 7,
          uploadUrl: 'https://cos.example/upload',
          objectKey: 'setting/template.xlsx',
          method: 'PUT',
          headers: {},
          expiresAt: '2030-01-01T00:00:00Z',
          publicUrl: 'https://cdn.example/template.xlsx',
        },
      ],
    })
    const wrapper = mountComponent({
      modelValue: '',
      ruleCode: 'setting',
      variant: 'file',
      accept: '.xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    })
    await chooseFiles(wrapper, [new File(['header'], 'template.xlsx', { type: '' })])
    expect(credentialsMock).toHaveBeenCalledWith(
      'setting',
      [
        {
          fileName: 'template.xlsx',
          contentType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
          fileSizeBytes: 6,
        },
      ],
      expect.any(AbortSignal),
    )
    expect(wrapper.emitted('uploading-change')).toContainEqual([true])
    expect(wrapper.emitted('uploading-change')?.at(-1)).toEqual([false])
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual(['setting/template.xlsx'])
  })

  it.each(['other.csv', 'other.xls', 'other.xlsm', 'other.exe'])(
    'rejects %s even with a spoofed XLSX MIME type',
    async (fileName) => {
      const wrapper = mountComponent({
        modelValue: '',
        ruleCode: 'setting',
        variant: 'file',
        accept: '.xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      })
      await chooseFiles(wrapper, [
        new File(['other'], fileName, {
          type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
        }),
      ])
      expect(credentialsMock).not.toHaveBeenCalled()
    },
  )

  it('keeps other explicitly accepted extensions available in a mixed file picker', async () => {
    credentialsMock.mockResolvedValue({
      items: [
        {
          fileId: 7,
          uploadUrl: 'https://cos.example/upload',
          objectKey: 'setting/image.png',
          method: 'PUT',
          headers: {},
          expiresAt: '2030-01-01T00:00:00Z',
        },
      ],
    })
    const wrapper = mountComponent({
      modelValue: '',
      ruleCode: 'setting',
      variant: 'file',
      accept: '.xlsx,.png',
    })
    await chooseFiles(wrapper, [new File(['image'], 'image.png', { type: 'image/png' })])
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual(['setting/image.png'])
  })

  it('shows an inline upload failure while preserving the existing file binding', async () => {
    credentialsMock.mockRejectedValueOnce(new Error('credential request failed'))
    const wrapper = mountComponent({
      modelValue: 'setting/old.xlsx',
      ruleCode: 'setting',
      variant: 'file',
      accept: '.xlsx',
    })
    await chooseFiles(wrapper, [
      new File(['csv'], 'new.xlsx', {
        type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      }),
    ])
    expect(wrapper.text()).toContain('文件上传失败')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(fetch).not.toHaveBeenCalled()
  })

  it('does not upload or bind an old selection after upload is disabled', async () => {
    const pending = deferred<Awaited<ReturnType<typeof requestUploadCredentials>>>()
    credentialsMock.mockReturnValueOnce(pending.promise)
    const wrapper = mountComponent({
      modelValue: '',
      ruleCode: 'setting',
      variant: 'file',
      accept: '.xlsx',
    })
    await chooseFiles(wrapper, [
      new File(['csv'], 'template.xlsx', {
        type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      }),
    ])
    await wrapper.setProps({ uploadDisabled: true })
    pending.resolve({
      items: [
        {
          fileId: 7,
          uploadUrl: 'https://cos.example/upload',
          objectKey: 'setting/template.xlsx',
          method: 'PUT',
          headers: {},
          expiresAt: '2030-01-01T00:00:00Z',
        },
      ],
    })
    await flushPromises()
    expect(fetch).not.toHaveBeenCalled()
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('cancels a pending upload when the parent replaces the model value', async () => {
    const pending = deferred<Awaited<ReturnType<typeof requestUploadCredentials>>>()
    credentialsMock.mockReturnValueOnce(pending.promise)
    const wrapper = mountComponent({
      modelValue: 'setting/old.xlsx',
      ruleCode: 'setting',
      variant: 'file',
      accept: '.xlsx',
    })
    await chooseFiles(wrapper, [
      new File(['csv'], 'template.xlsx', {
        type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      }),
    ])
    await wrapper.setProps({ modelValue: 'setting/replaced.xlsx' })
    pending.resolve({
      items: [
        {
          fileId: 7,
          uploadUrl: 'https://cos.example/upload',
          objectKey: 'setting/uploaded.xlsx',
          method: 'PUT',
          headers: {},
          expiresAt: '2030-01-01T00:00:00Z',
        },
      ],
    })
    await flushPromises()
    expect(fetch).not.toHaveBeenCalled()
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(wrapper.emitted('uploading-change')?.at(-1)).toEqual([false])
  })

  it('appends all uploaded object keys when multiple is enabled', async () => {
    credentialsMock.mockResolvedValue({
      items: [
        {
          fileId: 7,
          uploadUrl: 'https://cos.example/a',
          objectKey: 'gallery/a.png',
          method: 'PUT',
          headers: {},
          expiresAt: '2026-08-30T00:10:00Z',
        },
        {
          fileId: 8,
          uploadUrl: 'https://cos.example/b',
          objectKey: 'gallery/b.png',
          method: 'PUT',
          headers: {},
          expiresAt: '2026-08-30T00:10:00Z',
        },
      ],
    })
    const wrapper = mountComponent({
      modelValue: ['gallery/old.png'],
      ruleCode: 'gallery',
      multiple: true,
    })

    await chooseFiles(wrapper, [
      new File(['a'], 'a.png', { type: 'image/png' }),
      new File(['b'], 'b.png', { type: 'image/png' }),
    ])

    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([
      ['gallery/old.png', 'gallery/a.png', 'gallery/b.png'],
    ])
  })

  it('retains confirmed files when a later file fails completion', async () => {
    credentialsMock.mockResolvedValue({
      items: [
        {
          fileId: 1,
          uploadUrl: 'https://cos.example/a',
          objectKey: 'a.png',
          method: 'PUT',
          headers: {},
          expiresAt: '2030-01-01T00:00:00Z',
        },
        {
          fileId: 2,
          uploadUrl: 'https://cos.example/b',
          objectKey: 'b.png',
          method: 'PUT',
          headers: {},
          expiresAt: '2030-01-01T00:00:00Z',
        },
      ],
    })
    completeMock
      .mockResolvedValueOnce({ fileId: 1, objectKey: 'a.png' })
      .mockRejectedValueOnce(new Error('complete failed'))
    const wrapper = mountComponent({ modelValue: ['old.png'], ruleCode: 'gallery', multiple: true })
    await chooseFiles(wrapper, [
      new File(['a'], 'a.png', { type: 'image/png' }),
      new File(['b'], 'b.png', { type: 'image/png' }),
    ])
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([['old.png', 'a.png']])
    expect(wrapper.emitted('uploaded')?.at(-1)).toEqual([[{ fileId: 1, objectKey: 'a.png' }]])
  })

  it('uploads from an empty image square and previews or replaces an existing image', async () => {
    const empty = mountComponent({ modelValue: '', ruleCode: 'avatar', variant: 'image' })
    const click = vi.spyOn(HTMLInputElement.prototype, 'click').mockImplementation(() => {})
    await empty.get('[data-testid="up-media-upload"]').trigger('click')
    expect(click).toHaveBeenCalledOnce()

    const filled = mountComponent({
      modelValue: 'avatar/alice.png',
      ruleCode: 'avatar',
      variant: 'image',
    })
    await flushPromises()
    await filled.get('.up-media__preview').trigger('click')
    await flushPromises()
    expect(document.body.querySelector('[data-testid="up-media-preview-image"]')).not.toBeNull()
    expect(click).toHaveBeenCalledOnce()
    await filled.get('[data-testid="up-media-replace"]').trigger('click')
    expect(click).toHaveBeenCalledTimes(2)
    expect(filled.find('.up-media__clear').exists()).toBe(true)
    click.mockRestore()
  })

  it('shows video thumbnails and opens a player without treating video as an image', async () => {
    const wrapper = mountComponent({ modelValue: 'video/clip.mp4', ruleCode: 'file', variant: 'video' })
    await flushPromises()
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.get('video').attributes()).toMatchObject({ preload: 'metadata', muted: '' })
    expect(wrapper.get('input').attributes('accept')).toBe('video/*')
    await wrapper.get('.up-media__preview').trigger('click')
    await flushPromises()
    const player = new DOMWrapper(document.body).get('[data-testid="up-media-preview-video"]')
    expect(player.attributes('controls')).toBeDefined()
    expect(player.attributes('autoplay')).toBeUndefined()
    await wrapper.setProps({ modelValue: '' })
    await flushPromises()
    expect(document.body.querySelector('[data-testid="up-media-preview-video"]')).toBeNull()
  })

  it('keeps a button upload free of attachments and signed preview requests', async () => {
    const wrapper = mountComponent({ modelValue: ['old.pdf'], ruleCode: 'file', variant: 'button', multiple: true })
    await flushPromises()
    expect(wrapper.get('[data-testid="up-media-upload"]').text()).toBe('上传文件')
    expect(wrapper.find('[data-testid="up-media-file"]').exists()).toBe(false)
    expect(objectURLMock).not.toHaveBeenCalled()
    expect(wrapper.get('input').attributes('accept')).toBe('')
    credentialsMock.mockResolvedValue({ items: [{ fileId: 12, uploadUrl: 'https://cos.example/pdf', objectKey: 'new.pdf', method: 'PUT', headers: {}, expiresAt: '2030-01-01T00:00:00Z' }] })
    await chooseFiles(wrapper, [new File(['pdf'], 'report.pdf', { type: 'application/pdf' })])
    expect(wrapper.emitted('uploaded')?.at(-1)).toEqual([[{fileId: 12, objectKey: 'new.pdf', publicUrl: undefined}]])
    expect(objectURLMock).not.toHaveBeenCalled()
  })

  it('replaces one attachment while retaining the other attachments and original filenames', async () => {
    const wrapper = mountComponent({ modelValue: ['old.pdf', 'other.pdf'], ruleCode: 'file', variant: 'file', multiple: true })
    await flushPromises()
    credentialsMock.mockResolvedValue({items: [{fileId: 13, uploadUrl: 'https://cos.example/replacement', objectKey: 'new.pdf', method: 'PUT', headers: {}, expiresAt: '2030-01-01T00:00:00Z'}]})
    const click = vi.spyOn(HTMLInputElement.prototype, 'click').mockImplementation(() => {})
    await wrapper.get('[data-testid="up-media-replace"]').trigger('click')
    await chooseFiles(wrapper, [new File(['pdf'], 'report.pdf', {type: 'application/pdf'})])
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([['new.pdf', 'other.pdf']])
    await wrapper.setProps({modelValue: ['new.pdf', 'other.pdf']})
    expect(wrapper.text()).toContain('report.pdf')
    expect(wrapper.find('.up-media__trigger').exists()).toBe(false)
    click.mockRestore()
  })

  it('restores the preview URL for an existing object key after reload', async () => {
    objectURLMock.mockResolvedValue({
      url: 'https://cdn.example/avatar/alice.png',
      expiresAt: null,
    })
    const wrapper = mountComponent({
      modelValue: 'avatar/alice.png',
      ruleCode: 'avatar',
      variant: 'image',
    })
    await flushPromises()

    expect(requestObjectURL).toHaveBeenCalledWith('avatar/alice.png')
    expect(wrapper.get('.up-media__preview img').attributes('src')).toBe(
      'https://cdn.example/avatar/alice.png',
    )
    expect(wrapper.emitted('preview-change')?.at(-1)).toEqual([
      'https://cdn.example/avatar/alice.png',
    ])
  })

  it('resolves a private object immediately after PUT succeeds', async () => {
    credentialsMock.mockResolvedValue({
      items: [
        {
          fileId: 7,
          uploadUrl: 'https://cos.example/upload',
          objectKey: 'avatar/.admin-storage/v2/private.png',
          method: 'PUT',
          headers: {},
          expiresAt: '2026-09-17T00:10:00Z',
        },
      ],
    })
    objectURLMock.mockResolvedValue({
      url: 'https://cos.example/signed-private',
      expiresAt: '2026-09-17T00:10:00Z',
    })
    const wrapper = mountComponent({ modelValue: '', ruleCode: 'avatar' })

    await chooseFiles(wrapper, [new File(['image'], 'private.png', { type: 'image/png' })])

    expect(objectURLMock).toHaveBeenCalledWith('avatar/.admin-storage/v2/private.png')
  })

  it('ignores an old object URL response after the model changes', async () => {
    const oldResponse = deferred<{ url: string; expiresAt: string | null }>()
    const newResponse = deferred<{ url: string; expiresAt: string | null }>()
    objectURLMock.mockReturnValueOnce(oldResponse.promise).mockReturnValueOnce(newResponse.promise)
    const wrapper = mountComponent({
      modelValue: 'avatar/old.png',
      ruleCode: 'avatar',
      variant: 'image',
    })
    await wrapper.setProps({ modelValue: 'avatar/new.png' })
    newResponse.resolve({ url: 'https://cdn.example/new.png', expiresAt: null })
    await flushPromises()
    oldResponse.resolve({ url: 'https://cdn.example/old.png', expiresAt: null })
    await flushPromises()

    expect(wrapper.get('.up-media__preview img').attributes('src')).toBe(
      'https://cdn.example/new.png',
    )
  })

  it('does not poll expired private URLs and refreshes only after image failure', async () => {
    objectURLMock
      .mockResolvedValueOnce({
        url: 'https://cos.example/first',
        expiresAt: '2020-01-01T00:00:00Z',
      })
      .mockResolvedValueOnce({
        url: 'https://cos.example/refreshed',
        expiresAt: '2030-01-01T00:00:00Z',
      })
    const wrapper = mountComponent({
      modelValue: 'avatar/private.png',
      ruleCode: 'avatar',
      variant: 'image',
    })
    await flushPromises()
    await new Promise((resolve) => setTimeout(resolve, 25))
    expect(objectURLMock).toHaveBeenCalledTimes(1)

    await wrapper.get('.up-media__preview img').trigger('error')
    await flushPromises()
    expect(objectURLMock).toHaveBeenCalledTimes(2)
    expect(wrapper.get('.up-media__preview img').attributes('src')).toBe(
      'https://cos.example/refreshed',
    )
  })

  it('stops retrying a persistently broken image', async () => {
    const wrapper = mountComponent({
      modelValue: 'avatar/broken.png',
      ruleCode: 'avatar',
      variant: 'image',
    })
    await flushPromises()
    await wrapper.get('.up-media__preview img').trigger('error')
    await flushPromises()
    await wrapper.get('.up-media__preview img').trigger('error')
    await flushPromises()
    expect(objectURLMock).toHaveBeenCalledTimes(2)
    expect(wrapper.find('.up-media__preview img').exists()).toBe(false)
  })

  it('resolves a fresh file URL at download time instead of using an expired signature', async () => {
    objectURLMock
      .mockResolvedValueOnce({
        url: 'https://cos.example/expired',
        expiresAt: '2020-01-01T00:00:00Z',
      })
      .mockResolvedValueOnce({
        url: 'https://cos.example/fresh',
        expiresAt: '2030-01-01T00:00:00Z',
      })
    const clicked: string[] = []
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (
      this: HTMLAnchorElement,
    ) {
      clicked.push(this.href)
    })
    try {
      const wrapper = mountComponent({
        modelValue: 'setting/template.xlsx',
        ruleCode: 'setting',
        variant: 'file',
      })
      await flushPromises()
      await wrapper.get('[data-testid="up-media-download"]').trigger('click')
      await flushPromises()
      expect(objectURLMock).toHaveBeenCalledTimes(2)
      expect(clicked).toEqual(['https://cos.example/fresh'])
    } finally {
      click.mockRestore()
    }
  })
})

function mountComponent(props: UpMediaProps) {
  const wrapper = mount(UpMedia, { props, global: { plugins: [ElementPlus, appI18n] } })
  wrappers.push(wrapper)
  return wrapper
}

async function chooseFiles(wrapper: ReturnType<typeof mount>, files: File[]): Promise<void> {
  const input = wrapper.get('[data-testid="up-media-input"]')
  Object.defineProperty(input.element, 'files', { configurable: true, value: files })
  await input.trigger('change')
  await flushPromises()
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}
