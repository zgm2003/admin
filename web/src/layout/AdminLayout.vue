<script setup lang="ts">
import { Connection, DataAnalysis } from '@element-plus/icons-vue'
</script>

<template>
  <div class="admin-shell">
    <aside class="admin-sidebar">
      <div class="admin-brand" aria-label="Admin">
        <span class="admin-brand__mark">A</span>
        <span class="admin-brand__name">ADMIN</span>
      </div>

      <nav class="admin-nav" aria-label="主导航">
        <RouterLink class="admin-nav__item" to="/dashboard">
          <el-icon><DataAnalysis /></el-icon>
          <span>运行概览</span>
        </RouterLink>
      </nav>

      <div class="admin-sidebar__footer">
        <el-icon><Connection /></el-icon>
        <span>本地环境</span>
      </div>
    </aside>

    <div class="admin-main">
      <RouterView />
    </div>
  </div>
</template>
