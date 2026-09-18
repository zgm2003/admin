import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { appI18n } from '@/i18n'
import { requestObjectURL, requestUploadCredentials } from '@/api/storage/upload'
import UpMedia from '@/components/UpMedia/index.vue'

vi.mock('@/api/storage/upload', () => ({
  requestUploadCredentials: vi.fn(),
  requestObjectURL: vi.fn(),
}))
const credentialsMock = vi.mocked(requestUploadCredentials)
const objectURLMock = vi.mocked(requestObjectURL)

describe('UpMedia', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true }))
		objectURLMock.mockResolvedValue({ url: 'https://cdn.example/existing.png', expiresAt: null })
  })

  it('uploads one file and emits its object key instead of the upload URL', async () => {
    credentialsMock.mockResolvedValue({
      items: [
        {
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

  it('appends all uploaded object keys when multiple is enabled', async () => {
    credentialsMock.mockResolvedValue({
      items: [
        {
          uploadUrl: 'https://cos.example/a',
          objectKey: 'gallery/a.png',
          method: 'PUT',
          headers: {},
          expiresAt: '2026-08-30T00:10:00Z',
        },
        {
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

  it('renders dedicated avatar actions instead of the generic square uploader', () => {
    const empty = mountComponent({ modelValue: '', ruleCode: 'avatar', variant: 'avatar' })
    expect(empty.find('.avatar-uploader').exists()).toBe(true)
    expect(empty.find('.avatar-uploader .el-upload').exists()).toBe(true)
    expect(empty.find('.avatar-uploader-icon').exists()).toBe(true)

    const filled = mountComponent({
      modelValue: 'avatar/alice.png',
      ruleCode: 'avatar',
      variant: 'avatar',
    })
    expect(filled.find('.avatar-uploader').exists()).toBe(true)
    expect(filled.find('.up-media__avatar-clear').exists()).toBe(true)
    expect(filled.find('.up-media__trigger').exists()).toBe(false)
  })

  it('restores the preview URL for an existing object key after reload', async () => {
		objectURLMock.mockResolvedValue({
			url: 'https://cdn.example/avatar/alice.png',
			expiresAt: null,
		})
    const wrapper = mountComponent({
      modelValue: 'avatar/alice.png',
      ruleCode: 'avatar',
      variant: 'avatar',
    })
    await flushPromises()

		expect(requestObjectURL).toHaveBeenCalledWith('avatar/alice.png')
    expect(wrapper.get('.avatar').attributes('src')).toBe('https://cdn.example/avatar/alice.png')
    expect(wrapper.emitted('preview-change')?.at(-1)).toEqual([
      'https://cdn.example/avatar/alice.png',
    ])
  })

	it('resolves a private object immediately after PUT succeeds', async () => {
		credentialsMock.mockResolvedValue({
			items: [
				{
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
			variant: 'avatar',
		})
		await wrapper.setProps({ modelValue: 'avatar/new.png' })
		newResponse.resolve({ url: 'https://cdn.example/new.png', expiresAt: null })
		await flushPromises()
		oldResponse.resolve({ url: 'https://cdn.example/old.png', expiresAt: null })
		await flushPromises()

		expect(wrapper.get('.avatar').attributes('src')).toBe('https://cdn.example/new.png')
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
			variant: 'avatar',
		})
		await flushPromises()
		await new Promise((resolve) => setTimeout(resolve, 25))
		expect(objectURLMock).toHaveBeenCalledTimes(1)

		await wrapper.get('.avatar').trigger('error')
		await flushPromises()
		expect(objectURLMock).toHaveBeenCalledTimes(2)
		expect(wrapper.get('.avatar').attributes('src')).toBe('https://cos.example/refreshed')
	})
})

function mountComponent(props: {
  modelValue: string | string[]
  ruleCode: string
  multiple?: boolean
  variant?: 'default' | 'avatar'
}) {
  return mount(UpMedia, { props, global: { plugins: [ElementPlus, appI18n] } })
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
