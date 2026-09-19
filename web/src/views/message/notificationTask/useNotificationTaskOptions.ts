import { computed, onBeforeUnmount, reactive } from 'vue'

import * as taskApi from '@/api/message/notificationTask'

export type NotificationTaskOptionKind = 'platform' | 'user' | 'role'

interface OptionState {
  items: Array<{ value: number; label: string }>
  nextAfterId: number | null
  error: string
  loading: boolean
  keyword: string | null
  sequence: number
}

const newOptionState = (): OptionState => ({
  items: [],
  nextAfterId: null,
  error: '',
  loading: false,
  keyword: null,
  sequence: 0,
})

export function useNotificationTaskOptions(
  audience: () => taskApi.NotificationAudience,
  intent: () => 'create' | 'update',
  optionFailedMessage: () => string,
) {
  const optionStates = reactive<Record<NotificationTaskOptionKind, OptionState>>({
    platform: newOptionState(),
    user: newOptionState(),
    role: newOptionState(),
  })
  const optionTimers: Record<NotificationTaskOptionKind, number | null> = {
    platform: null,
    user: null,
    role: null,
  }
  const platformOptions = computed(() => optionStates.platform.items)
  const targetKind = computed<NotificationTaskOptionKind>(() => audience())
  const targetState = computed(() => optionStates[targetKind.value])

  async function loadOptions(
    kind: NotificationTaskOptionKind,
    keyword = '',
    append = false,
  ): Promise<void> {
    const state = optionStates[kind]
    const timer = optionTimers[kind]
    if (timer !== null) window.clearTimeout(timer)
    optionTimers[kind] = null
    const current = ++state.sequence
    const normalizedKeyword = keyword.trim()
    const afterId = append && state.nextAfterId !== null ? state.nextAfterId : 0
    if (!append) {
      state.items = []
      state.nextAfterId = null
    }
    state.error = ''
    state.loading = true
    state.keyword = normalizedKeyword
    try {
      const result = await taskApi.listNotificationTaskOptions(intent(), kind, {
        ...(normalizedKeyword === '' ? {} : { keyword: normalizedKeyword }),
        afterId,
        limit: 50,
      })
      if (current !== state.sequence) return
      const options = result.items.map((option) => ({ value: option.id, label: option.label }))
      const merged = append ? [...state.items, ...options] : options
      state.items = Array.from(
        new Map(merged.map((option) => [option.value, option])).values(),
      ).slice(-1000)
      state.nextAfterId = result.nextAfterId
    } catch (error) {
      if (current === state.sequence)
        state.error = error instanceof Error ? error.message : optionFailedMessage()
    } finally {
      if (current === state.sequence) state.loading = false
    }
  }

  function remoteOptions(kind: NotificationTaskOptionKind, keyword: string): void {
    const state = optionStates[kind]
    const normalizedKeyword = keyword.trim()
    if (state.keyword === normalizedKeyword && (state.loading || state.error === '')) return
    const timer = optionTimers[kind]
    if (timer !== null) window.clearTimeout(timer)
    state.sequence += 1
    state.items = []
    state.nextAfterId = null
    state.error = ''
    state.loading = true
    state.keyword = normalizedKeyword
    optionTimers[kind] = window.setTimeout(() => {
      optionTimers[kind] = null
      void loadOptions(kind, normalizedKeyword)
    }, 300)
  }

  function loadMoreOptions(kind: NotificationTaskOptionKind): Promise<void> {
    return loadOptions(kind, optionStates[kind].keyword ?? '', true)
  }

  function ensureSelectedTargets(kind: 'user' | 'role', ids: number[]): void {
    const state = optionStates[kind]
    const known = new Set(state.items.map((option) => option.value))
    state.items = [
      ...ids
        .filter((targetID) => !known.has(targetID))
        .map((targetID) => ({ value: targetID, label: String(targetID) })),
      ...state.items,
    ].slice(0, 1000)
  }

  function resetOptions(): void {
    for (const kind of ['platform', 'user', 'role'] as const) {
      const timer = optionTimers[kind]
      if (timer !== null) window.clearTimeout(timer)
      optionTimers[kind] = null
      const nextSequence = optionStates[kind].sequence + 1
      Object.assign(optionStates[kind], newOptionState())
      optionStates[kind].sequence = nextSequence
    }
  }

  const remotePlatformOptions = (keyword: string): void => remoteOptions('platform', keyword)
  const remoteTargetOptions = (keyword: string): void => remoteOptions(targetKind.value, keyword)
  onBeforeUnmount(resetOptions)

  return {
    ensureSelectedTargets,
    loadMoreOptions,
    loadOptions,
    optionStates,
    platformOptions,
    remotePlatformOptions,
    remoteTargetOptions,
    resetOptions,
    targetKind,
    targetState,
  }
}
