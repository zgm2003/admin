<!-- 组件：顶部导航栏 -->
<script setup lang="ts">
import { Menu, Setting } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import type { HeaderBreadcrumb } from '@/layout/breadcrumbs'
import logoUrl from '@/assets/logo.png'
import NotificationBell from '@/components/NotificationBell/index.vue'

defineOptions({ name: 'AppHeader' })

defineProps<{
  breadcrumbs: HeaderBreadcrumb[]
  showBreadcrumb: boolean
  showMenuToggle: boolean
  showBrand?: boolean
  brandName?: string
}>()

const emit = defineEmits<{
  toggleMenu: []
  openSettings: []
}>()

const { t } = useI18n()
</script>

<template>
  <div class="app-header">
    <div class="app-header__leading">
      <div
        v-if="showBrand"
        class="app-header__brand"
        :aria-label="brandName || t('navigation.admin')"
      >
        <img
          class="app-header__brand-logo"
          :src="logoUrl"
          :alt="brandName || t('navigation.admin')"
        />
        <span class="app-header__brand-name">{{ brandName || t('navigation.admin') }}</span>
      </div>
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
        <el-breadcrumb-item
          v-for="breadcrumb in breadcrumbs"
          :key="`${breadcrumb.path ?? 'directory'}:${breadcrumb.i18nKey}`"
          :to="breadcrumb.path ?? undefined"
        >
          {{ t(breadcrumb.i18nKey) }}
        </el-breadcrumb-item>
      </el-breadcrumb>
    </div>

    <div class="app-header__actions">
      <LocaleSwitch />

      <NotificationBell />

      <el-tooltip :content="t('layout.header.settings')">
        <el-button
          data-testid="open-settings"
          text
          :icon="Setting"
          :aria-label="t('layout.header.settings')"
          @click="emit('openSettings')"
        />
      </el-tooltip>
    </div>
  </div>
</template>

<style scoped lang="scss">
.app-header {
  display: flex;
  align-items: center;
  width: 100%;
  height: 100%;
  gap: 16px;
  padding: 0 22px;
  color: var(--admin-text);
  background: var(--admin-surface);

  &__leading,
  &__actions {
    display: flex;
    align-items: center;
  }

  &__leading {
    min-width: 0;
    flex: 1;
    gap: 12px;
  }

  &__brand {
    display: flex;
    align-items: center;
    flex: 0 0 auto;
    gap: 9px;
  }

  &__brand-logo {
    width: auto;
    height: 30px;
    flex: 0 0 auto;
    object-fit: contain;
  }

  &__brand-name {
    max-width: 180px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 14px;
    font-weight: 750;
    letter-spacing: 0.01em;
  }

  &__breadcrumb {
    min-width: 0;
    overflow: hidden;
    font-size: 13px;
    line-height: 1.5;
  }

  &__breadcrumb :deep(.el-breadcrumb__inner) {
    color: var(--admin-text-soft);
    font-weight: 450;
    transition: color var(--admin-motion-fast) ease;
  }

  &__breadcrumb :deep(.el-breadcrumb__inner.is-link:hover) {
    color: var(--el-color-primary);
  }

  &__breadcrumb :deep(.el-breadcrumb__separator) {
    margin: 0 10px;
    color: var(--el-text-color-placeholder);
    font-weight: 400;
  }

  &__breadcrumb :deep(.el-breadcrumb__item:last-child .el-breadcrumb__inner) {
    color: var(--admin-text);
    font-weight: 650;
  }

  &__actions {
    flex: 0 0 auto;
    gap: 6px;
    margin-left: auto;
  }

  &__leading > :deep(.el-button),
  &__actions :deep(.el-button) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 34px;
    height: 34px;
    padding: 8px;
    margin: 0;
    color: var(--admin-text-soft);
    border-radius: var(--admin-radius-sm);
    transition:
      color var(--admin-motion-fast) ease,
      background-color var(--admin-motion-fast) ease,
      transform var(--admin-motion-fast) ease;
  }

  &__leading > :deep(.el-button:hover),
  &__actions :deep(.el-button:hover) {
    color: var(--el-color-primary);
    background: var(--el-color-primary-light-9);
  }

  &__leading > :deep(.el-button:active),
  &__actions :deep(.el-button:active) {
    transform: scale(0.94);
  }

  &__leading > :deep(.el-button:focus-visible),
  &__actions :deep(.el-button:focus-visible) {
    outline: 2px solid var(--el-color-primary);
    outline-offset: 2px;
  }
}

.app-header {
  @media (max-width: 650px) {
    gap: 8px;
    padding: 0 12px;

    &__leading {
      gap: 8px;
    }

    &__brand {
      min-width: 0;
      flex-shrink: 1;
      gap: 7px;
    }

    &__brand-logo {
      height: 26px;
    }

    &__brand-name {
      max-width: 104px;
      font-size: 13px;
    }

    &__actions {
      gap: 2px;
    }

    &__breadcrumb :deep(.el-breadcrumb__separator) {
      margin-inline: 6px;
    }
  }
}

@media (prefers-reduced-motion: reduce) {
  .app-header :deep(*) {
    transition-duration: 0.01ms !important;
  }
}
</style>
