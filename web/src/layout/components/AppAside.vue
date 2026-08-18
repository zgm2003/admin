<script setup lang="ts">
import { Monitor } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'

import { useAccessStore } from '../../store/access'
import AccessMenuNode from './AccessMenuNode.vue'

defineProps<{
  collapsed: boolean
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
    >
      <el-menu-item index="/dashboard" data-testid="dashboard-menu-item">
        <el-icon><Monitor /></el-icon>
        <template #title>{{ t('navigation.dashboard') }}</template>
      </el-menu-item>
      <AccessMenuNode v-for="node in access.menuTree" :key="node.code" :node="node" />
    </el-menu>
  </aside>
</template>
