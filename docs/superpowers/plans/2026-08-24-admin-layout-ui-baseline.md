# Admin Layout UI Baseline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将现有后台壳层和六个系统业务页调整为老项目的紧凑布局基线，并增加真实生效、严格持久化的系统设置抽屉。

**Architecture:** UI 偏好使用一个具体的 `uiPreferences` Pinia Store，并通过严格的版本化 localStorage 契约持久化；App 根组件在任何路由页面渲染前初始化该 Store，Layout 只展示已保留的偏好错误；Header 面包屑由纯函数从当前 `route.path` 和 `access.menuTree` 线性求出。Layout 继续串联 Aside、Header、RouteTabs、Main、Footer 和瞬时全屏/折叠状态，业务页面继续直接调用各自 API，不把请求、权限或错误处理移入布局组件。

**Tech Stack:** Vue 3 `<script setup>`, TypeScript strict mode, Pinia 4, Vue Router 4, Vue I18n 11, Element Plus 2.14, Vitest 4, SCSS.

**Git boundary:** 项目所有者控制 Git 历史。本计划不创建 commit、不 fetch、不 pull、不 push；每个任务末尾只检查 diff 和工作区，不清理其他已有改动。

---

## File Map

**Create:**

- `web/src/utils/ui-preferences.ts`：版本化 UI 偏好 DTO、严格解析、localStorage 读写。
- `web/src/utils/ui-preferences.test.ts`：缺省值、严格解析、坏数据和存储失败测试。
- `web/src/store/ui-preferences.ts`：Pinia UI 偏好状态、初始化失败状态、更新和重置动作。
- `web/src/store/ui-preferences.test.ts`：Store 初始化、更新、失败可见性和重置测试。
- `web/src/layout/breadcrumbs.ts`：Dashboard 与动态 RBAC 菜单面包屑求解。
- `web/src/layout/breadcrumbs.test.ts`：目录/页面路径、未知路径和输入不变性测试。
- `web/src/layout/components/SettingDrawer.vue`：主题、主题色、界面显示、过渡和重置控件。
- `web/src/layout/components/SettingDrawer.test.ts`：抽屉控件到 Store 的真实联动测试。
- `web/src/styles/system-pages.scss`：六个系统页面共用的内容面、筛选区、操作区和移动端密度。

**Modify:**

- `web/src/utils/theme.ts`, `web/src/utils/theme.test.ts`：移除散落的主题存储，增加 Element Plus 主题色派生变量。
- `web/src/App.vue`：在任何路由页面渲染前初始化 UI 偏好；坏数据时使用内存默认值并保留错误状态。
- `web/src/main.ts`：删除旧 `initializeTheme()` 入口。
- `web/src/layout/components/AppHeader.vue`：动态面包屑、设置入口和 SettingDrawer。
- `web/src/layout/components/AppAside.vue`：真实 padding、独立滚动、折叠密度和 `uniqueOpened`。
- `web/src/layout/components/AccessMenuNode.vue`：保持 RBAC 渲染，只补稳定菜单类名和可访问性属性。
- `web/src/layout/index.vue`, `web/src/layout/index.test.ts`：偏好联动、错误状态、条件 Header/RouteTabs/Footer、页面过渡和 Main padding。
- `web/src/layout/components/AccessMenuNode.test.ts`：Aside `uniqueOpened` 和菜单结构测试。
- `web/src/styles/index.scss`：引入系统页面基线并收敛为壳层级全局样式。
- `web/src/i18n/messages/zh-CN.ts`, `web/src/i18n/messages/en-US.ts`：设置抽屉、偏好错误和面包屑错误文案。
- 六个系统页面及对应测试：删除重复标题，保留并重新安置真实操作。

---

### Task 1: Element Plus 主题色应用工具

**Files:**

- Modify: `web/src/utils/theme.ts`
- Modify: `web/src/utils/theme.test.ts`

- [ ] **Step 1: 写主题色派生变量失败测试**

在 `theme.test.ts` 中保留明暗模式断言，但删除 `admin:theme` localStorage 断言，增加：

```ts
import { applyPrimaryColor, applyTheme, mixHexColor } from './theme'

it('derives the Element Plus primary palette from one six-digit hex color', () => {
  applyPrimaryColor('#409EFF')

  const style = document.documentElement.style
  expect(style.getPropertyValue('--el-color-primary')).toBe('#409EFF')
  expect(style.getPropertyValue('--el-color-primary-rgb')).toBe('64, 158, 255')
  expect(style.getPropertyValue('--el-color-primary-light-3')).toBe('#79BBFF')
  expect(style.getPropertyValue('--el-color-primary-light-5')).toBe('#A0CFFF')
  expect(style.getPropertyValue('--el-color-primary-dark-2')).toBe('#337ECC')
})

it('rejects malformed colors instead of applying a partial palette', () => {
  expect(() => applyPrimaryColor('blue')).toThrow('primary color must be a six-digit hex color')
})

it('mixes channels deterministically', () => {
  expect(mixHexColor('#000000', '#FFFFFF', 0.5)).toBe('#808080')
})
```

