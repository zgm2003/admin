import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { afterEach, expect, it, vi } from 'vitest'
import { setLocale } from '@/i18n'
import { useLocalizedOptions } from '@/composables/useLocalizedOptions'

afterEach(() => setLocale('zh-CN'))

it('loads backend labels, preserves numeric values and ignores stale locale results', async () => {
  setLocale('zh-CN')
  let resolveFirst!: (value: Array<{ value: number; label: string }>) => void
  const first = new Promise<Array<{ value: number; label: string }>>((resolve) => {
    resolveFirst = resolve
  })
  const load = vi
    .fn()
    .mockReturnValueOnce(first)
    .mockResolvedValueOnce([{ value: 3, label: 'Other' }])
  const wrapper = mount(
    defineComponent({
      setup() {
        return useLocalizedOptions(load, () => [] as Array<{ value: number; label: string }>)
      },
      template: '<div>{{ options }}</div>',
    }),
  )
  setLocale('en-US')
  await flushPromises()
  expect(wrapper.vm.options).toEqual([{ value: 3, label: 'Other' }])
  resolveFirst([{ value: 0, label: '未知' }])
  await flushPromises()
  expect(wrapper.vm.options).toEqual([{ value: 3, label: 'Other' }])
  wrapper.unmount()
})

it('clears options on failure and allows retry without caching fake data', async () => {
  const load = vi.fn().mockRejectedValueOnce(new Error('unavailable')).mockResolvedValueOnce([])
  const wrapper = mount(
    defineComponent({
      setup() {
        return useLocalizedOptions(load, () => [] as string[])
      },
      template: '<div>{{ error }}</div>',
    }),
  )
  await flushPromises()
  expect(wrapper.vm.options).toEqual([])
  expect(wrapper.vm.error).toBe('unavailable')
  await wrapper.vm.reload()
  expect(wrapper.vm.error).toBe('')
  expect(load).toHaveBeenCalledTimes(2)
  wrapper.unmount()
})
