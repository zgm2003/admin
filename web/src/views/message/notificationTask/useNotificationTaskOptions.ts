import { computed, onBeforeUnmount, reactive } from 'vue'

import * as taskApi from '@/api/message/notificationTask'

export type NotificationTaskOptionKind = 'platform' | 'user' | 'role'

interface OptionState {
  items: taskApi.NotificationTaskOption[]
  nextAfterId: number | null
  error: string
  loading: boolean
  sequence: number
}

const newOptionState = (): OptionState => ({
  items: [],
  nextAfterId: 0,
  error: '',
  loading: false,
  sequence: 0,
})

export function useNotificationTaskOptions(
  audience: () => taskApi.NotificationAudience,
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
    const current = ++state.sequence
    const afterId = append && state.nextAfterId !== null ? state.nextAfterId : 0
    state.error = ''
    state.loading = true
    try {
      const result = await taskApi.listNotificationTaskOptions(kind, {
        ...(keyword.trim() === '' ? {} : { keyword: keyword.trim() }),
        afterId,
        limit: 50,
      })
      if (current !== state.sequence) return
      const merged = append ? [...state.items, ...result.items] : result.items
      state.items = Array.from(new Map(merged.map((option) => [option.id, option])).values()).slice(
        -1000,
      )
      state.nextAfterId = result.nextAfterId
    } catch (error) {
      if (current === state.sequence)
        state.error = error instanceof Error ? error.message : optionFailedMessage()
    } finally {
      if (current === state.sequence) state.loading = false
    }
  }

  function remoteOptions(kind: NotificationTaskOptionKind, keyword: string): void {
    const timer = optionTimers[kind]
    if (timer !== null) window.clearTimeout(timer)
    optionTimers[kind] = window.setTimeout(() => {
      optionTimers[kind] = null
      void loadOptions(kind, keyword)
    }, 300)
  }

  function ensureSelectedTargets(kind: 'user' | 'role', ids: number[]): void {
    const state = optionStates[kind]
    const known = new Set(state.items.map((option) => option.id))
    state.items = [
      ...ids
        .filter((targetID) => !known.has(targetID))
        .map((targetID) => ({ id: targetID, label: String(targetID) })),
      ...state.items,
    ].slice(0, 1000)
  }

  function resetOptions(): void {
    for (const kind of ['platform', 'user', 'role'] as const) {
      const timer = optionTimers[kind]
      if (timer !== null) window.clearTimeout(timer)
      optionTimers[kind] = null
      Object.assign(optionStates[kind], newOptionState())
    }
  }

  const remotePlatformOptions = (keyword: string): void => remoteOptions('platform', keyword)
  const remoteTargetOptions = (keyword: string): void => remoteOptions(targetKind.value, keyword)
  onBeforeUnmount(resetOptions)

  return {
    ensureSelectedTargets,
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
