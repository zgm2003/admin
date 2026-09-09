<script setup lang="ts">
import { ArrowUp, Monitor, SwitchButton, User } from '@element-plus/icons-vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { requestObjectURL } from '@/api/storage/upload'
import logoUrl from '@/assets/logo.png'
import { usePermissionStore } from '@/store/permission'
import PermissionMenuNode from '@/layout/components/PermissionMenuNode/index.vue'

defineOptions({ name: 'AppAside' })

const props = withDefaults(
  defineProps<{
    collapsed: boolean
    uniqueOpened: boolean
    showBrand?: boolean
    username?: string
    email?: string
    avatar?: string
    logoutPending?: boolean
  }>(),
  {
    showBrand: true,
    username: '',
    email: '',
    avatar: '',
    logoutPending: false,
  },
)

const emit = defineEmits<{
  logout: []
}>()

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const access = usePermissionStore()
const avatarText = computed(() => props.username.slice(0, 1).toUpperCase() || 'A')
const avatarURL = ref('')
const canOpenProfile = computed(() => access.hasPermission('user:profile:view'))
let avatarRequestID = 0

async function hydrateAvatar(objectKey: string): Promise<void> {
  const requestID = ++avatarRequestID
  avatarURL.value = ''
  if (!objectKey) return
  try {
    const result = await requestObjectURL('avatar', objectKey)
    if (requestID === avatarRequestID) avatarURL.value = result.url
  } catch {
    if (requestID === avatarRequestID) avatarURL.value = ''
  }
}

function handleAvatarError(): void {
  avatarRequestID += 1
  avatarURL.value = ''
}

watch(
  () => props.avatar,
  (objectKey) => {
    void hydrateAvatar(objectKey)
  },
  { immediate: true },
)

function handleAccountCommand(command: string | number | object): void {
  if (command === 'logout') {
    emit('logout')
    return
  }
  if (command === 'profile') {
    void router.push('/user/profile')
    return
  }
  throw new Error(`Unsupported account command: ${String(command)}`)
}
</script>

<template>
  <aside
    class="app-aside"
    data-testid="app-aside"
    :data-collapsed="String(collapsed)"
    :aria-label="t('navigation.main')"
  >
    <div v-if="showBrand" class="app-aside__brand" :aria-label="t('navigation.admin')">
      <img class="app-aside__logo" :src="logoUrl" :alt="t('navigation.admin')" />
      <span class="app-aside__name">{{ t('navigation.admin') }}</span>
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
      <PermissionMenuNode v-for="node in access.menuTree" :key="node.code" :node="node" />
    </el-menu>

    <div class="app-aside__account">
      <el-dropdown trigger="click" placement="top-start" @command="handleAccountCommand">
        <button
          type="button"
          class="app-aside__account-trigger"
          data-testid="aside-account-menu"
          :title="t('layout.user.title')"
          :aria-label="t('layout.user.title')"
        >
          <el-avatar
            class="app-aside__avatar"
            :size="34"
            :src="avatarURL"
            @error="handleAvatarError"
            >{{ avatarText }}</el-avatar
          >
          <span class="app-aside__account-copy">
            <strong data-testid="aside-account-name">{{ username }}</strong>
            <small>{{ email }}</small>
          </span>
          <el-icon class="app-aside__account-arrow" aria-hidden="true"><ArrowUp /></el-icon>
        </button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item
              v-if="canOpenProfile"
              data-testid="aside-account-profile"
              :icon="User"
              command="profile"
            >
              {{ t('layout.user.profile') }}
            </el-dropdown-item>
            <el-dropdown-item
              data-testid="aside-account-logout"
              command="logout"
              :icon="SwitchButton"
              divided
              :disabled="logoutPending"
            >
              {{ t('layout.header.logout') }}
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
  </aside>
</template>

<style scoped src="./AppAside.css"></style>
