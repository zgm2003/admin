import { onUnmounted, shallowReactive, watch, type Ref } from 'vue'

export interface MessageTabLoadContext {
  isCurrent: () => boolean
}

export type MessageTabLoader = (context: MessageTabLoadContext) => Promise<void>

export interface UseMessageAggregateTabsOptions<T extends string> {
  activeTab: Ref<T>
  canList: Ref<boolean>
  loaders: Record<T, MessageTabLoader>
  errorMessage: (error: unknown) => string
}

export function useMessageAggregateTabs<T extends string>({
  activeTab,
  canList,
  loaders,
  errorMessage,
}: UseMessageAggregateTabsOptions<T>) {
  const loadingState = shallowReactive<Record<string, boolean>>({})
  const errorState = shallowReactive<Record<string, string>>({})
  let requestSequence = 0
  let mounted = true

  for (const tab of Object.keys(loaders) as T[]) {
    loadingState[tab] = false
    errorState[tab] = ''
  }

  function isCurrent(tab: T, sequence: number): boolean {
    return mounted && sequence === requestSequence && activeTab.value === tab
  }

  async function load(tab: T = activeTab.value): Promise<void> {
    if (!canList.value) return
    const sequence = ++requestSequence
    loadingState[tab] = true
    errorState[tab] = ''
    const current = () => isCurrent(tab, sequence)
    try {
      await loaders[tab]({ isCurrent: current })
    } catch (error: unknown) {
      if (current()) errorState[tab] = errorMessage(error)
    } finally {
      if (current()) loadingState[tab] = false
    }
  }

  watch(activeTab, (tab) => void load(tab), { immediate: true })
  onUnmounted(() => {
    mounted = false
    requestSequence += 1
  })

  return {
    loading: loadingState as Record<T, boolean>,
    errors: errorState as Record<T, string>,
    load,
  }
}
