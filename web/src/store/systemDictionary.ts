import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { getDictionaryOptions, type DictionaryOptions } from '@/api/system/dictionary'
import { appI18n, type AppLocale } from '@/i18n'

type DictionaryOption = DictionaryOptions[string][number]
type DictionaryState =
  | { status: 'idle'; error: null }
  | { status: 'loading'; error: null }
  | { status: 'ready'; options: DictionaryOption[]; error: null }
  | { status: 'error'; error: unknown }

export const useSystemDictionaryStore = defineStore('systemDictionary', () => {
  const entries = ref<Record<AppLocale, Record<string, DictionaryState>>>({
    'zh-CN': {},
    'en-US': {},
  })
  const pending = new Map<string, Promise<void>>()
  let generation = 0

  const activeLocale = computed<AppLocale>(() => appI18n.global.locale.value)
  const loading = computed(() =>
    Object.values(entries.value[activeLocale.value]).some((entry) => entry.status === 'loading'),
  )
  const error = computed<unknown>(() => {
    const failed = Object.values(entries.value[activeLocale.value]).find(
      (entry) => entry.status === 'error',
    )
    return failed?.error ?? null
  })
  const values = computed<Record<string, DictionaryOption[]>>(() => {
    const result: Record<string, DictionaryOption[]> = {}
    for (const [code, entry] of Object.entries(entries.value[activeLocale.value])) {
      if (entry.status === 'ready') result[code] = entry.options
    }
    return result
  })

  async function load(codes: string[]): Promise<void> {
    const locale = activeLocale.value
    const requestedCodes = [...new Set(codes)]
    const waitFor = new Set<Promise<void>>()
    const missing: string[] = []

    for (const code of requestedCodes) {
      const key = pendingKey(locale, code)
      const inFlight = pending.get(key)
      if (inFlight !== undefined) {
        waitFor.add(inFlight)
      } else if (entries.value[locale][code]?.status !== 'ready') {
        missing.push(code)
      }
    }

    if (missing.length > 0) {
      const requestGeneration = generation
      for (const code of missing) entries.value[locale][code] = { status: 'loading', error: null }

      const requestPromise = getDictionaryOptions(missing)
        .then((result) => {
          if (requestGeneration !== generation) return
          for (const code of missing) {
            entries.value[locale][code] = {
              status: 'ready',
              options: result[code],
              error: null,
            }
          }
        })
        .catch((cause: unknown) => {
          if (requestGeneration === generation) {
            for (const code of missing)
              entries.value[locale][code] = { status: 'error', error: cause }
          }
          throw cause
        })
        .finally(() => {
          for (const code of missing) {
            const key = pendingKey(locale, code)
            if (pending.get(key) === requestPromise) pending.delete(key)
          }
        })
      for (const code of missing) pending.set(pendingKey(locale, code), requestPromise)
      waitFor.add(requestPromise)
    }

    await Promise.all(waitFor)
  }

  function state(code: string) {
    return computed<DictionaryState>(() =>
      entries.value[activeLocale.value][code] === undefined
        ? { status: 'idle', error: null }
        : entries.value[activeLocale.value][code],
    )
  }

  function options(code: string) {
    return computed<DictionaryOption[] | undefined>(() => {
      const entry = entries.value[activeLocale.value][code]
      return entry?.status === 'ready' ? entry.options : undefined
    })
  }

  function reset() {
    generation += 1
    entries.value = { 'zh-CN': {}, 'en-US': {} }
    pending.clear()
  }

  return { values, loading, error, load, state, options, reset }
})

function pendingKey(locale: AppLocale, code: string): string {
  return `${locale}\u0000${code}`
}
