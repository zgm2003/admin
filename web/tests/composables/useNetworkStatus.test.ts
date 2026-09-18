import { mount } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { useNetworkStatus } from '@/composables/useNetworkStatus'

describe('useNetworkStatus', () => {
  beforeEach(() => setNavigatorOnline(true))
  afterEach(() => setNavigatorOnline(true))

  it('reflects the initial offline state and records when it started', () => {
    setNavigatorOnline(false)
    const { status, wrapper } = mountNetworkStatus()

    expect(status.isOnline.value).toBe(false)
    expect(status.isOffline.value).toBe(true)
    expect(status.lastOfflineAt.value).toBeInstanceOf(Date)

    wrapper.unmount()
  })

  it('tracks browser connectivity events and stops after unmount', async () => {
    const { status, wrapper } = mountNetworkStatus()

    window.dispatchEvent(new Event('offline'))
    await nextTick()
    const firstOfflineAt = status.lastOfflineAt.value
    expect(status.isOffline.value).toBe(true)
    expect(firstOfflineAt).toBeInstanceOf(Date)

    window.dispatchEvent(new Event('online'))
    await nextTick()
    expect(status.isOnline.value).toBe(true)
    expect(status.lastOfflineAt.value).toBe(firstOfflineAt)

    wrapper.unmount()
    window.dispatchEvent(new Event('offline'))
    await nextTick()
    expect(status.isOnline.value).toBe(true)
  })
})

function mountNetworkStatus() {
  let status!: ReturnType<typeof useNetworkStatus>
  const wrapper = mount(
    defineComponent({
      setup() {
        status = useNetworkStatus()
        return () => null
      },
    }),
  )
  return { status, wrapper }
}

function setNavigatorOnline(value: boolean): void {
  Object.defineProperty(window.navigator, 'onLine', {
    configurable: true,
    value,
  })
}