- [ ] **Step 2: 运行测试确认失败原因正确**

Run:

```powershell
cd D:\admin\web
pnpm vitest run src/utils/theme.test.ts
```

Expected: FAIL，提示 `applyPrimaryColor` / `mixHexColor` 未导出；不能是测试环境或导入路径错误。

- [ ] **Step 3: 实现单一主题色到 Element Plus 变量的确定性转换**

将 `theme.ts` 收敛为纯视觉应用工具：

```ts
export type ThemeMode = 'light' | 'dark'

const sixDigitHex = /^#[0-9a-fA-F]{6}$/

export function applyTheme(theme: ThemeMode): void {
  document.documentElement.classList.toggle('dark', theme === 'dark')
  document.documentElement.style.colorScheme = theme
}

export function mixHexColor(base: string, target: string, weight: number): string {
  const baseChannels = parseHex(base)
  const targetChannels = parseHex(target)
  if (!Number.isFinite(weight) || weight < 0 || weight > 1) {
    throw new Error('color mix weight must be between 0 and 1')
  }
  return `#${baseChannels.map((channel, index) => (
    Math.round(channel + (targetChannels[index] - channel) * weight)
      .toString(16).padStart(2, '0')
  )).join('').toUpperCase()}`
}

export function applyPrimaryColor(color: string): void {
  if (!sixDigitHex.test(color)) throw new Error('primary color must be a six-digit hex color')
  const normalized = color.toUpperCase()
  const channels = parseHex(normalized)
  const style = document.documentElement.style
  style.setProperty('--el-color-primary', normalized)
  style.setProperty('--el-color-primary-rgb', channels.join(', '))
  for (const [suffix, weight] of [['3', 0.3], ['5', 0.5], ['7', 0.7], ['8', 0.8], ['9', 0.9]] as const) {
    style.setProperty(`--el-color-primary-light-${suffix}`, mixHexColor(normalized, '#FFFFFF', weight))
  }
  style.setProperty('--el-color-primary-dark-2', mixHexColor(normalized, '#000000', 0.2))
}

function parseHex(value: string): [number, number, number] {
  if (!sixDigitHex.test(value)) throw new Error('color must be a six-digit hex color')
  return [
    Number.parseInt(value.slice(1, 3), 16),
    Number.parseInt(value.slice(3, 5), 16),
    Number.parseInt(value.slice(5, 7), 16),
  ]
}
```

不要在此文件读写 localStorage；持久化只属于 Task 2。

- [ ] **Step 4: 运行主题工具测试**

Run: `pnpm vitest run src/utils/theme.test.ts`

Expected: PASS。

- [ ] **Step 5: 检查本任务 diff，不提交**

Run:

```powershell
cd D:\admin
git diff --check -- web/src/utils/theme.ts web/src/utils/theme.test.ts
git status --short
```

Expected: `git diff --check` 无错误；保留其他已有改动。

---

### Task 2: 严格、版本化的 UI 偏好契约

**Files:**

- Create: `web/src/utils/ui-preferences.ts`
- Create: `web/src/utils/ui-preferences.test.ts`

- [ ] **Step 1: 写默认值、闭合对象和损坏数据失败测试**

测试必须覆盖：storage 缺失返回项目默认值；合法记录解析；未知字段、缺字段、非法主题、非法颜色、非法过渡名和错误版本全部抛出。

```ts
import {
  defaultUIPreferences,
  parseStoredUIPreferences,
  readUIPreferences,
  uiPreferencesStorageKey,
  writeUIPreferences,
} from './ui-preferences'

it('uses explicit defaults only when storage is absent', () => {
  expect(readUIPreferences()).toEqual(defaultUIPreferences)
})

it('rejects unknown and missing persisted fields', () => {
  const valid = { version: 1, preferences: defaultUIPreferences }
  expect(() => parseStoredUIPreferences({ ...valid, extra: true })).toThrow()
  const { showFooter: _removed, ...incomplete } = defaultUIPreferences
  expect(() => parseStoredUIPreferences({ version: 1, preferences: incomplete })).toThrow()
})

it('round trips one versioned record', () => {
  writeUIPreferences({ ...defaultUIPreferences, theme: 'dark', primaryColor: '#059669' })
  expect(JSON.parse(localStorage.getItem(uiPreferencesStorageKey) ?? '')).toMatchObject({ version: 1 })
  expect(readUIPreferences().theme).toBe('dark')
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm vitest run src/utils/ui-preferences.test.ts`

