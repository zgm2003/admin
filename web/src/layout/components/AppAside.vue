<script setup lang="ts">
import { Monitor } from '@element-plus/icons-vue'

defineProps<{
  collapsed: boolean
}>()
</script>

<template>
  <aside
    class="app-aside"
    data-testid="app-aside"
    :data-collapsed="String(collapsed)"
    aria-label="主导航"
  >
    <div class="app-aside__brand" aria-label="Admin">
      <span class="app-aside__mark">A</span>
      <span v-show="!collapsed" class="app-aside__name">ADMIN</span>
    </div>

    <el-menu
      class="app-aside__menu"
      router
      :collapse="collapsed"
      :collapse-transition="false"
      default-active="/dashboard"
    >
      <el-menu-item index="/dashboard" data-testid="dashboard-menu-item">
        <el-icon><Monitor /></el-icon>
        <template #title>工作台</template>
      </el-menu-item>
    </el-menu>
  </aside>
</template>
