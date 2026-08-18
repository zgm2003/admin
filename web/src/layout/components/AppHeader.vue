<script setup lang="ts">
import { Connection, Menu, Moon, Sunny, SwitchButton, User } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import type { AppLocale } from '../../i18n'
import type { ThemeMode } from '../../utils/theme'

defineProps<{
  locale: AppLocale
  theme: ThemeMode
  username: string
  logoutPending: boolean
}>()

const emit = defineEmits<{
  toggleMenu: []
  toggleTheme: []
  changeLocale: [locale: AppLocale]
  logout: []
}>()

const { t } = useI18n()

function handleLocaleCommand(command: string | number | object): void {
  if (command !== 'zh-CN' && command !== 'en-US') {
    throw new Error(`Unsupported locale command: ${String(command)}`)
  }
  emit('changeLocale', command)
}
</script>

<template>
  <div class="app-header">
    <el-button
      data-testid="toggle-menu"
      :icon="Menu"
      text
      :title="t('layout.header.toggleMenu')"
      :aria-label="t('layout.header.toggleMenu')"
      @click="$emit('toggleMenu')"
    />

    <span class="app-header__location">{{ t('navigation.dashboard') }}</span>

    <div class="app-header__actions">
      <el-dropdown @command="handleLocaleCommand">
        <el-button
          data-testid="locale-switch"
          text
          :icon="Connection"
          :aria-label="t('layout.header.switchLanguage')"
        />
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item
              command="zh-CN"
              data-testid="locale-switch-zh"
              :disabled="locale === 'zh-CN'"
            >
              中文
            </el-dropdown-item>
            <el-dropdown-item
              command="en-US"
              data-testid="locale-switch-en"
              :disabled="locale === 'en-US'"
            >
              English
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>

      <el-tooltip :content="theme === 'dark' ? t('layout.header.switchToLight') : t('layout.header.switchToDark')">
        <el-button
          data-testid="toggle-theme"
          text
          :icon="theme === 'dark' ? Sunny : Moon"
          :aria-label="theme === 'dark' ? t('layout.header.switchToLight') : t('layout.header.switchToDark')"
          @click="$emit('toggleTheme')"
        />
      </el-tooltip>
    </div>

    <div class="app-header__account">
      <el-icon aria-hidden="true"><User /></el-icon>
      <span data-testid="current-username" class="app-header__username">{{ username }}</span>
      <el-button
        data-testid="logout"
        :icon="SwitchButton"
        text
        :loading="logoutPending"
        :disabled="logoutPending"
        :title="t('layout.header.logout')"
        @click="$emit('logout')"
      >
        {{ t('layout.header.logout') }}
      </el-button>
    </div>
  </div>
</template>