Expected: FAIL，模块不存在。

- [ ] **Step 3: 实现明确 DTO 与严格解析器**

实现以下公开契约：

```ts
export type PageTransitionName = 'fade' | 'slide-left' | 'zoom'

export interface UIPreferences {
  theme: 'light' | 'dark'
  primaryColor: string
  showBreadcrumb: boolean
  showMenuToggle: boolean
  showRouteTabs: boolean
  uniqueOpened: boolean
  showFooter: boolean
  pageTransition: boolean
  transitionName: PageTransitionName
}

export const uiPreferencesStorageKey = 'admin:ui-preferences'
export const defaultUIPreferences: Readonly<UIPreferences> = Object.freeze({
  theme: 'light',
  primaryColor: '#409EFF',
  showBreadcrumb: true,
  showMenuToggle: true,
  showRouteTabs: true,
  uniqueOpened: true,
  showFooter: true,
  pageTransition: true,
  transitionName: 'fade',
})

export class UIPreferencesError extends Error {
  constructor(message: string, options?: ErrorOptions) {
    super(message, options)
    this.name = 'UIPreferencesError'
  }
}
```

`parseStoredUIPreferences(value: unknown)` 必须先验证根对象键精确为 `version/preferences`，版本严格等于 `1`，偏好对象键精确等于接口全部九个字段，再逐字段缩小类型。`readUIPreferences()` 只在 key 不存在时返回默认值；JSON 损坏或字段不合法必须抛 `UIPreferencesError`。`writeUIPreferences()` 只写 `{ version: 1, preferences }`。

- [ ] **Step 4: 运行偏好契约测试**

Run: `pnpm vitest run src/utils/ui-preferences.test.ts`

Expected: PASS。

- [ ] **Step 5: 执行定向 TypeScript 检查**

Run:

```powershell
pnpm exec vue-tsc -b --pretty false
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" src/utils/ui-preferences.ts src/utils/ui-preferences.test.ts
```

Expected: `vue-tsc` exit 0；`rg` 无输出。

---

### Task 3: UI 偏好 Store 与应用启动初始化

**Files:**

- Create: `web/src/store/ui-preferences.ts`
- Create: `web/src/store/ui-preferences.test.ts`
- Modify: `web/src/App.vue`
- Modify: `web/src/main.ts`

- [ ] **Step 1: 写 Store 的红灯测试**

覆盖初始化视觉状态、更新持久化、写失败不篡改内存、坏数据使用内存默认值并保留错误、重置覆盖坏记录：

```ts
const store = useUIPreferencesStore(createPinia())

it('applies valid persisted preferences during initialization', () => {
  localStorage.setItem(uiPreferencesStorageKey, JSON.stringify({
    version: 1,
    preferences: { ...defaultUIPreferences, theme: 'dark', primaryColor: '#059669' },
  }))
  store.initialize()
  expect(store.preferences.theme).toBe('dark')
  expect(document.documentElement.classList.contains('dark')).toBe(true)
  expect(document.documentElement.style.getPropertyValue('--el-color-primary')).toBe('#059669')
})

it('keeps defaults and records an explicit error when initialization fails', () => {
  localStorage.setItem(uiPreferencesStorageKey, '{broken')
  store.initializeSafely()
  expect(store.preferences).toEqual(defaultUIPreferences)
  expect(store.persistenceError).toBe('invalid')
})

it('does not mutate live preferences when persistence fails', () => {
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new DOMException('quota') })
  store.update({ theme: 'dark' })
  expect(store.preferences.theme).toBe('light')
  expect(store.persistenceError).toBe('write')
})
```

- [ ] **Step 2: 运行 Store 测试确认失败**

Run: `pnpm vitest run src/store/ui-preferences.test.ts`

Expected: FAIL，Store 不存在。

- [ ] **Step 3: 实现具体 Store**

Store 公开状态和动作固定为：

```ts
export type UIPersistenceError = 'invalid' | 'write' | null

export const useUIPreferencesStore = defineStore('uiPreferences', () => {
  const preferences = ref<UIPreferences>({ ...defaultUIPreferences })
  const persistenceError = ref<UIPersistenceError>(null)
  const initialized = ref(false)

  function initialize(): void {
    replace(readUIPreferences())
    persistenceError.value = null
    initialized.value = true
  }

  function initializeSafely(): void {
    try {
      initialize()
    } catch (error: unknown) {
      replace(defaultUIPreferences)
      persistenceError.value = 'invalid'
      initialized.value = true
      if (!(error instanceof Error)) throw error
    }
  }

  function update(patch: Partial<UIPreferences>): void {
    const next = { ...preferences.value, ...patch }
    try {
      writeUIPreferences(next)
    } catch {
      persistenceError.value = 'write'
      return
    }
    replace(next)
    persistenceError.value = null
  }

  function reset(): void {
    const next = { ...defaultUIPreferences }
    try {
      writeUIPreferences(next)
    } catch {
      persistenceError.value = 'write'
      return
    }
    replace(next)
    persistenceError.value = null
  }

  function replace(next: Readonly<UIPreferences>): void {
    preferences.value = { ...next }
    applyTheme(next.theme)
    applyPrimaryColor(next.primaryColor)
  }

  return { preferences, persistenceError, initialized, initialize, initializeSafely, update, reset }
})
```

