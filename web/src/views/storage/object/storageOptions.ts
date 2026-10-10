import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { getCosConfigOptions, type CosConfigOptions } from '@/api/storage/cosConfigOptions'

type StorageOption = CosConfigOptions['regions'][number]

export function useStorageOptions() {
  const { t, locale } = useI18n()
  const cosRegionOptions = ref<StorageOption[]>([])
  const commonExtensionOptions = ref<StorageOption[]>([])
  const commonMimeTypeOptions = ref<StorageOption[]>([])
  const storageOptionsLoading = ref(false)
  const storageOptionsError = ref('')
  const commonExtensionValues = computed(() =>
    commonExtensionOptions.value.map((item) => item.value),
  )
  const commonMimeTypeValues = computed(() => commonMimeTypeOptions.value.map((item) => item.value))
  let requestSequence = 0

  async function load(): Promise<void> {
    const request = ++requestSequence
    storageOptionsLoading.value = true
    storageOptionsError.value = ''
    try {
      const options = await getCosConfigOptions()
      if (request !== requestSequence) return
      cosRegionOptions.value = options.regions
      commonExtensionOptions.value = options.extensions
      commonMimeTypeOptions.value = options.mimeTypes
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
