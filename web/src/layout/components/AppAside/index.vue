<script setup lang="ts">
import { ArrowUp, Monitor, SwitchButton, User } from '@element-plus/icons-vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { requestObjectURL } from '@/api/storage/upload'
import { usePermissionStore } from '@/store/permission'
import PermissionMenuNode from '@/layout/components/PermissionMenuNode/index.vue'

defineOptions({ name: 'AppAside' })

const props = withDefaults(
  defineProps<{
    collapsed: boolean
    uniqueOpened: boolean
    username?: string
    email?: string
    avatar?: string
    logoutPending?: boolean
  }>(),
  {
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
const canOpenProfile = computed(() => access.hasPermission('account:profile:view'))
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
    void router.push('/account/profile')
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
    <div class="app-aside__brand" aria-label="Admin">
      <span class="app-aside__mark">A</span>
      <span v-show="!collapsed" class="app-aside__name">Admin</span>
    </div>

    <el-menu
      class="app-aside__menu"
      router
      :collapse="collapsed"
      :collapse-transition="true"
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
          :title="t('layout.account.title')"
          :aria-label="t('layout.account.title')"
        >
          <el-avatar
            class="app-aside__avatar"
            :size="34"
            :src="avatarURL"
            @error="handleAvatarError"
            >{{ avatarText }}</el-avatar
          >
          <span v-show="!collapsed" class="app-aside__account-copy">
            <strong data-testid="aside-account-name">{{ username }}</strong>
            <small>{{ email }}</small>
          </span>
          <el-icon v-show="!collapsed" class="app-aside__account-arrow" aria-hidden="true"
            ><ArrowUp
          /></el-icon>
        </button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item
              v-if="canOpenProfile"
              data-testid="aside-account-profile"
              :icon="User"
              command="profile"
            >
              {{ t('layout.account.profile') }}
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

<style scoped>
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

.app-aside__account {
  flex: 0 0 auto;
  padding-top: 10px;
  border-top: 1px solid var(--admin-border);
}

.app-aside__account :deep(.el-dropdown) {
  display: block;
  width: 100%;
}

.app-aside__account-trigger {
  display: flex;
  align-items: center;
  width: 100%;
  min-height: 56px;
  padding: 9px;
  gap: 9px;
  color: inherit;
  font: inherit;
  text-align: left;
  border: 1px solid var(--admin-border);
  border-radius: 8px;
  background: var(--admin-surface-soft);
  cursor: pointer;
}

.app-aside__account-trigger:hover {
  border-color: var(--el-color-primary-light-5);
  background: var(--el-color-primary-light-9);
}

.app-aside__account-trigger:focus-visible {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 2px;
}

.app-aside__avatar {
  flex: 0 0 auto;
  color: var(--el-color-white);
  background: var(--el-color-primary);
  font-size: 13px;
  font-weight: 700;
}

.app-aside__account-copy {
  display: grid;
  min-width: 0;
  flex: 1;
  gap: 2px;
}

.app-aside__account-copy strong,
.app-aside__account-copy small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.app-aside__account-copy strong {
  color: var(--admin-text);
  font-size: 13px;
  font-weight: 700;
}

.app-aside__account-copy small {
  color: var(--admin-text-soft);
  font-size: 11px;
}

.app-aside__account-arrow {
  flex: 0 0 auto;
  color: var(--admin-text-soft);
  font-size: 13px;
}
</style>
