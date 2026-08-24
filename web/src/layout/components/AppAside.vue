<script setup lang="ts">
import { Monitor } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'

import { useAccessStore } from '../../store/access'
import AccessMenuNode from './AccessMenuNode.vue'

defineProps<{
  collapsed: boolean
  uniqueOpened: boolean
}>()

const { t } = useI18n()
const route = useRoute()
const access = useAccessStore()
</script>

<template>
  <aside
    class="app-aside"
    data-testid="app-aside"
    :data-collapsed="String(collapsed)"
    :aria-label="t('navigation.main')"
  >
    <div class="app-aside__brand" aria-label="Admin">
      <span class="app-aside__mark">A</span>
      <span v-show="!collapsed" class="app-aside__name">Admin</span>
    </div>

    <el-menu
      class="app-aside__menu"
      router
      :collapse="collapsed"
      :collapse-transition="false"
      :default-active="route.path"
      :unique-opened="uniqueOpened"
    >
      <el-menu-item index="/dashboard" data-testid="dashboard-menu-item">
        <el-icon><Monitor /></el-icon>
        <template #title>{{ t('navigation.dashboard') }}</template>
      </el-menu-item>
      <AccessMenuNode v-for="node in access.menuTree" :key="node.code" :node="node" />
    </el-menu>
  </aside>
</template>

<style scoped lang="scss">
.app-aside {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  padding: 14px 12px 12px;
  gap: 10px;
  color: var(--admin-text);
}

.app-aside__brand {
  display: flex;
  align-items: center;
  flex: 0 0 58px;
  min-height: 58px;
  padding: 0 12px;
  gap: 10px;
}

.app-aside__mark {
  display: grid;
  width: 30px;
  height: 30px;
  flex: 0 0 30px;
  place-items: center;
  color: var(--el-color-white);
  background: var(--el-color-primary);
  border-radius: 8px;
  font-size: 14px;
  font-weight: 800;
}

.app-aside__name {
  white-space: nowrap;
  font-size: 14px;
  font-weight: 750;
}

.app-aside__menu {
  --el-menu-active-color: var(--el-color-primary);
  --el-menu-bg-color: transparent;
  --el-menu-hover-bg-color: var(--el-fill-color-light);
  --el-menu-text-color: var(--el-text-color-regular);
  flex: 1;
  min-height: 0;
  width: 100%;
  padding: 4px 0;
  overflow-y: auto;
  border-right: 0;
}

.app-aside__menu :deep(.el-menu-item),
.app-aside__menu :deep(.el-sub-menu__title) {
  height: 44px;
  margin: 3px 0;
  border-radius: 12px;
}

.app-aside__menu :deep(.el-menu-item.is-active) {
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}

.app-aside__menu :deep(.el-sub-menu .el-menu-item) {
  min-width: 0;
  padding-left: 46px;
}

.app-aside__menu :deep(.el-menu--collapse .el-menu-item),
.app-aside__menu :deep(.el-menu--collapse .el-sub-menu__title) {
  justify-content: center;
  padding: 0;
}

.app-aside__menu :deep(.el-menu--collapse .el-menu-item .el-icon),
.app-aside__menu :deep(.el-menu--collapse .el-sub-menu__title .el-icon) {
  margin: 0;
}
</style>
