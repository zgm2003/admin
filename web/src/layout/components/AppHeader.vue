<script setup lang="ts">
import { Menu, Setting } from '@element-plus/icons-vue'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { HeaderBreadcrumb } from '../breadcrumbs'
import { LocaleSwitch } from '../../components/LocaleSwitch'
import SettingDrawer from './SettingDrawer.vue'

defineProps<{
  breadcrumbs: HeaderBreadcrumb[]
  showBreadcrumb: boolean
  showMenuToggle: boolean
  contentFullscreen: boolean
}>()

const emit = defineEmits<{
  toggleMenu: []
}>()

const { t } = useI18n()
const settingsOpen = ref(false)

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
		<el-breadcrumb-item v-for="breadcrumb in breadcrumbs" :key="`${breadcrumb.path ?? 'directory'}:${breadcrumb.i18nKey}`" :to="breadcrumb.path ?? undefined">
			{{ t(breadcrumb.i18nKey) }}
        </el-breadcrumb-item>
      </el-breadcrumb>
    </div>

    <div class="app-header__actions">
      <LocaleSwitch />

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
.app-header__actions {
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

@media (max-width: 650px) {
  .app-header {
    gap: 8px;
    padding: 0 12px;
  }
}
</style>
