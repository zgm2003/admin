<script setup lang="ts">
import { Menu, Moon, Sunny, SwitchButton, User } from '@element-plus/icons-vue'

import type { ThemeMode } from '../../utils/theme'

defineProps<{
  theme: ThemeMode
  username: string
  logoutPending: boolean
}>()

defineEmits<{
  toggleMenu: []
  toggleTheme: []
  logout: []
}>()
</script>

<template>
  <div class="app-header">
    <el-button
      data-testid="toggle-menu"
      :icon="Menu"
      text
      title="切换菜单"
      aria-label="切换菜单"
      @click="$emit('toggleMenu')"
    />

    <span class="app-header__location">工作台</span>

    <div class="app-header__actions">
      <el-tooltip :content="theme === 'dark' ? '切换为浅色主题' : '切换为深色主题'">
        <el-button
          data-testid="toggle-theme"
          text
          :icon="theme === 'dark' ? Sunny : Moon"
          :aria-label="theme === 'dark' ? '切换为浅色主题' : '切换为深色主题'"
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
        title="退出登录"
        @click="$emit('logout')"
      >
        退出
      </el-button>
    </div>
  </div>
</template>
