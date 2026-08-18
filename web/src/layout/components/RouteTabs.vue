<script setup lang="ts">
import { ArrowLeft, ArrowRight, Close, DArrowLeft, DArrowRight, FullScreen, MoreFilled, Refresh } from '@element-plus/icons-vue'
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import type { AppMessageKey } from '../../i18n'

interface RouteTab {
  path: string
  titleKey: AppMessageKey
  affix: boolean
}

const root = ref<HTMLElement>()
const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const tabs = ref<RouteTab[]>([])
const contextPath = ref('')
const contextMenuOpen = ref(false)
const contextPosition = reactive({ x: 0, y: 0 })

const emit = defineEmits<{
  refresh: []
  toggleFullscreen: []
}>()

const activeIndex = computed(() => tabs.value.findIndex((tab) => tab.path === route.path))
const contextTab = computed(() => tabs.value.find((tab) => tab.path === contextPath.value))
const previousDisabled = computed(() => activeIndex.value <= 0)
const nextDisabled = computed(() => activeIndex.value < 0 || activeIndex.value >= tabs.value.length - 1)

function slug(path: string): string {
  return path.replace(/^\//, '').replace(/[^a-zA-Z0-9_-]+/g, '-') || 'root'
}

function getCurrentTab(): RouteTab | null {
  const matched = [...route.matched].reverse().find((record) => record.meta.titleKey !== undefined)
  if (matched === undefined) {
    if (route.meta.requiresAuth === true) {
      throw new Error(`Route ${route.fullPath} must declare titleKey`)
    }
    return null
  }
  const titleKey = matched.meta.titleKey
  if (titleKey === undefined) {
    throw new Error(`Route ${route.fullPath} must declare titleKey`)
  }
  return {
    path: route.path,
    titleKey,
    affix: matched.meta.affix === true,
  }
}

function prefersReducedMotion(): boolean {
  return typeof window.matchMedia === 'function'
    && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

function scrollActiveTab(): void {
  void nextTick(() => {
    const activeElement = root.value?.querySelector<HTMLElement>(
      `[data-testid="route-tab-${slug(route.path)}"]`,
    )
    if (typeof activeElement?.scrollIntoView === 'function') {
      activeElement.scrollIntoView({
        block: 'nearest',
        inline: 'nearest',
        behavior: prefersReducedMotion() ? 'auto' : 'smooth',
      })
    }
  })
}

function syncCurrentTab(): void {
  const currentTab = getCurrentTab()
  contextMenuOpen.value = false
  if (currentTab === null) return

  const existing = tabs.value.find((tab) => tab.path === currentTab.path)
  if (existing === undefined) {
    tabs.value.push(currentTab)
  } else {
    existing.titleKey = currentTab.titleKey
    existing.affix = currentTab.affix
  }
  scrollActiveTab()
}

async function navigateTo(path: string): Promise<void> {
  dismissContextMenu()
  await router.push(path)
}

async function closeTab(path: string): Promise<void> {
  const index = tabs.value.findIndex((tab) => tab.path === path)
  const tab = tabs.value[index]
  if (index < 0 || tab === undefined || tab.affix) return

  const isActive = route.path === path
  tabs.value.splice(index, 1)
  dismissContextMenu()
  if (!isActive) return

  const destination = tabs.value[index - 1] ?? tabs.value[index] ?? tabs.value.find((item) => item.affix)
  await router.push(destination?.path ?? '/dashboard')
}

async function closeOthers(path: string): Promise<void> {
  const selected = tabs.value.find((tab) => tab.path === path)
  tabs.value = tabs.value.filter((tab) => tab.affix || tab.path === path)
  dismissContextMenu()
  if (selected !== undefined && route.path !== selected.path) {
    await router.push(selected.path)
  }
}

async function closeAll(): Promise<void> {
  tabs.value = tabs.value.filter((tab) => tab.affix)
  dismissContextMenu()
  await router.push('/dashboard')
}

async function navigateRelative(offset: -1 | 1): Promise<void> {
  const target = tabs.value[activeIndex.value + offset]
  if (target === undefined) return
  await router.push(target.path)
}

function refreshCurrent(): void {
  dismissContextMenu()
  emit('refresh')
}

function toggleFullscreen(): void {
  dismissContextMenu()
  emit('toggleFullscreen')
}

function openContextMenu(event: MouseEvent, path: string): void {
  event.preventDefault()
  contextPath.value = path
  contextPosition.x = Math.max(8, Math.min(event.clientX, window.innerWidth - 188))
  contextPosition.y = Math.max(8, Math.min(event.clientY, window.innerHeight - 170))
  contextMenuOpen.value = true
}

function dismissContextMenu(): void {
  contextMenuOpen.value = false
  contextPath.value = ''
}

function handleDocumentPointerDown(event: PointerEvent): void {
  const target = event.target
  if (contextMenuOpen.value && root.value !== undefined && target instanceof Node && !root.value.contains(target)) {
    dismissContextMenu()
  }
}

function handleDocumentKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') dismissContextMenu()
}

watch(() => route.fullPath, syncCurrentTab, { immediate: true })

onMounted(() => {
  document.addEventListener('pointerdown', handleDocumentPointerDown)
  document.addEventListener('keydown', handleDocumentKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', handleDocumentPointerDown)
  document.removeEventListener('keydown', handleDocumentKeydown)
})
</script>

<template>
  <div
    ref="root"
    class="route-tabs"
    data-testid="route-tabs"
    tabindex="-1"
    @keydown.esc="dismissContextMenu"
  >
    <div class="route-tabs__toolbar">
      <el-tooltip :content="t('layout.routeTabs.previous')">
        <el-button
          data-testid="route-tabs-previous"
          text
          :icon="ArrowLeft"
          :disabled="previousDisabled"
          :aria-label="t('layout.routeTabs.previous')"
          @click="navigateRelative(-1)"
        />
      </el-tooltip>
      <el-tooltip :content="t('layout.routeTabs.next')">
        <el-button
          data-testid="route-tabs-next"
          text
          :icon="ArrowRight"
          :disabled="nextDisabled"
          :aria-label="t('layout.routeTabs.next')"
          @click="navigateRelative(1)"
        />
      </el-tooltip>
      <el-tooltip :content="t('layout.routeTabs.refresh')">
        <el-button
          data-testid="route-tabs-refresh"
          text
          :icon="Refresh"
          :aria-label="t('layout.routeTabs.refresh')"
          @click="refreshCurrent"
        />
      </el-tooltip>
      <el-tooltip :content="t('layout.routeTabs.fullscreen')">
        <el-button
          data-testid="route-tabs-fullscreen"
          text
          :icon="FullScreen"
          :aria-label="t('layout.routeTabs.fullscreen')"
          @click="toggleFullscreen"
        />
      </el-tooltip>
      <el-tooltip :content="t('layout.routeTabs.closeOthers')">
        <el-button
          data-testid="route-tabs-close-others"
          text
          :icon="DArrowLeft"
          :aria-label="t('layout.routeTabs.closeOthers')"
          @click="closeOthers(route.path)"
        />
      </el-tooltip>
      <el-tooltip :content="t('layout.routeTabs.closeAll')">
        <el-button
          data-testid="route-tabs-close-all"
          text
          :icon="DArrowRight"
          :aria-label="t('layout.routeTabs.closeAll')"
          @click="closeAll"
        />
      </el-tooltip>
    </div>

    <nav class="route-tabs__list route-tabs__horizontal-scroll" :aria-label="t('layout.routeTabs.contextMenu')">
      <div
        v-for="tab in tabs"
        :key="tab.path"
        class="route-tabs__item"
        data-testid="route-tab"
        :data-affix="String(tab.affix)"
        :data-active="String(tab.path === route.path)"
        @contextmenu="openContextMenu($event, tab.path)"
      >
        <button
          class="route-tabs__tab"
          :data-testid="`route-tab-${slug(tab.path)}`"
          :data-affix="String(tab.affix)"
          :aria-current="tab.path === route.path ? 'page' : undefined"
          :data-active="String(tab.path === route.path)"
          @click="navigateTo(tab.path)"
        >
          <span>{{ t(tab.titleKey) }}</span>
        </button>
        <button
          v-if="!tab.affix"
          class="route-tabs__close"
          :data-testid="`route-tab-${slug(tab.path)}-close`"
          :aria-label="t('layout.routeTabs.close')"
          @click.stop="closeTab(tab.path)"
        >
          <el-icon aria-hidden="true"><Close /></el-icon>
        </button>
        <button
          class="route-tabs__menu-trigger"
          :data-testid="`route-tab-${slug(tab.path)}-menu`"
          :aria-label="t('layout.routeTabs.contextMenu')"
          @click.stop="openContextMenu($event, tab.path)"
          @contextmenu.stop="openContextMenu($event, tab.path)"
        >
          <el-icon aria-hidden="true"><MoreFilled /></el-icon>
        </button>
      </div>
    </nav>

    <div
      v-if="contextMenuOpen"
      class="route-tabs__context-menu"
      role="menu"
      :aria-label="t('layout.routeTabs.contextMenu')"
      :style="{ left: `${contextPosition.x}px`, top: `${contextPosition.y}px` }"
      @pointerdown.stop
    >
      <button
        data-testid="route-tabs-refresh-context"
        role="menuitem"
        @click="refreshCurrent"
      >
        {{ t('layout.routeTabs.refresh') }}
      </button>
      <button
        data-testid="route-tabs-close"
        role="menuitem"
        :disabled="contextTab?.affix === true"
        @click="closeTab(contextPath)"
      >
        {{ t('layout.routeTabs.close') }}
      </button>
      <button
        data-testid="route-tabs-close-others-context"
        role="menuitem"
        @click="closeOthers(contextPath)"
      >
        {{ t('layout.routeTabs.closeOthers') }}
      </button>
      <button
        data-testid="route-tabs-close-all-context"
        role="menuitem"
        @click="closeAll"
      >
        {{ t('layout.routeTabs.closeAll') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.route-tabs {
  position: relative;
  display: flex;
  align-items: center;
  min-width: 0;
  height: 42px;
  padding: 0 10px;
  gap: 8px;
  color: var(--el-text-color-regular);
  background: var(--el-bg-color);
  border-bottom: 1px solid var(--el-border-color-light);
}

.route-tabs__toolbar {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 2px;
}

.route-tabs__toolbar .el-button {
  width: 28px;
  height: 28px;
  margin: 0;
}

.route-tabs__list {
  display: flex;
  align-items: center;
  min-width: 0;
  height: 100%;
  overflow-x: auto;
  gap: 4px;
}

.route-tabs__item {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  height: 30px;
  max-width: 220px;
  background: var(--el-fill-color-light);
  border: 1px solid transparent;
  border-radius: 4px;
}

.route-tabs__item[data-active='true'] {
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  border-color: var(--el-color-primary-light-7);
}

.route-tabs__tab,
.route-tabs__close,
.route-tabs__menu-trigger {
  height: 28px;
  border: 0;
  color: inherit;
  background: transparent;
  cursor: pointer;
}

.route-tabs__tab {
  min-width: 74px;
  max-width: 180px;
  padding: 0 8px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font: inherit;
  font-size: 12px;
}

.route-tabs__close {
  display: grid;
  width: 24px;
  flex: 0 0 24px;
  place-items: center;
  padding: 0;
  font-size: 12px;
}

.route-tabs__menu-trigger {
  display: grid;
  width: 22px;
  flex: 0 0 22px;
  place-items: center;
  padding: 0;
  font-size: 12px;
}

.route-tabs__tab:hover,
.route-tabs__close:hover,
.route-tabs__menu-trigger:hover {
  color: var(--el-color-primary);
}

.route-tabs__context-menu {
  position: fixed;
  z-index: 100;
  display: grid;
  width: 180px;
  padding: 4px;
  background: var(--el-bg-color-overlay);
  border: 1px solid var(--el-border-color-light);
  border-radius: 4px;
  box-shadow: var(--el-box-shadow-light);
}

.route-tabs__context-menu button {
  min-height: 30px;
  padding: 0 10px;
  color: var(--el-text-color-regular);
  text-align: left;
  background: transparent;
  border: 0;
  border-radius: 3px;
  cursor: pointer;
  font: inherit;
  font-size: 12px;
}

.route-tabs__context-menu button:hover:not(:disabled) {
  background: var(--el-fill-color-light);
}

.route-tabs__context-menu button:disabled {
  color: var(--el-text-color-placeholder);
  cursor: not-allowed;
}
</style>
