import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { DictionaryOptions } from '@/api/system/dictionary'
import { useSystemDictionaryStore } from '@/store/systemDictionary'

type DictionaryOption = DictionaryOptions[string][number]

const storageDictionaryCodes = [
  'storage.cos.region',
  'storage.file.extension',
  'storage.mime.type',
] as const

export function useStorageDictionaries() {
  const { t, locale } = useI18n()
  const dictionaries = useSystemDictionaryStore()
  const cosRegionOptions = ref<DictionaryOption[]>([])
  const commonExtensionOptions = ref<DictionaryOption[]>([])
  const commonMimeTypeOptions = ref<DictionaryOption[]>([])
  const storageOptionsLoading = ref(false)
  const storageOptionsError = ref('')
  const commonExtensionValues = computed(() =>
    commonExtensionOptions.value.map((item) => item.value),
  )
  const commonMimeTypeValues = computed(() =>
    commonMimeTypeOptions.value.map((item) => item.value),
  )
  let requestSequence = 0

  function dictionaryOptions(code: (typeof storageDictionaryCodes)[number]): DictionaryOption[] {
    const options = dictionaries.options(code).value
    if (options === undefined) throw new Error(`${code} dictionary is not ready`)
    return options.map((item) => ({ ...item }))
  }

  async function load(): Promise<void> {
    const request = ++requestSequence
    storageOptionsLoading.value = true
    storageOptionsError.value = ''
    try {
      await dictionaries.load([...storageDictionaryCodes])
      if (request !== requestSequence) return
      cosRegionOptions.value = dictionaryOptions('storage.cos.region')
      commonExtensionOptions.value = dictionaryOptions('storage.file.extension')
      commonMimeTypeOptions.value = dictionaryOptions('storage.mime.type')
    } catch {
      if (request !== requestSequence) return
      cosRegionOptions.value = []
      commonExtensionOptions.value = []
      commonMimeTypeOptions.value = []
      storageOptionsError.value = t('storage.optionsLoadFailed')
    } finally {
      if (request === requestSequence) storageOptionsLoading.value = false
    }
  }

  watch(locale, () => void load(), { immediate: true })

  return {
    commonExtensionOptions,
    commonExtensionValues,
    commonMimeTypeOptions,
    commonMimeTypeValues,
    cosRegionOptions,
    storageOptionsError,
    storageOptionsLoading,
  }
}