`initializeSafely()` 的 catch 不能只记录日志；它必须设置可渲染的 `persistenceError`。不要加入 Access/Auth/API 状态。

- [ ] **Step 4: 在 App 根组件同步初始化 Store**

`App.vue` 在 `<RouterView />` 前执行：

```vue
<script setup lang="ts">
import { useUIPreferencesStore } from './store/ui-preferences'

const uiPreferences = useUIPreferencesStore()
if (!uiPreferences.initialized) uiPreferences.initializeSafely()
</script>

<template><RouterView /></template>
```

从 `main.ts` 删除 `initializeTheme` import 和调用；保留 locale 与 permission guard 顺序。

这里由 App 根组件调用安全初始化，是为了让登录、注册等公开路由也在首次渲染前应用主题。Store 保留 `persistenceError`，进入后台 Layout 后由 Layout 的不可关闭错误提示展示，不在 App 根组件静默清除。

- [ ] **Step 5: 运行 Store、App 和主题测试**

Run:

```powershell
pnpm vitest run src/store/ui-preferences.test.ts src/utils/ui-preferences.test.ts src/utils/theme.test.ts
pnpm exec vue-tsc -b --pretty false
```

Expected: 全部 PASS。

---

### Task 4: RBAC 面包屑纯函数

**Files:**

- Create: `web/src/layout/breadcrumbs.ts`
- Create: `web/src/layout/breadcrumbs.test.ts`

- [ ] **Step 1: 写 Dashboard、动态树和未知路径测试**

```ts
it('returns the fixed Dashboard breadcrumb', () => {
  expect(resolveBreadcrumbs('/dashboard', [])).toEqual([
    { path: '/dashboard', titleKey: 'navigation.dashboard' },
  ])
})

it('returns directory to leaf order without inventing a directory path', () => {
  expect(resolveBreadcrumbs('/system/users', [systemDirectory()])).toEqual([
    { path: null, titleKey: 'navigation.system' },
    { path: '/system/users', titleKey: 'navigation.systemUsers' },
  ])
})

it('returns null for an authenticated path missing from the access tree', () => {
  expect(resolveBreadcrumbs('/system/missing', [systemDirectory()])).toBeNull()
})
```

同时冻结输入树，确认函数不排序、不改写 `children`。

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm vitest run src/layout/breadcrumbs.test.ts`

Expected: FAIL，模块不存在。

- [ ] **Step 3: 实现迭代 DFS 路径求解**

公开类型和函数：

```ts
export interface HeaderBreadcrumb {
  path: string | null
  titleKey: AppMessageKey
}

export function resolveBreadcrumbs(
  routePath: string,
  menuTree: readonly AccessMenuNode[],
): HeaderBreadcrumb[] | null
```

Dashboard 直接返回固定项。动态菜单使用显式 stack 保存 `{ node, ancestors }`；目录只加入 `titleKey/path:null`，页面匹配 `node.path === routePath` 时返回祖先加页面。Action 节点不会出现在 access menu tree；若出现或 page 缺 path，由现有 access contract 在更早阶段拒绝，不在此兼容。

- [ ] **Step 4: 运行测试与类型检查**

Run:

```powershell
pnpm vitest run src/layout/breadcrumbs.test.ts
pnpm exec vue-tsc -b --pretty false
```

Expected: PASS。

---

### Task 5: SettingDrawer 与 Header

**Files:**

- Create: `web/src/layout/components/SettingDrawer.vue`
- Create: `web/src/layout/components/SettingDrawer.test.ts`
- Modify: `web/src/layout/components/AppHeader.vue`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`

- [ ] **Step 1: 写设置抽屉红灯测试**

测试用 Pinia 挂载抽屉，断言：

```ts
it('updates theme, primary color and real layout switches', async () => {
  const wrapper = mountDrawer()
  await wrapper.get('[data-testid="theme-dark"]').trigger('click')
  expect(store.preferences.theme).toBe('dark')

  wrapper.findComponent({ name: 'ElColorPicker' }).vm.$emit('change', '#059669')
  expect(store.preferences.primaryColor).toBe('#059669')

  await wrapper.get('[data-testid="show-footer"]').trigger('click')
  expect(store.preferences.showFooter).toBe(false)
})

it('shows persistent storage errors and resets only UI preferences', async () => {
  store.persistenceError = 'invalid'
  const wrapper = mountDrawer()
  expect(wrapper.get('[data-testid="ui-preferences-error"]').text()).toContain('配置')
  await wrapper.get('[data-testid="reset-ui-preferences"]').trigger('click')
  expect(store.persistenceError).toBeNull()
})
```

