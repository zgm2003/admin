import { onScopeDispose, shallowRef, ref, watch } from 'vue'
import { appI18n } from '@/i18n'

// Reusable loading interaction only: no business catalog, parsing or cache.
export function useLocalizedOptions<T>(load: () => Promise<T>, empty: () => T) {
  const options = shallowRef<T>(empty())
  const loading = ref(false)
  const error = ref('')
  let sequence = 0

  async function reload(): Promise<void> {
    const current = ++sequence
    loading.value = true
    error.value = ''
    options.value = empty()
    try {
      const result = await load()
      if (current === sequence) options.value = result
    } catch (cause: unknown) {
      if (current === sequence) {
        options.value = empty()
        error.value = cause instanceof Error ? cause.message : String(cause)
      }
    } finally {
      if (current === sequence) loading.value = false
    }
  }

  watch(appI18n.global.locale, () => void reload(), { immediate: true })
  onScopeDispose(() => {
    sequence += 1
  })
  return { options, loading, error, reload }
}
