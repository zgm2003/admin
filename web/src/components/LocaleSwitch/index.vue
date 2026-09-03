<script setup lang="ts">
import { Languages } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { setLocale } from '@/i18n'

defineOptions({ name: 'LocaleSwitch' })

withDefaults(
  defineProps<{
    testId?: string
  }>(),
  {
    testId: 'locale-switch',
  },
)

const { locale, t } = useI18n()

function handleCommand(command: string | number | object): void {
  if (command !== 'zh-CN' && command !== 'en-US') {
    throw new Error(`Unsupported locale command: ${String(command)}`)
  }
  setLocale(command)
}
</script>
<template>
  <el-dropdown @command="handleCommand">
    <el-button
      :data-testid="testId"
      text
      :icon="Languages"
      :aria-label="t('layout.header.switchLanguage')"
    />
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item
          command="zh-CN"
          :data-testid="`${testId}-zh`"
          :disabled="locale === 'zh-CN'"
        >
          中文
        </el-dropdown-item>
        <el-dropdown-item
          command="en-US"
          :data-testid="`${testId}-en`"
          :disabled="locale === 'en-US'"
        >
          English
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>