另测 `contentFullscreen=true` 时 `showRouteTabs` switch disabled。

- [ ] **Step 2: 运行抽屉测试确认失败**

Run: `pnpm vitest run src/layout/components/SettingDrawer.test.ts`

Expected: FAIL，组件不存在。

- [ ] **Step 3: 加入中英文键**

在两个 message 文件中一一对应增加：

```ts
'layout.header.settings'
'layout.breadcrumb.missing'
'layout.settings.title'
'layout.settings.theme'
'layout.settings.light'
'layout.settings.dark'
'layout.settings.primaryColor'
'layout.settings.display'
'layout.settings.breadcrumb'
'layout.settings.menuToggle'
'layout.settings.routeTabs'
'layout.settings.uniqueOpened'
'layout.settings.footer'
'layout.settings.transition'
'layout.settings.transitionEnabled'
'layout.settings.transitionFade'
'layout.settings.transitionSlideLeft'
'layout.settings.transitionZoom'
'layout.settings.reset'
'layout.settings.invalidStorage'
'layout.settings.writeFailed'
'layout.settings.fullscreenTabsLocked'
```

`en-US.ts` 必须继续 `satisfies Record<AppMessageKey, string>`，不能漏键。

- [ ] **Step 4: 实现 SettingDrawer 的真实控件**

组件 props：

```ts
defineProps<{ modelValue: boolean; contentFullscreen: boolean }>()
defineEmits<{ 'update:modelValue': [value: boolean] }>()
```

抽屉结构固定为：Header `Setting + title`；主题模式二选一；七个预设色 swatch + `el-color-picker`；五个显示 switch；一个过渡 switch + 一个有限 `el-select`；底部重置按钮。所有变化只调用 `uiPreferences.update({ field: value })`，不直接写 DOM/localStorage。错误 alert 根据 `persistenceError` 映射 i18n。

使用 Element Plus icons：`Setting`, `Sunny`, `Moon`, `Check`, `RefreshRight`。预设颜色保留旧项目验证过的：

```ts
const themeColors = ['#409EFF', '#3B82F6', '#475569', '#059669', '#0891B2', '#7C3AED', '#EA580C'] as const
```

- [ ] **Step 5: 改造 AppHeader**

Props/Emits 调整为：

```ts
defineProps<{
  locale: AppLocale
  breadcrumbs: HeaderBreadcrumb[]
  showBreadcrumb: boolean
  showMenuToggle: boolean
  contentFullscreen: boolean
  username: string
  logoutPending: boolean
}>()

defineEmits<{
  toggleMenu: []
  changeLocale: [locale: AppLocale]
  logout: []
}>()
```

删除 Header 的直接明暗切换按钮。左侧渲染可选菜单按钮和 `el-breadcrumb`；右侧渲染语言、`data-testid="open-settings"` 设置按钮、用户名、退出。AppHeader 自己维护 `settingsOpen`，挂载 `<SettingDrawer v-model="settingsOpen" :content-fullscreen="contentFullscreen" />`。

将 Header/Drawer 样式放回各自 scoped style，使用 Element Plus 变量；按钮 34px，圆角 10px，Header 56px，面包屑末项 700 字重。

- [ ] **Step 6: 运行抽屉测试与 i18n 类型检查**

Run:

```powershell
pnpm vitest run src/layout/components/SettingDrawer.test.ts
pnpm exec vue-tsc -b --pretty false
```

Expected: PASS。

---

### Task 6: Layout 和 Aside 真实联动

**Files:**

- Modify: `web/src/layout/index.vue`
- Modify: `web/src/layout/index.test.ts`
- Modify: `web/src/layout/components/AppAside.vue`
- Modify: `web/src/layout/components/AccessMenuNode.vue`
- Modify: `web/src/layout/components/AccessMenuNode.test.ts`
- Modify: `web/src/styles/index.scss`

- [ ] **Step 1: 更新 Layout 测试为新契约并确认红灯**

删除旧 `toggle-theme` 测试，新增：

