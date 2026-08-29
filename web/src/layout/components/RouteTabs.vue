<script setup lang="ts">
import {
  CircleClose,
  Close,
  DArrowLeft,
  DArrowRight,
  FolderDelete,
  FullScreen,
  Refresh,
  Setting,
} from '@element-plus/icons-vue'
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import type { AccessMenuNode } from '../../api/rbac/access'

interface RouteTab {
  path: string
	i18nKey: string
  affix: boolean
}

interface ScrollbarHandle {
  wrapRef?: HTMLElement
  setScrollLeft: (value: number) => void
}

type TabCommand = 'refresh' | 'fullscreen' | 'closeOthers' | 'closeAll'

const props = withDefaults(defineProps<{
	fullscreen?: boolean
	menuTree: readonly AccessMenuNode[]
}>(), { fullscreen: false })
const root = ref<HTMLElement>()
const tagsInnerRef = ref<HTMLElement | null>(null)
const scrollPaneRef = ref<ScrollbarHandle>()
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
const previousTab = computed(() => activeIndex.value > 0 ? tabs.value[activeIndex.value - 1] : undefined)
const nextTab = computed(() => activeIndex.value >= 0 && activeIndex.value < tabs.value.length - 1
  ? tabs.value[activeIndex.value + 1]
  : undefined)
const contextTab = computed(() => tabs.value.find((tab) => tab.path === contextPath.value))
const tagSignature = computed(() => tabs.value.map((tab) => tab.path).join('|'))

