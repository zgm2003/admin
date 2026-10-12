<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { provideGlobalConfig } from 'element-plus/es/components/config-provider/index'
import { useI18n } from 'vue-i18n'

import NetworkStatusNotice from './components/NetworkStatusNotice/index.vue'
import { elementPlusLocaleFor } from './i18n'
import { useUIPreferencesStore } from './store/uiPreferences'

const { locale } = useI18n()
const uiPreferences = useUIPreferencesStore()
const elementLocale = computed(() =>
  elementPlusLocaleFor(locale.value === 'en-US' ? 'en-US' : 'zh-CN'),
)

// Programmatic services render outside this provider's subtree. Keep their global
// configuration reactive as well, including after a previous service has mounted.
const serviceConfig = reactive({ locale: elementLocale.value })
watch(elementLocale, (value) => { serviceConfig.locale = value }, { flush: 'sync' })
provideGlobalConfig(serviceConfig, undefined, true)

if (!uiPreferences.initialized) uiPreferences.initializeSafely()
</script>

<template>
  <el-config-provider :locale="elementLocale">
    <NetworkStatusNotice />
    <RouterView />
  </el-config-provider>
</template>
