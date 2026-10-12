<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import NetworkStatusNotice from './components/NetworkStatusNotice/index.vue'
import { elementPlusLocaleFor } from './i18n'
import { useUIPreferencesStore } from './store/uiPreferences'

const { locale } = useI18n()
const uiPreferences = useUIPreferencesStore()
const elementLocale = computed(() =>
  elementPlusLocaleFor(locale.value === 'en-US' ? 'en-US' : 'zh-CN'),
)

if (!uiPreferences.initialized) uiPreferences.initializeSafely()
</script>

<template>
  <el-config-provider :locale="elementLocale">
    <NetworkStatusNotice />
    <RouterView />
  </el-config-provider>
</template>