```ts
it('renders Dashboard breadcrumbs without repeating a page title in Header', async () => {
  const { wrapper } = await mountLayout()
  expect(wrapper.findAllComponents({ name: 'ElBreadcrumbItem' })).toHaveLength(1)
  expect(wrapper.get('.app-header__breadcrumb').text()).toBe('工作台')
})

it('renders directory and leaf breadcrumbs from the RBAC menu tree', async () => {
  // mountLayout('/system/users') with access tree and a registered test route
  expect(wrapper.get('.app-header__breadcrumb').text()).toContain('系统管理')
  expect(wrapper.get('.app-header__breadcrumb').text()).toContain('用户管理')
})

it('applies every display preference to real shell regions', async () => {
  ui.update({ showRouteTabs: false, showFooter: false, showBreadcrumb: false })
  await nextTick()
  expect(wrapper.find('.admin-layout__tabs').exists()).toBe(false)
  expect(wrapper.find('.admin-layout__footer').exists()).toBe(false)
  expect(wrapper.find('.app-header__breadcrumb').exists()).toBe(false)
})

it('expands the aside before hiding its toggle', async () => {
  await wrapper.get('[data-testid="toggle-menu"]').trigger('click')
  ui.update({ showMenuToggle: false })
  await nextTick()
  expect(wrapper.get('[data-testid="app-aside"]').attributes('data-collapsed')).toBe('false')
})

it('keeps RouteTabs mounted as the fullscreen exit path', async () => {
  ui.update({ showRouteTabs: false })
  // enter fullscreen via existing RouteTabs before hiding, then assert tabs remain while fullscreen
})
```

同时为 `persistenceError` 和未知动态路径增加不可关闭 `el-alert` 断言。

- [ ] **Step 2: 运行 Layout 测试确认失败**

Run: `pnpm vitest run src/layout/index.test.ts`

Expected: FAIL，缺新 props/面包屑/偏好联动。

- [ ] **Step 3: 将偏好和面包屑接入 Layout**

在 `layout/index.vue`：

```ts
const uiPreferences = useUIPreferencesStore()
const breadcrumbs = computed(() => resolveBreadcrumbs(route.path, access.menuTree) ?? [])
const breadcrumbMissing = computed(() => (
  route.meta.requiresAuth === true
  && route.path !== '/dashboard'
  && access.status === 'ready'
  && breadcrumbs.value.length === 0
))

watch(() => uiPreferences.preferences.showMenuToggle, (visible) => {
  if (!visible) collapsed.value = false
})
```

Aside 宽度改为展开 `248px`、折叠 `80px`。Header 传入新 props。RouteTabs 条件为：

```vue
<div v-if="uiPreferences.preferences.showRouteTabs || contentFullscreen" class="admin-layout__tabs ...">
```

桌面和移动端两个 AppAside 都传入 `:unique-opened="uiPreferences.preferences.uniqueOpened"`，复用同一份 access tree。Footer 同时检查 `showFooter` 和 `!contentFullscreen`。Main 顶部显示 access error、preference error、breadcrumb missing 三种明确 alert；每种使用独立 testid。

- [ ] **Step 4: 接入路由过渡但保持单一 RouterView 数据流**

将 RouterView 改为 slot：

```vue
<RouterView v-slot="{ Component }">
  <Transition
    v-if="uiPreferences.preferences.pageTransition"
    :name="uiPreferences.preferences.transitionName"
    mode="out-in"
  >
    <component :is="Component" :key="`${route.fullPath}::${refreshKey}`" />
  </Transition>
  <component v-else :is="Component" :key="`${route.fullPath}::${refreshKey}`" />
</RouterView>
```

在全局样式中定义 `fade`、`slide-left`、`zoom` 三组进入/离开状态，并继续由已有 `prefers-reduced-motion` 规则压缩时长。

- [ ] **Step 5: 对齐 Aside 结构和样式**

AppAside 新 prop：`uniqueOpened: boolean`，传给 `el-menu :unique-opened="uniqueOpened"`。Aside 使用：外层 `padding: 14px 12px 12px; gap: 10px`；品牌最小高度 58px；菜单 `flex:1; min-height:0; overflow-y:auto; padding:4px 0`；菜单项 44px、圆角 12px、子菜单左侧缩进 34px；折叠态居中图标。

AccessMenuNode 只补 `class="app-aside__menu-node"` 等稳定结构类，不改变目录/page 判断和 `node.path` 路由。

- [ ] **Step 6: Main 统一 8px padding 和唯一滚动所有权**

`styles/index.scss` 中布局核心值调整为：

```scss
.admin-layout__main {
  flex: 1 1 auto;
  min-width: 0;
  min-height: 0;
  padding: 8px;
  overflow: auto;
  background: var(--admin-page);
}

.admin-layout__tabs {
  min-width: 0;
  flex: 0 0 41px;
  overflow: hidden;
  background: var(--admin-surface-soft);
  border-bottom: 1px solid var(--admin-border);
}

@media (max-width: 840px) {
  .admin-layout__main { padding: 10px; }
}
```

继续保留 `html, body, #app { overflow: hidden; }`；从全局样式删除已经迁入 AppHeader/AppAside scoped style 的重复规则，避免 specificity 相互覆盖。

