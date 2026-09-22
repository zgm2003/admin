import { defineComponent, nextTick, ref, type Ref } from 'vue'
import { mount, type VueWrapper } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import { useMessageAggregateTabs } from '@/composables/useMessageAggregateTabs'

type TabName = 'config' | 'templates'

describe('useMessageAggregateTabs', () => {
  it('loads the active tab when list permission becomes ready asynchronously', async () => {
    const activeTab = ref<TabName>('config')
    const canList = ref(false)
    const loaders = {
      config: vi.fn(async () => undefined),
      templates: vi.fn(async () => undefined),
    }
    const harness = mount(
      defineComponent({
        setup() {
          useMessageAggregateTabs({
            activeTab,
            canList,
            loaders,
            errorMessage: () => 'load failed',
          })
          return () => null
        },
      }),
    )

    await nextTick()
    expect(loaders.config).not.toHaveBeenCalled()

    canList.value = true
    await nextTick()
    expect(loaders.config).toHaveBeenCalledOnce()

    harness.unmount()
  })

  it('does not commit an older tab response after the active tab changes', async () => {
    const activeTab = ref<TabName>('config')
    const value = ref('')
    const oldResponse = deferred<void>()
    const loaders = {
      config: vi.fn(async ({ isCurrent }: { isCurrent: () => boolean }) => {
        await oldResponse.promise
        if (isCurrent()) value.value = 'old'
      }),
      templates: vi.fn(async ({ isCurrent }: { isCurrent: () => boolean }) => {
        if (isCurrent()) value.value = 'new'
      }),
    }
    const harness = mountHarness(activeTab, loaders)

    await nextTick()
    activeTab.value = 'templates'
    await nextTick()
    expect(value.value).toBe('new')

    oldResponse.resolve()
    await oldResponse.promise
    await nextTick()
    expect(value.value).toBe('new')
    expect(loaders.config).toHaveBeenCalledOnce()
    expect(loaders.templates).toHaveBeenCalledOnce()

    harness.unmount()
  })

  it('marks an in-flight loader stale when the host component unmounts', async () => {
    const activeTab = ref<TabName>('config')
    const oldResponse = deferred<void>()
    let currentAfterUnmount = true
    const harness = mountHarness(activeTab, {
      config: async ({ isCurrent }: { isCurrent: () => boolean }) => {
        await oldResponse.promise
        currentAfterUnmount = isCurrent()
      },
      templates: async () => undefined,
    })

    await nextTick()
    harness.unmount()
    oldResponse.resolve()
    await oldResponse.promise
    expect(currentAfterUnmount).toBe(false)
  })

  it('marks an in-flight loader stale when list permission is revoked', async () => {
    const activeTab = ref<TabName>('config')
    const canList = ref(true)
    const response = deferred<void>()
    let currentAfterRevocation = true
    const harness = mount(
      defineComponent({
        setup() {
          useMessageAggregateTabs({
            activeTab,
            canList,
            loaders: {
              config: async ({ isCurrent }) => {
                await response.promise
                currentAfterRevocation = isCurrent()
              },
              templates: async () => undefined,
            },
            errorMessage: () => 'load failed',
          })
          return () => null
        },
      }),
    )

    await nextTick()
    canList.value = false
    await nextTick()
    response.resolve()
    await response.promise
    expect(currentAfterRevocation).toBe(false)

    harness.unmount()
  })
})

function mountHarness(
  activeTab: Ref<TabName>,
  loaders: Record<TabName, (context: { isCurrent: () => boolean }) => Promise<void>>,
): VueWrapper {
  return mount(
    defineComponent({
      setup() {
        useMessageAggregateTabs({
          activeTab,
          canList: ref(true),
          loaders,
          errorMessage: () => 'load failed',
        })
        return () => null
      },
    }),
  )
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}
