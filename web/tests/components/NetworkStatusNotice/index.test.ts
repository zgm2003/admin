import { mount } from '@vue/test-utils'
import { nextTick, shallowRef } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import NetworkStatusNotice from '@/components/NetworkStatusNotice/index.vue'
import { appI18n, setLocale } from '@/i18n'

const network = vi.hoisted(() => ({
  isOffline: null as ReturnType<typeof shallowRef<boolean>> | null,
  lastOfflineAt: null as ReturnType<typeof shallowRef<Date | null>> | null,
  refreshPage: vi.fn(),
}))

vi.mock('@/composables/useNetworkStatus', async () => {
  const { shallowRef: createRef } = await import('vue')
  network.isOffline = createRef(false)
  network.lastOfflineAt = createRef<Date | null>(null)
  return {
    useNetworkStatus: () => ({
      isOffline: network.isOffline,
      lastOfflineAt: network.lastOfflineAt,
      refreshPage: network.refreshPage,
    }),
  }
})

describe('NetworkStatusNotice', () => {
  beforeEach(() => {
    setLocale('zh-CN')
    network.refreshPage.mockReset()
    network.isOffline!.value = false
    network.lastOfflineAt!.value = null
  })

  it('stays hidden while online', () => {
    const wrapper = mountNotice()

    expect(wrapper.find('[data-testid="network-status-notice"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('announces an offline state and reloads on request', async () => {
    const wrapper = mountNotice()
    network.lastOfflineAt!.value = new Date(2026, 8, 18, 8, 30)
    network.isOffline!.value = true
    await nextTick()

    const notice = wrapper.get('[data-testid="network-status-notice"]')
    expect(notice.attributes('role')).toBe('status')
    expect(notice.attributes('aria-live')).toBe('assertive')
    expect(notice.text()).toContain('网络连接已断开')
    expect(notice.text()).toContain('08:30')

    const refresh = wrapper.get('[data-testid="network-status-refresh"]')
    expect(refresh.attributes('aria-label')).toBe('刷新页面')
    await refresh.trigger('click')
    expect(network.refreshPage).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })
})

function mountNotice() {
  return mount(NetworkStatusNotice, {
    global: {
      plugins: [appI18n],
      stubs: { transition: false },
    },
  })
}