- [ ] **Step 7: 运行 Layout/Aside/RouteTabs 测试**

Run:

```powershell
pnpm vitest run src/layout/index.test.ts src/layout/components/AccessMenuNode.test.ts src/layout/components/RouteTabs.test.ts
pnpm exec vue-tsc -b --pretty false
```

Expected: PASS。

---

### Task 7: 系统页面公共内容面

**Files:**

- Create: `web/src/styles/system-pages.scss`
- Modify: `web/src/styles/index.scss`

- [ ] **Step 1: 实现最小公共系统页面样式**

`system-pages.scss` 只定义真实重复结构：

```scss
.system-page {
  display: flex;
  min-width: 0;
  min-height: 100%;
  padding: 16px;
  flex-direction: column;
  gap: 12px;
  background: var(--admin-surface);
  border: 1px solid var(--admin-border);
  border-radius: 8px;
}

.system-page__filters,
.system-page__actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.system-page__actions { justify-content: flex-end; }
.system-page > .app-table { min-height: 0; }

@media (max-width: 768px) {
  .system-page { min-height: 100%; padding: 12px; }
  .system-page__filters,
  .system-page__actions { align-items: stretch; }
}
```

在 `index.scss` 顶部 `@use './system-pages';`。不要创建卡片套卡片样式，不给 Dashboard/Auth 套此 class。

- [ ] **Step 2: 运行构建确认 SCSS 生效**

Run: `pnpm build`

Expected: build PASS；不能有 Sass selector 或 import 错误。

`system-page` 的组件级契约由 Task 8 和 Task 9 的六个页面红灯测试覆盖；Main 的 `admin-layout__scroll-owner` 契约由 Task 6 的 Layout 回归继续覆盖。jsdom 不用于伪造 CSS 像素布局断言。

---

### Task 8: 角色、用户和认证平台页面去标题

**Files:**

- Modify: `web/src/views/system/roles/index.vue`
- Modify: `web/src/views/system/roles/index.test.ts`
- Modify: `web/src/views/system/users/index.vue`
- Modify: `web/src/views/system/users/index.test.ts`
- Modify: `web/src/views/system/auth-platforms/index.vue`
- Modify: `web/src/views/system/auth-platforms/index.test.ts`

- [ ] **Step 1: 给三个页面添加“无重复 h1、操作仍存在”失败测试**

每个页面测试增加：

```ts
expect(wrapper.find('h1').exists()).toBe(false)
expect(wrapper.classes()).toContain('system-page')
```

并保留刷新/新增按钮断言。例如角色页仍断言 `role.refresh` 按钮和 `canCreate` 控制的 create 按钮；用户页刷新仍调用 `loadUsers`；认证平台仍保留 `auth-platform-refresh/create` testid。

- [ ] **Step 2: 运行三个页面测试确认红灯**

Run:

```powershell
pnpm vitest run src/views/system/roles/index.test.ts src/views/system/users/index.test.ts src/views/system/auth-platforms/index.test.ts
```

Expected: FAIL，仍存在 `<h1>` / 缺 `system-page`。

- [ ] **Step 3: 改造三个模板**

三个根节点都添加 `system-page`。删除只展示标题的 `<h1>`。

角色和认证平台：将刷新/新增按钮放入对应 `AppTable` 的 `#toolbar-right`；筛选区添加 `system-page__filters`。用户页把刷新按钮放入 AppTable `#toolbar-right`。示例：

```vue
<section class="role-page system-page">
  <div class="role-filters system-page__filters">...</div>
  <AppTable ...>
    <template #toolbar-right>
      <el-button :icon="Refresh" @click="loadRoles">{{ t('role.refresh') }}</el-button>
      <el-button v-if="canCreate" type="primary" :icon="CirclePlus" @click="openCreate">
        {{ t('role.create') }}
      </el-button>
    </template>
    ...
  </AppTable>
</section>
```

认证平台的 deployment `<h2>` 是真实分区标题，保留。不要改变 API、permissions、表格列、pagination 或 dialog。

- [ ] **Step 4: 删除三个页面已失效的局部 toolbar/title CSS**

删除 `.role-toolbar h1`、`.user-toolbar h1`、`.auth-platform-toolbar h1` 和只为旧标题行存在的响应式规则。保留筛选输入宽度、dialog、权限矩阵和 deployment 专用样式。

- [ ] **Step 5: 运行页面测试和类型检查**

Run:

```powershell
pnpm vitest run src/views/system/roles/index.test.ts src/views/system/users/index.test.ts src/views/system/auth-platforms/index.test.ts
pnpm exec vue-tsc -b --pretty false
```

Expected: PASS。

---

### Task 9: 菜单、会话和操作日志页面去标题

**Files:**

