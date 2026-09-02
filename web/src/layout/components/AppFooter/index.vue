<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineOptions({ name: 'AppFooter' })

const { t } = useI18n()
</script>

<template>
  <div class="app-footer">{{ t('layout.footer') }}</div>
</template>