function slug(path: string): string {
  return path.replace(/^\//, '').replace(/[^a-zA-Z0-9_-]+/g, '-') || 'root'
}

function getCurrentTab(): RouteTab | null {
	const menuPage = findMenuPage(route.path, props.menuTree)
	if (menuPage !== null) {
		return { path: route.path, i18nKey: menuPage.i18nKey, affix: false }
	}
	if (route.path !== '/dashboard') return null
	const matched = [...route.matched].reverse().find((record) => record.name === 'dashboard')
  if (matched === undefined) {
		if (route.meta.requiresAuth === true) throw new Error(`Route ${route.fullPath} must declare i18nKey`)
    return null
  }
	const i18nKey = matched.meta.i18nKey
	if (i18nKey === undefined) throw new Error(`Route ${route.fullPath} must declare i18nKey`)
	return { path: route.path, i18nKey, affix: matched.meta.affix === true }
}

function findMenuPage(path: string, roots: readonly AccessMenuNode[]): AccessMenuNode | null {
	const stack = [...roots].reverse()
	while (stack.length > 0) {
		const node = stack.pop()
		if (node === undefined) continue
		if (node.menuType === 'page' && node.path === path) return node
		if (node.menuType === 'directory') stack.push(...[...node.children].reverse())
	}
	return null
}

function prefersReducedMotion(): boolean {
  return typeof window.matchMedia === 'function'
    && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

function scrollActiveTab(): void {
  void nextTick(() => {
    const activeElement = tagsInnerRef.value?.querySelector<HTMLElement>('.tag-item.active')
    if (typeof activeElement?.scrollIntoView !== 'function') return
    activeElement.scrollIntoView({
      block: 'nearest',
      inline: 'center',
      behavior: prefersReducedMotion() ? 'auto' : 'smooth',
    })
  })
}

function syncCurrentTab(): void {
  const currentTab = getCurrentTab()
  contextMenuOpen.value = false
  if (currentTab === null) return
  const existing = tabs.value.find((tab) => tab.path === currentTab.path)
  if (existing === undefined) tabs.value.push(currentTab)
  else {
		existing.i18nKey = currentTab.i18nKey
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
  if (selected !== undefined && route.path !== selected.path) await router.push(selected.path)
}

async function closeAll(): Promise<void> {
  tabs.value = tabs.value.filter((tab) => tab.affix)
  dismissContextMenu()
  await router.push('/dashboard')
}

async function navigateRelative(target: RouteTab | undefined): Promise<void> {
  if (target !== undefined) await router.push(target.path)
}

function refreshCurrent(): void {
  dismissContextMenu()
  emit('refresh')
}

function toggleFullscreen(): void {
  dismissContextMenu()
  emit('toggleFullscreen')
}

function handleScroll(event: WheelEvent): void {
  const wrap = scrollPaneRef.value?.wrapRef
  if (wrap === undefined) return
  const delta = event.deltaY || -event.detail
  scrollPaneRef.value?.setScrollLeft(wrap.scrollLeft + delta / 4)
}

function openContextMenu(event: MouseEvent, path: string): void {
  event.preventDefault()
  contextPath.value = path
  contextPosition.x = Math.max(8, Math.min(event.clientX, window.innerWidth - 188))
  contextPosition.y = Math.max(8, Math.min(event.clientY + 10, window.innerHeight - 170))
  contextMenuOpen.value = true
}

function dismissContextMenu(): void {
  contextMenuOpen.value = false
  contextPath.value = ''
}

function handleCommand(command: string | number | object): void {
  if (typeof command !== 'string') throw new Error(`Unsupported route tab command: ${String(command)}`)
  const value = command as TabCommand
  switch (value) {
    case 'refresh': refreshCurrent(); break
    case 'fullscreen': toggleFullscreen(); break
    case 'closeOthers': void closeOthers(route.path); break
    case 'closeAll': void closeAll(); break
    default: throw new Error(`Unsupported route tab command: ${command}`)
  }
}

async function handleContextRefresh(): Promise<void> {
  const selected = contextTab.value
  if (selected === undefined) return
  dismissContextMenu()
  if (selected.path !== route.path) await router.push(selected.path)
  emit('refresh')
}

function handleDocumentPointerDown(event: PointerEvent): void {
  const target = event.target
  if (contextMenuOpen.value && root.value !== undefined && target instanceof Node && !root.value.contains(target)) {
    dismissContextMenu()
  }
}

function handleDocumentKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape' && contextMenuOpen.value) {
    dismissContextMenu()
    event.preventDefault()
  }
}

watch([() => route.fullPath, () => props.menuTree], syncCurrentTab, { immediate: true })
watch(tagSignature, () => { scrollActiveTab() })

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
  <div ref="root" class="route-tabs" data-testid="route-tabs" tabindex="-1" @keydown.esc="dismissContextMenu">
    <div class="route-tabs__previous">
      <el-tooltip :content="t('layout.routeTabs.previous')">
        <el-button
          data-testid="route-tabs-previous"
          text
          :icon="DArrowLeft"
          :disabled="previousTab === undefined"
          :aria-label="t('layout.routeTabs.previous')"
          @click="navigateRelative(previousTab)"
        />
      </el-tooltip>
    </div>

    <div class="route-tabs__scroll route-tabs__horizontal-scroll">
      <el-scrollbar ref="scrollPaneRef" class="route-tabs__scrollbar" @wheel.prevent="handleScroll">
        <div ref="tagsInnerRef" class="route-tabs__inner">
          <TransitionGroup name="route-tab" tag="div" class="route-tabs__list">
            <div
              v-for="tab in tabs"
              :key="tab.path"
              class="route-tabs__item tag-item"
              :class="{ active: tab.path === route.path }"
              data-testid="route-tab"
              :data-path="tab.path"
              :data-affix="String(tab.affix)"
              :data-active="String(tab.path === route.path)"
              role="button"
              tabindex="0"
              :aria-current="tab.path === route.path ? 'page' : undefined"
              @click="navigateTo(tab.path)"
              @keydown.enter.prevent="navigateTo(tab.path)"
              @keydown.space.prevent="navigateTo(tab.path)"
              @contextmenu="openContextMenu($event, tab.path)"
            >
              <span v-if="tab.path === route.path" class="route-tabs__dot" />
				<span class="route-tabs__label">{{ t(tab.i18nKey) }}</span>
              <span
                v-if="!tab.affix"
                class="route-tabs__close"
                :data-testid="`route-tab-${slug(tab.path)}-close`"
                :aria-label="t('layout.routeTabs.close')"
                role="button"
                tabindex="0"
                @click.stop="closeTab(tab.path)"
              >
                <el-icon aria-hidden="true"><Close /></el-icon>
              </span>
            </div>
          </TransitionGroup>
        </div>
      </el-scrollbar>
    </div>

    <div class="route-tabs__next">
      <el-tooltip :content="t('layout.routeTabs.next')">
        <el-button
          data-testid="route-tabs-next"
          text
          :icon="DArrowRight"
          :disabled="nextTab === undefined"
          :aria-label="t('layout.routeTabs.next')"
          @click="navigateRelative(nextTab)"
        />
      </el-tooltip>
    </div>

    <el-tooltip v-if="props.fullscreen" :content="t('layout.routeTabs.exitFullscreen')">
      <el-button
        data-testid="exit-content-fullscreen"
        class="route-tabs__exit-fullscreen"
        text
        :icon="Close"
        :aria-label="t('layout.routeTabs.exitFullscreen')"
        @click="toggleFullscreen"
      />
    </el-tooltip>

    <el-dropdown class="route-tabs__actions" trigger="click" @command="handleCommand">
      <button
        data-testid="route-tabs-settings"
        type="button"
        class="route-tabs__action"
        :aria-label="t('layout.routeTabs.contextMenu')"
      >
        <el-icon aria-hidden="true"><Setting /></el-icon>
      </button>
      <template #dropdown>
        <el-dropdown-menu>
          <el-dropdown-item data-testid="route-tabs-refresh" command="refresh" :icon="Refresh">
            {{ t('layout.routeTabs.refresh') }}
          </el-dropdown-item>
          <el-dropdown-item data-testid="route-tabs-fullscreen" command="fullscreen" :icon="FullScreen">
            {{ props.fullscreen ? t('layout.routeTabs.exitFullscreen') : t('layout.routeTabs.fullscreen') }}
          </el-dropdown-item>
          <el-dropdown-item data-testid="route-tabs-close-others" command="closeOthers" :icon="CircleClose">
            {{ t('layout.routeTabs.closeOthers') }}
          </el-dropdown-item>
          <el-dropdown-item data-testid="route-tabs-close-all" command="closeAll" :icon="FolderDelete">
            {{ t('layout.routeTabs.closeAll') }}
          </el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>

    <ul
      v-if="contextMenuOpen"
      class="route-tabs__context-menu"
      role="menu"
      :aria-label="t('layout.routeTabs.contextMenu')"
      :style="{ left: `${contextPosition.x}px`, top: `${contextPosition.y}px` }"
      @pointerdown.stop
    >
      <li data-testid="route-tabs-refresh-context" role="menuitem" @click="handleContextRefresh">
        <el-icon><Refresh /></el-icon>{{ t('layout.routeTabs.refresh') }}
      </li>
      <li
        v-if="contextTab?.affix !== true"
        data-testid="route-tabs-close"
        role="menuitem"
        @click="closeTab(contextPath)"
      >
        <el-icon><Close /></el-icon>{{ t('layout.routeTabs.close') }}
      </li>
      <li data-testid="route-tabs-close-others-context" role="menuitem" @click="closeOthers(contextPath)">
        <el-icon><CircleClose /></el-icon>{{ t('layout.routeTabs.closeOthers') }}
      </li>
      <li data-testid="route-tabs-close-all-context" role="menuitem" @click="closeAll">
        <el-icon><FolderDelete /></el-icon>{{ t('layout.routeTabs.closeAll') }}
      </li>
    </ul>
  </div>
</template>

<style scoped>
.route-tabs {
  display: flex;
  align-items: center;
  min-width: 0;
  height: 40px;
  padding: 0 10px 0 8px;
  gap: 4px;
  color: var(--el-text-color-regular);
  background: transparent;
}

.route-tabs__previous,
.route-tabs__next,
.route-tabs__exit-fullscreen,
.route-tabs__actions {
  flex: 0 0 auto;
}

.route-tabs__previous .el-button,
.route-tabs__next .el-button,
.route-tabs__exit-fullscreen {
  width: 30px;
  height: 30px;
  margin: 0;
  border: 1px solid var(--el-border-color-light);
  border-radius: 9px;
  color: var(--el-text-color-secondary);
}

.route-tabs__previous .el-button:hover:not(:disabled),
.route-tabs__next .el-button:hover:not(:disabled),
.route-tabs__exit-fullscreen:hover,
.route-tabs__action:hover {
  color: var(--el-text-color-primary);
  background: var(--el-fill-color-light);
  border-color: var(--el-border-color);
}

.route-tabs__scroll {
  flex: 1 1 auto;
  min-width: 0;
  height: 100%;
  overflow: hidden;
}

.route-tabs__scrollbar { height: 100%; }
.route-tabs__scrollbar :deep(.el-scrollbar__view) { height: 100%; }
.route-tabs__inner { display: flex; align-items: center; height: 100%; }
.route-tabs__list { display: flex; align-items: center; min-width: max-content; gap: 4px; padding: 0 2px; }

.route-tabs__item {
  position: relative;
  display: inline-flex;
  align-items: center;
  height: 30px;
  max-width: 220px;
  padding: 0 12px;
  gap: 7px;
  border: 1px solid transparent;
  border-radius: 9px;
  background: transparent;
  color: var(--el-text-color-secondary);
  cursor: pointer;
  user-select: none;
  white-space: nowrap;
  transition: all 160ms ease;
}

.route-tabs__item:hover { color: var(--el-text-color-primary); background: var(--el-fill-color-light); border-color: var(--el-border-color-light); }
.route-tabs__item.active { color: var(--el-text-color-primary); background: var(--el-bg-color); border-color: var(--el-border-color-light); }
.route-tabs__item:focus-visible { outline: 2px solid var(--el-color-primary); outline-offset: 1px; }
.route-tabs__dot { width: 5px; height: 5px; flex: 0 0 5px; border-radius: 50%; background: var(--el-color-primary); }
.route-tabs__label { max-width: 144px; overflow: hidden; text-overflow: ellipsis; font-size: 13px; font-weight: 500; }
.route-tabs__close { display: grid; width: 16px; height: 16px; flex: 0 0 16px; place-items: center; margin-right: -4px; border-radius: 6px; color: currentColor; opacity: 0; transform: scale(0.88); transition: all 160ms ease; }
.route-tabs__item:hover .route-tabs__close, .route-tabs__item.active .route-tabs__close { opacity: 1; transform: scale(1); }
.route-tabs__close:hover { background: var(--el-fill-color); }

.route-tabs__action {
  position: relative;
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  padding: 0;
  border: 1px solid var(--el-border-color-light);
  border-radius: 9px;
  color: var(--el-text-color-secondary);
  background: var(--el-bg-color);
  cursor: pointer;
}
.route-tabs__actions { position: relative; margin-left: 6px; }
.route-tabs__actions::before { position: absolute; width: 1px; height: 18px; margin-left: -8px; background: var(--el-border-color-light); content: ''; }

.route-tabs__context-menu { position: fixed; z-index: 3000; min-width: 132px; margin: 0; padding: 8px 0; list-style: none; background: var(--el-bg-color-overlay); border: 1px solid var(--el-border-color-light); border-radius: 12px; box-shadow: var(--el-box-shadow-light); }
.route-tabs__context-menu li { display: flex; align-items: center; gap: 8px; padding: 9px 16px; color: var(--el-text-color-primary); font-size: 13px; cursor: pointer; }
.route-tabs__context-menu li:hover { background: var(--el-fill-color-light); }
.route-tabs__context-menu .el-icon { color: var(--el-text-color-regular); }

.route-tab-enter-active, .route-tab-leave-active, .route-tab-move { transition: transform 160ms ease, opacity 120ms ease; }
.route-tab-enter-from, .route-tab-leave-to { opacity: 0; transform: translateY(6px) scale(0.96); }
.route-tab-leave-active { position: absolute; pointer-events: none; }

@media (max-width: 768px) {
  .route-tabs { height: 34px; padding-inline: 6px; }
  .route-tabs__previous .el-button, .route-tabs__next .el-button, .route-tabs__exit-fullscreen, .route-tabs__action { width: 26px; height: 26px; border-radius: 7px; }
  .route-tabs__list { gap: 3px; }
  .route-tabs__item { height: 26px; padding: 0 10px; border-radius: 7px; }
  .route-tabs__label { max-width: 88px; font-size: 12px; }
  .route-tabs__dot { width: 4px; height: 4px; flex-basis: 4px; }
}
</style>