- Modify: `web/src/views/system/menus/index.vue`
- Modify: `web/src/views/system/menus/index.test.ts`
- Modify: `web/src/views/system/sessions/index.vue`
- Modify: `web/src/views/system/sessions/index.test.ts`
- Modify: `web/src/views/system/operation-logs/index.vue`
- Modify: `web/src/views/system/operation-logs/index.test.ts`

- [ ] **Step 1: 更新菜单页已有标题测试并为三个页面补红灯**

将菜单页：

```ts
expect(wrapper.get('#menu-management-title').text()).toBe('菜单管理')
```

改为：

```ts
expect(wrapper.find('h1').exists()).toBe(false)
expect(wrapper.get('.menu-management-page').classes()).toContain('system-page')
expect(wrapper.get('[data-testid="add-root-menu"]').exists()).toBe(true)
```

会话与日志同样断言无 `<h1>`，同时保留 batch revoke、refresh、search 和 reset 行为断言。

- [ ] **Step 2: 运行三个页面测试确认红灯**

Run:

```powershell
pnpm vitest run src/views/system/menus/index.test.ts src/views/system/sessions/index.test.ts src/views/system/operation-logs/index.test.ts
```

Expected: FAIL，标题仍存在。

- [ ] **Step 3: 改造菜单页**

根节点改为：

```vue
<section class="menu-management-page system-page" :aria-label="t('menu.title')">
  <div class="menu-management__toolbar-actions system-page__actions">
    <!-- add root + refresh，权限条件不变 -->
  </div>
  <div class="menu-management__content">...</div>
</section>
```

删除 heading icon、`aria-labelledby` 和 `<h1>`。将 `menu-management__content` padding 改为 `0`，避免 Main 8 + system page 16 + content 20 的三重间距。树表、Drawer、expanded IDs 和 mutation flow 不变。

- [ ] **Step 4: 改造会话页**

删除 `<h1>`，将 batch revoke/refresh 放入 `.session-toolbar-actions.system-page__actions`；保留 stats 作为真实业务概览。根节点加入 `system-page`，filters 加 `system-page__filters`。AppTable 的 selection 和 revoke 行为不变。

- [ ] **Step 5: 改造操作日志页**

删除完整标题 header，将 refresh 放到 filters 末尾：查询、重置、刷新仍全部可见。根节点加入 `system-page`，filters 加 `system-page__filters`。展开详情中的 `<h2>/<h3>` 是真实内容层级，保留。

- [ ] **Step 6: 清理三个页面失效样式并验证**

删除 toolbar `justify-content`、toolbar h1 和相关移动端规则；保留菜单树、session stats、log detail 专用样式。

Run:

```powershell
pnpm vitest run src/views/system/menus/index.test.ts src/views/system/sessions/index.test.ts src/views/system/operation-logs/index.test.ts
pnpm exec vue-tsc -b --pretty false
```

Expected: PASS。

---

### Task 10: 跨页面验收与技术约束扫描

**Files:**

- Verify all modified frontend files
- Update only failing tests that still encode the superseded, approved title/layout contract

- [ ] **Step 1: 运行布局和六个页面的集中回归**

Run:

```powershell
cd D:\admin\web
pnpm vitest run src/layout src/views/system/roles src/views/system/users src/views/system/menus src/views/system/auth-platforms src/views/system/sessions src/views/system/operation-logs
```

Expected: PASS，且没有 Vue warning、未处理 promise 或焦点/Teleport 污染。

- [ ] **Step 2: 扫描重复标题和 TypeScript 绕过**

Run:

```powershell
rg -n "<h1" src/views/system/roles src/views/system/users src/views/system/menus src/views/system/auth-platforms src/views/system/sessions src/views/system/operation-logs
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>|@ts-ignore" src -g "*.ts" -g "*.vue"
```

Expected: 两条命令均无输出。Dashboard/Auth 的 `<h1>` 不在扫描范围内并继续保留。

- [ ] **Step 3: 运行完整前端测试**

Run: `pnpm vitest run`

Expected: 48 个或更多 test files 全部 PASS，0 failed。记录实际文件数和测试数，不沿用旧数字。

- [ ] **Step 4: 运行生产构建**

Run: `pnpm build`

Expected: `vue-tsc -b` 与 Vite build exit 0。现有 chunk-size warning 可以记录，但不能出现新增编译错误。

- [ ] **Step 5: 最终 diff 与边界检查**

Run:

```powershell
cd D:\admin
git diff --check
git status --short
git diff --stat
```

确认：

- 没有后端、数据库、RBAC/API 契约改动；
- 没有 Docker 文件；
- 没有启动、停止用户服务；
- 没有 commit/push；
- 没有覆盖本任务开始前的 AppTable/AppDialog 和页面未提交改动；
- 完成报告列出实际验证结果和仍存在的构建 warning。
