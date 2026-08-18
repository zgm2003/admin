<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'

import { logout } from '../api/auth'
import { useAuthStore } from '../store/auth'
import AppAside from './components/AppAside.vue'
import AppFooter from './components/AppFooter.vue'
import AppHeader from './components/AppHeader.vue'

const mobileBreakpoint = 840
const router = useRouter()
const auth = useAuthStore()
const collapsed = ref(false)
const mobileMenuOpen = ref(false)
const isMobile = ref(window.innerWidth <= mobileBreakpoint)
const logoutPending = ref(false)

const asideWidth = computed(() => collapsed.value ? '64px' : '224px')
const username = computed(() => auth.user === null ? '' : auth.user.username)

function updateViewport(): void {
  isMobile.value = window.innerWidth <= mobileBreakpoint
  if (!isMobile.value) mobileMenuOpen.value = false
}

function toggleMenu(): void {
  if (isMobile.value) {
    mobileMenuOpen.value = true
    return
  }
  collapsed.value = !collapsed.value
}

async function handleLogout(): Promise<void> {
  if (logoutPending.value) return
  logoutPending.value = true
  try {
    await logout()
    auth.setAnonymous()
    await router.replace({ name: 'login' })
  } catch (error: unknown) {
    const message = error instanceof Error && error.message !== '' ? error.message : '退出登录失败'
    ElMessage.error(message)
  } finally {
    logoutPending.value = false
  }
}

onMounted(() => window.addEventListener('resize', updateViewport))
onBeforeUnmount(() => window.removeEventListener('resize', updateViewport))
</script>

<template>
  <el-container class="admin-layout">
    <el-aside class="admin-layout__aside" :width="asideWidth">
      <AppAside :collapsed="collapsed" />
    </el-aside>

    <el-container class="admin-layout__workspace">
      <el-header class="admin-layout__header" height="56px">
        <AppHeader
          :username="username"
          :logout-pending="logoutPending"
          @toggle-menu="toggleMenu"
          @logout="handleLogout"
        />
      </el-header>
      <el-main class="admin-layout__main">
        <RouterView />
      </el-main>
      <el-footer class="admin-layout__footer" height="40px">
        <AppFooter />
      </el-footer>
    </el-container>

    <el-drawer
      v-model="mobileMenuOpen"
      class="admin-layout__drawer"
      direction="ltr"
      :with-header="false"
      size="240px"
    >
      <AppAside :collapsed="false" />
    </el-drawer>
  </el-container>
</template>
