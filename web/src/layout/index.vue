<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { logout } from '../api/auth'
import { readLocale, setLocale, type AppLocale } from '../i18n'
import { useAuthStore } from '../store/auth'
import { ProtocolError } from '../types/http'
import { readTheme, toggleTheme, type ThemeMode } from '../utils/theme'
import AppAside from './components/AppAside.vue'
import AppFooter from './components/AppFooter.vue'
import AppHeader from './components/AppHeader.vue'
import RouteTabs from './components/RouteTabs.vue'

const mobileBreakpoint = 840
const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const auth = useAuthStore()
const collapsed = ref(false)
const mobileMenuOpen = ref(false)
const isMobile = ref(window.innerWidth <= mobileBreakpoint)
const logoutPending = ref(false)
const theme = ref<ThemeMode>(readTheme())
const locale = ref<AppLocale>(readLocale())
const refreshKey = ref(0)
const contentFullscreen = ref(false)

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

function handleToggleTheme(): void {
  theme.value = toggleTheme(theme.value)
}

function handleLocaleChange(nextLocale: AppLocale): void {
  locale.value = nextLocale
  setLocale(nextLocale)
}

function handleRefresh(): void {
  refreshKey.value += 1
}

function handleToggleFullscreen(): void {
  contentFullscreen.value = !contentFullscreen.value
}

async function handleLogout(): Promise<void> {
  if (logoutPending.value) return
  logoutPending.value = true
  try {
    await logout()
    auth.setAnonymous()
    await router.replace({ name: 'login' })
  } catch (error: unknown) {
    const message = error instanceof ProtocolError
      ? t('request.protocolError')
      : error instanceof Error && error.message !== '' ? error.message : t('auth.logoutFailed')
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
    <el-aside v-if="!contentFullscreen" class="admin-layout__aside" :width="asideWidth">
      <AppAside :collapsed="collapsed" />
    </el-aside>

    <el-container class="admin-layout__workspace">
      <el-header v-if="!contentFullscreen" class="admin-layout__header" height="56px">
        <AppHeader
          :locale="locale"
          :theme="theme"
          :username="username"
          :logout-pending="logoutPending"
          @toggle-menu="toggleMenu"
          @toggle-theme="handleToggleTheme"
          @change-locale="handleLocaleChange"
          @logout="handleLogout"
        />
      </el-header>
      <div class="admin-layout__tabs admin-layout__horizontal-scroll">
        <RouteTabs @refresh="handleRefresh" @toggle-fullscreen="handleToggleFullscreen" />
      </div>
      <el-main class="admin-layout__main admin-layout__scroll-owner">
        <RouterView :key="`${route.fullPath}::${refreshKey}`" />
      </el-main>
      <el-footer v-if="!contentFullscreen" class="admin-layout__footer" height="40px">
        <AppFooter />
      </el-footer>
    </el-container>

    <el-drawer
      v-if="!contentFullscreen"
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
