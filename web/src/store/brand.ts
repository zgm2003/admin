import { defineStore } from 'pinia'
import { ref } from 'vue'

import { getBrandSettings, type BrandSettings } from '@/api/system/setting'

export type BrandStatus = 'idle' | 'loading' | 'ready' | 'error'

const emptyBrand = (): BrandSettings => ({ titleZhCN: '', titleEnUS: '', defaultAvatar: '' })

export const useBrandStore = defineStore('brand', () => {
  const settings = ref<BrandSettings>(emptyBrand())
  const status = ref<BrandStatus>('idle')
  let loadPromise: Promise<void> | null = null
  let generation = 0

  function apply(next: BrandSettings): void {
    settings.value = { ...next }
    status.value = 'ready'
  }

  function reset(): void {
    generation += 1
    loadPromise = null
    settings.value = emptyBrand()
    status.value = 'idle'
  }

  function load(): Promise<void> {
    if (status.value === 'ready') return Promise.resolve()
    if (loadPromise !== null) return loadPromise

    status.value = 'loading'
    const requestGeneration = generation
    const pending = getBrandSettings()
      .then((result) => {
        if (generation === requestGeneration) apply(result)
      })
      .catch((error: unknown) => {
        if (generation === requestGeneration) {
          settings.value = emptyBrand()
          status.value = 'error'
        }
        throw error
      })
    loadPromise = pending
    pending
      .finally(() => {
        if (loadPromise === pending) loadPromise = null
      })
      .catch(() => undefined)
    return pending
  }

  return { settings, status, apply, load, reset }
})
