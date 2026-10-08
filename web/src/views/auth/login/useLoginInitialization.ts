import { computed, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { getLoginConfig, type LoginConfigOption, type LoginType } from '@/api/auth/login'
import { useAuthStore } from '@/store/auth'
import { ProtocolError } from '@/types/http'
import { safeRedirect } from './loginPage'

export function useLoginInitialization(isBusy: () => boolean) {
  const auth = useAuthStore()
  const route = useRoute()
  const router = useRouter()
  const { t } = useI18n()
  const options = ref<LoginConfigOption[]>([])
  const allowRegister = ref(false)
  const activeType = ref<LoginType>()
  const loading = ref(false)
  const retrying = ref(false)
  const retryError = ref('')
  const configFailed = ref(false)
  const bootstrapError = computed(
    () => retryError.value || (auth.status === 'error' ? auth.errorMessage : ''),
  )
  let disposed = false

  onUnmounted(() => {
    disposed = true
  })

  async function loadLoginConfig(): Promise<void> {
    if (loading.value || disposed) return
    loading.value = true
    try {
      const config = await getLoginConfig()
      if (disposed) return
      options.value = config.loginTypes
      allowRegister.value = config.allowRegister
      if (!config.loginTypes.some((option) => option.value === activeType.value)) {
        activeType.value = config.loginTypes[0]?.value
      }
      configFailed.value = false
    } catch {
      if (disposed) return
      options.value = []
      allowRegister.value = false
      configFailed.value = true
    } finally {
      loading.value = false
    }
  }

  async function retryInitialization(): Promise<void> {
    if (loading.value || retrying.value || isBusy() || disposed) return
    const restoreSession = auth.status === 'error' || retryError.value !== ''
    retryError.value = ''
    retrying.value = true
    try {
      if (restoreSession) {
        // Reuse the guard's refresh -> me -> Access flow. A successful public
        // login-config request alone cannot establish the session's state.
        let target = safeRedirect(route.query.redirect)
        const matched = router.resolve(target).matched
        if (matched.length > 0 && matched.every((record) => record.meta.requiresAuth === false)) {
          target = '/dashboard'
        }
        await router.replace(target)
        if (disposed || route.name !== 'login') return
      }
      await loadLoginConfig()
    } catch (error: unknown) {
      if (disposed) return
      retryError.value =
        error instanceof ProtocolError
          ? t('request.protocolError')
          : t('auth.login.bootstrapFailed')
    } finally {
      retrying.value = false
    }
  }

  return {
    options,
    allowRegister,
    activeType,
    loading,
    retrying,
    configFailed,
    bootstrapError,
    loadLoginConfig,
    retryInitialization,
  }
}
