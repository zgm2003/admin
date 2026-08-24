<script setup lang="ts">
import { Connection, Menu, Setting, SwitchButton, User } from '@element-plus/icons-vue'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { AppLocale } from '../../i18n'
import type { HeaderBreadcrumb } from '../breadcrumbs'
import SettingDrawer from './SettingDrawer.vue'

defineProps<{
  locale: AppLocale
  breadcrumbs: HeaderBreadcrumb[]
  showBreadcrumb: boolean
  showMenuToggle: boolean
  contentFullscreen: boolean
  username: string
  logoutPending: boolean
}>()

const emit = defineEmits<{
  toggleMenu: []
  changeLocale: [locale: AppLocale]
  logout: []
}>()

const { t } = useI18n()
const settingsOpen = ref(false)

function handleLocaleCommand(command: string | number | object): void {
  if (command !== 'zh-CN' && command !== 'en-US') {
    throw new Error(`Unsupported locale command: ${String(command)}`)
  }
  emit('changeLocale', command)
}
</script>

<template>
  <div class="app-header">
    <div class="app-header__leading">
      <el-button
        v-if="showMenuToggle"
        data-testid="toggle-menu"
        :icon="Menu"
        text
        :title="t('layout.header.toggleMenu')"
        :aria-label="t('layout.header.toggleMenu')"
        @click="emit('toggleMenu')"
      />

      <el-breadcrumb v-if="showBreadcrumb" class="app-header__breadcrumb" separator="/">
        <el-breadcrumb-item v-for="breadcrumb in breadcrumbs" :key="`${breadcrumb.path ?? 'directory'}:${breadcrumb.titleKey}`" :to="breadcrumb.path ?? undefined">
          {{ t(breadcrumb.titleKey) }}
        </el-breadcrumb-item>
      </el-breadcrumb>
    </div>

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

      <el-tooltip :content="t('layout.header.settings')">
        <el-button
          data-testid="open-settings"
          text
          :icon="Setting"
          :aria-label="t('layout.header.settings')"
          @click="settingsOpen = true"
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
        @click="emit('logout')"
      >
        {{ t('layout.header.logout') }}
      </el-button>
    </div>

    <SettingDrawer v-model="settingsOpen" :content-fullscreen="contentFullscreen" />
  </div>
</template>

<style scoped lang="scss">
.app-header {
  display: flex;
  align-items: center;
  width: 100%;
  height: 100%;
  gap: 14px;
  padding: 0 18px;
  color: var(--admin-text);
}

.app-header__leading,
.app-header__actions,
.app-header__account {
  display: flex;
  align-items: center;
}

.app-header__leading {
  min-width: 0;
  gap: 12px;
}

.app-header__breadcrumb {
  min-width: 0;
  overflow: hidden;
}

.app-header__breadcrumb :deep(.el-breadcrumb__item:last-child .el-breadcrumb__inner) {
  color: var(--el-text-color-primary);
  font-weight: 700;
}

.app-header__actions {
  gap: 4px;
  margin-left: auto;
}

.app-header__account {
  flex: 0 0 auto;
  gap: 8px;
  white-space: nowrap;
}

.app-header__username {
  max-width: 180px;
  overflow: hidden;
  color: var(--el-text-color-regular);
  text-overflow: ellipsis;
}

@media (max-width: 650px) {
  .app-header {
    gap: 8px;
    padding: 0 12px;
  }

  .app-header__username {
    max-width: 90px;
  }

  .app-header__account .el-button {
    padding-right: 0;
    padding-left: 0;
    font-size: 0;
  }

  .app-header__account .el-button :deep(.el-icon) {
    margin: 0;
    font-size: 16px;
  }
}
</style>
