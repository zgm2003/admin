<script setup lang="ts">
import { Menu, SwitchButton, User } from '@element-plus/icons-vue'

defineProps<{
  username: string
  logoutPending: boolean
}>()

defineEmits<{
  toggleMenu: []
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
