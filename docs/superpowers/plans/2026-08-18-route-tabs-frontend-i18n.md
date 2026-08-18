# Route Tabs and Frontend i18n Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the protected admin shell with a directly understandable route-tab bar, internal main scrolling, and typed Chinese/English frontend internationalization before RBAC work starts.

**Architecture:** Keep the existing Vue Router, Pinia auth store, and layout components as the only data flow. `RouteTabs` owns an in-memory visited-tab list and emits only refresh/fullscreen commands; `AdminLayout` owns RouterView remounting and shell visibility. A small `vue-i18n` setup provides flat, statically checked messages without copying the reference project's menu store, persistence kernel, or dynamic route registry.

**Tech Stack:** Vue 3 Composition API, TypeScript 6, Vue Router 4, Element Plus, vue-i18n 11, Pinia, SCSS, Vitest, Vue Test Utils, Vite.

---

## Execution Rules

- Work directly on the current `main` checkout. Do not create a worktree or use subagents.
- Do not start or stop the API, Worker, PostgreSQL, Redis, or frontend services.
- Do not commit, fetch, pull, or push. Keep changes inspectable with `git diff`.
- Preserve current authentication/API contracts and registration source/API files.
- TypeScript must not contain `any`, `as any`, `@ts-ignore`, or broad untyped event payloads.
- Use Element Plus icons, tooltips, accessible labels, and `--el-*` theme variables.
- Keep data flow linear: route -> RouteTabs -> router/layout command. No event bus, tab store, adapter, manager, factory, or global component registry.

## File Map

### Create

```text
web/src/i18n/index.ts
web/src/i18n/index.test.ts
web/src/i18n/messages/zh-CN.ts
web/src/i18n/messages/en-US.ts
web/src/layout/components/RouteTabs.vue
web/src/layout/components/RouteTabs.test.ts
```

### Modify

```text
web/package.json
web/pnpm-lock.yaml
web/src/main.ts
web/src/router/index.ts
web/src/router/index.test.ts
web/src/layout/index.vue
web/src/layout/index.test.ts
web/src/layout/components/AppHeader.vue
web/src/layout/components/AppAside.vue
web/src/layout/components/AppFooter.vue
web/src/views/auth/login/index.vue
web/src/views/auth/login/index.test.ts
web/src/views/dashboard/index.vue
web/src/views/dashboard/index.test.ts
web/src/styles/index.scss
```

## Task 1: Install and define the typed i18n foundation

**Files:**
- Modify: `web/package.json`
- Modify: `web/pnpm-lock.yaml`
- Create: `web/src/i18n/messages/zh-CN.ts`
- Create: `web/src/i18n/messages/en-US.ts`
- Create: `web/src/i18n/index.ts`
- Create: `web/src/i18n/index.test.ts`

- [ ] **Step 1: Add the one required dependency**

Run from `D:\admin\web`:

```powershell
pnpm add vue-i18n@^11.1.0
```

Expected: only `vue-i18n` and its lockfile entries are added.

- [ ] **Step 2: Write failing locale tests**

Create `web/src/i18n/index.test.ts`:

```ts
import { beforeEach, describe, expect, it } from 'vitest'

import { appI18n, initializeLocale, localeStorageKey, readLocale, setLocale } from './index'
import { enUS } from './messages/en-US'
import { zhCN } from './messages/zh-CN'

describe('frontend i18n', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.lang = ''
    setLocale('zh-CN')
  })

  it('defaults to Chinese and updates the root language', () => {
    expect(initializeLocale()).toBe('zh-CN')
    expect(readLocale()).toBe('zh-CN')
    expect(document.documentElement.lang).toBe('zh-CN')
    expect(appI18n.global.t('navigation.dashboard')).toBe('工作台')
  })

  it('normalizes an invalid stored locale to Chinese', () => {
    localStorage.setItem(localeStorageKey, 'fr-FR')
    expect(initializeLocale()).toBe('zh-CN')
    expect(localStorage.getItem(localeStorageKey)).toBe('zh-CN')
  })

  it('persists English and keeps both catalogs exactly shaped', () => {
    setLocale('en-US')
    expect(localStorage.getItem(localeStorageKey)).toBe('en-US')
    expect(document.documentElement.lang).toBe('en-US')
    expect(appI18n.global.t('navigation.dashboard')).toBe('Dashboard')
    expect(Object.keys(enUS).sort()).toEqual(Object.keys(zhCN).sort())
  })
})
```

- [ ] **Step 3: Run the tests and verify the missing-module failure**

```powershell
pnpm vitest run src/i18n/index.test.ts
```

Expected: FAIL because the i18n modules do not exist.

- [ ] **Step 4: Create the flat Chinese catalog**

Create `web/src/i18n/messages/zh-CN.ts` with one readonly object and its key type:

```ts
export const zhCN = {
  'navigation.dashboard': '工作台',
  'navigation.main': '主导航',
  'navigation.admin': 'Admin',
  'layout.header.toggleMenu': '切换菜单',
  'layout.header.switchToLight': '切换为浅色主题',
  'layout.header.switchToDark': '切换为深色主题',
  'layout.header.switchLanguage': '切换语言',
  'layout.header.logout': '退出登录',
  'layout.footer': 'Admin 管理控制台 · 2026',
  'layout.routeTabs.previous': '上一个页面',
  'layout.routeTabs.next': '下一个页面',
  'layout.routeTabs.refresh': '刷新当前页面',
  'layout.routeTabs.fullscreen': '内容全屏',
  'layout.routeTabs.exitFullscreen': '退出内容全屏',
  'layout.routeTabs.close': '关闭当前页面',
  'layout.routeTabs.closeOthers': '关闭其他页面',
  'layout.routeTabs.closeAll': '关闭全部页面',
  'layout.routeTabs.contextMenu': '页面操作',
  'auth.login.eyebrow': 'ADMIN CONSOLE',
  'auth.login.title': '欢迎回来',
  'auth.login.heading': '让系统状态清楚可见',
  'auth.login.description': '统一管理服务状态、任务与后续权限配置。',
  'auth.login.caption': '使用已授权的用户名和密码进入管理台。',
  'auth.login.username': '用户名',
  'auth.login.usernamePlaceholder': '请输入用户名',
  'auth.login.password': '密码',
  'auth.login.passwordPlaceholder': '请输入密码',
  'auth.login.submit': '登录管理台',
  'auth.login.authorizedOnly': '仅限已授权账号访问',
  'auth.login.usernameRequired': '请输入用户名',
  'auth.login.passwordRequired': '请输入密码',
  'auth.login.invalidCredentials': '用户名或密码错误',
  'auth.login.failed': '登录失败',
  'auth.login.bootstrapFailed': '认证服务响应异常',
  'auth.logoutFailed': '退出登录失败',
  'dashboard.eyebrow': 'SYSTEM OVERVIEW',
  'dashboard.title': '工作台',
  'dashboard.refresh': '刷新状态',
  'dashboard.liveStatus': '实时状态',
  'dashboard.dependencies': '依赖状态',
  'dashboard.live': '实时',
  'dashboard.asynq': 'ASYNQ',
  'dashboard.exampleTask': '示例任务',
  'dashboard.async': '异步',
  'dashboard.message': '消息内容',
  'dashboard.messagePlaceholder': '输入任务消息',
  'dashboard.submitTask': '投递任务',
  'dashboard.taskId': '任务 ID',
  'dashboard.api': 'API',
  'dashboard.postgresql': 'PostgreSQL',
  'dashboard.redis': 'Redis',
  'dashboard.status.up': '运行正常',
  'dashboard.status.error': '检查失败',
  'dashboard.status.checking': '检查中',
  'dashboard.healthInvalid': 'API 状态无效',
  'dashboard.unknownError': '状态检查返回未知错误',
  'dashboard.taskInvalid': '任务响应无效',
} as const

export type AppMessageKey = keyof typeof zhCN
```

- [ ] **Step 5: Create the English catalog with compile-time key equality**

Create `web/src/i18n/messages/en-US.ts` with the same keys and this type contract:

```ts
import type { AppMessageKey } from './zh-CN'

export const enUS = {
  'navigation.dashboard': 'Dashboard',
  'navigation.main': 'Main navigation',
  'navigation.admin': 'Admin',
  'layout.header.toggleMenu': 'Toggle menu',
  'layout.header.switchToLight': 'Switch to light theme',
  'layout.header.switchToDark': 'Switch to dark theme',
  'layout.header.switchLanguage': 'Switch language',
  'layout.header.logout': 'Sign out',
  'layout.footer': 'Admin Console · 2026',
  'layout.routeTabs.previous': 'Previous page',
  'layout.routeTabs.next': 'Next page',
  'layout.routeTabs.refresh': 'Refresh current page',
  'layout.routeTabs.fullscreen': 'Content fullscreen',
  'layout.routeTabs.exitFullscreen': 'Exit content fullscreen',
  'layout.routeTabs.close': 'Close current page',
  'layout.routeTabs.closeOthers': 'Close other pages',
  'layout.routeTabs.closeAll': 'Close all pages',
  'layout.routeTabs.contextMenu': 'Page actions',
  'auth.login.eyebrow': 'ADMIN CONSOLE',
  'auth.login.title': 'Welcome back',
  'auth.login.heading': 'See system status clearly',
  'auth.login.description': 'Manage service status, tasks, and future permissions in one place.',
  'auth.login.caption': 'Use an authorized username and password to enter the console.',
  'auth.login.username': 'Username',
  'auth.login.usernamePlaceholder': 'Enter username',
  'auth.login.password': 'Password',
  'auth.login.passwordPlaceholder': 'Enter password',
  'auth.login.submit': 'Sign in to console',
  'auth.login.authorizedOnly': 'Authorized accounts only',
  'auth.login.usernameRequired': 'Username is required',
  'auth.login.passwordRequired': 'Password is required',
  'auth.login.invalidCredentials': 'Incorrect username or password',
  'auth.login.failed': 'Sign-in failed',
  'auth.login.bootstrapFailed': 'Authentication service returned an unexpected response',
  'auth.logoutFailed': 'Sign-out failed',
  'dashboard.eyebrow': 'SYSTEM OVERVIEW',
  'dashboard.title': 'Dashboard',
  'dashboard.refresh': 'Refresh status',
  'dashboard.liveStatus': 'Live status',
  'dashboard.dependencies': 'Dependencies',
  'dashboard.live': 'Live',
  'dashboard.asynq': 'ASYNQ',
  'dashboard.exampleTask': 'Example task',
  'dashboard.async': 'Async',
  'dashboard.message': 'Message',
  'dashboard.messagePlaceholder': 'Enter task message',
  'dashboard.submitTask': 'Submit task',
  'dashboard.taskId': 'Task ID',
  'dashboard.api': 'API',
  'dashboard.postgresql': 'PostgreSQL',
  'dashboard.redis': 'Redis',
  'dashboard.status.up': 'Operational',
  'dashboard.status.error': 'Check failed',
  'dashboard.status.checking': 'Checking',
  'dashboard.healthInvalid': 'API status is invalid',
  'dashboard.unknownError': 'Status check returned an unknown error',
  'dashboard.taskInvalid': 'Task response is invalid',
} satisfies Record<AppMessageKey, string>
```

- [ ] **Step 6: Implement locale selection and the single i18n instance**

Create `web/src/i18n/index.ts`:

```ts
import { createI18n } from 'vue-i18n'

import { enUS } from './messages/en-US'
import { zhCN, type AppMessageKey } from './messages/zh-CN'

export type AppLocale = 'zh-CN' | 'en-US'
export type { AppMessageKey }
export const localeStorageKey = 'admin:locale'

function isAppLocale(value: string | null): value is AppLocale {
  return value === 'zh-CN' || value === 'en-US'
}

export function readLocale(): AppLocale {
  const stored = window.localStorage.getItem(localeStorageKey)
  return isAppLocale(stored) ? stored : 'zh-CN'
}

export const appI18n = createI18n({
  legacy: false,
  locale: readLocale(),
  fallbackLocale: false,
  flatJson: true,
  messages: { 'zh-CN': zhCN, 'en-US': enUS },
})

export function setLocale(locale: AppLocale): void {
  appI18n.global.locale.value = locale
  window.localStorage.setItem(localeStorageKey, locale)
  document.documentElement.lang = locale
}

export function initializeLocale(): AppLocale {
  const locale = readLocale()
  setLocale(locale)
  return locale
}
```

- [ ] **Step 7: Verify the foundation**

```powershell
pnpm vitest run src/i18n/index.test.ts
pnpm exec vue-tsc -b --pretty false
```

Expected: locale tests pass and there are no TypeScript diagnostics.

## Task 2: Register i18n and migrate current user-visible text

**Files:**
- Modify: `web/src/main.ts`
- Modify: `web/src/layout/index.vue`
- Modify: `web/src/layout/components/AppHeader.vue`
- Modify: `web/src/layout/components/AppAside.vue`
- Modify: `web/src/layout/components/AppFooter.vue`
- Modify: `web/src/views/auth/login/index.vue`
- Modify: `web/src/views/dashboard/index.vue`
- Modify: `web/src/layout/index.test.ts`
- Modify: `web/src/views/auth/login/index.test.ts`
- Modify: `web/src/views/dashboard/index.test.ts`

- [ ] **Step 1: Add failing UI locale tests**

Add a layout test that clicks `[data-testid="locale-switch-en"]`, then asserts `document.documentElement.lang === 'en-US'` and the Dashboard title is `Dashboard`. Add Login assertions for the Chinese username rule `请输入用户名` and Dashboard assertions for translated status text after successful health responses.

- [ ] **Step 2: Run focused tests and observe the intended failures**

```powershell
pnpm vitest run src/layout/index.test.ts src/views/auth/login/index.test.ts src/views/dashboard/index.test.ts
```

Expected: the new selector/translation assertions fail because components still contain literal UI strings.

- [ ] **Step 3: Register and initialize i18n before mounting Vue**

In `main.ts`, keep theme and permission initialization, then use the i18n instance:

```ts
import { appI18n, initializeLocale } from './i18n'

initializeLocale()
createApp(App).use(pinia).use(router).use(ElementPlus).use(appI18n).mount('#app')
```

- [ ] **Step 4: Add the typed Header language dropdown**

Use `useI18n`, `Translate`, and a typed `changeLocale` emit. The dropdown must contain these stable controls:

```vue
<el-dropdown @command="handleLocaleCommand">
  <el-button data-testid="locale-switch" text :icon="Translate" :aria-label="t('layout.header.switchLanguage')" />
  <template #dropdown>
    <el-dropdown-menu>
      <el-dropdown-item command="zh-CN" data-testid="locale-switch-zh">中文</el-dropdown-item>
      <el-dropdown-item command="en-US" data-testid="locale-switch-en">English</el-dropdown-item>
    </el-dropdown-menu>
  </template>
</el-dropdown>
```

Define `defineEmits<{ changeLocale: [locale: AppLocale] }>()` and have `handleLocaleCommand(command: AppLocale): void` emit it. Do not cast the command to `any`.

- [ ] **Step 5: Own the locale ref at the layout boundary**

Use `readLocale()` for a typed `locale` ref and `setLocale(locale)` in `handleLocaleChange`. Pass the locale to `AppHeader`; keep theme, collapse, mobile Drawer, logout, and auth behavior unchanged.

- [ ] **Step 6: Replace client-owned literals with `t()`**

Migrate Header, Aside, Footer, Login, and Dashboard headings, labels, placeholders, tooltips, validation messages, status labels, and client-generated errors. Preserve server-provided `ApiError.message` unchanged, all current API payloads, and all existing `data-testid` hooks.

- [ ] **Step 7: Verify migration**

```powershell
pnpm vitest run src/i18n/index.test.ts src/layout/index.test.ts src/views/auth/login/index.test.ts src/views/dashboard/index.test.ts
rg -n "工作台|请输入用户名|登录管理台|刷新状态|退出登录|运行正常|检查失败" web/src -g "*.vue" -g "*.ts"
```

Expected: tests pass and the search finds these strings only in message catalogs or intentional test expectations.

## Task 3: Add typed route metadata for translated tabs

**Files:**
- Modify: `web/src/router/index.ts`
- Modify: `web/src/router/index.test.ts`

- [ ] **Step 1: Add metadata assertions before implementation**

Add this test:

```ts
it('declares a translated fixed title for Dashboard', () => {
  const router = createAppRouter(createMemoryHistory())
  const dashboard = router.resolve('/dashboard')
  expect(dashboard.meta.requiresAuth).toBe(true)
  expect(dashboard.meta.titleKey).toBe('navigation.dashboard')
  expect(dashboard.meta.affix).toBe(true)
  expect(router.resolve('/login').meta.titleKey).toBeUndefined()
})
```

- [ ] **Step 2: Run the router test and verify the metadata failure**

```powershell
pnpm vitest run src/router/index.test.ts
```

Expected: the Dashboard title/affix assertions fail.

- [ ] **Step 3: Extend `RouteMeta` with the typed message key**

Add to `router/index.ts`:

```ts
import type { AppMessageKey } from '../i18n/messages/zh-CN'

declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth: boolean
    titleKey?: AppMessageKey
    affix?: boolean
  }
}
```

Set Dashboard metadata to `{ requiresAuth: true, titleKey: 'navigation.dashboard', affix: true }`. Keep Login public and without tab metadata; add no fake business routes.

- [ ] **Step 4: Verify route metadata and the existing guard**

```powershell
pnpm vitest run src/router/index.test.ts
pnpm exec vue-tsc -b --pretty false
```

Expected: all router tests pass and metadata values are compile-time checked.

## Task 4: Build the linear `RouteTabs` component with TDD

**Files:**
- Create: `web/src/layout/components/RouteTabs.vue`
- Create: `web/src/layout/components/RouteTabs.test.ts`

- [ ] **Step 1: Write focused tab behavior tests**

Use an in-memory router fixture with `/dashboard`, `/users`, and `/roles` protected leaf routes carrying `titleKey` metadata. Stub the view with `data-testid="route-view"` and cover these behaviors:

```ts
it('adds each visited leaf once and keeps Dashboard fixed', async () => {
  const { wrapper, router } = await mountTabs('/dashboard')
  await router.push('/users'); await flushPromises()
  await router.push('/users'); await flushPromises()
  expect(wrapper.findAll('[data-testid="route-tab"]')).toHaveLength(2)
  expect(wrapper.get('[data-testid="route-tab-dashboard"]').attributes('data-affix')).toBe('true')
  expect(wrapper.get('[data-testid="route-tab-dashboard-close"]').exists()).toBe(false)
})

it('closes the active tab and selects the nearest remaining tab', async () => {
  const { wrapper, router } = await mountTabs('/dashboard')
  await router.push('/users'); await router.push('/roles'); await flushPromises()
  await wrapper.get('[data-testid="route-tab-roles-close"]').trigger('click')
  await flushPromises()
  expect(router.currentRoute.value.path).toBe('/users')
})

it('close others and close all retain Dashboard', async () => {
  const { wrapper, router } = await mountTabs('/dashboard')
  await router.push('/users'); await router.push('/roles'); await flushPromises()
  await wrapper.get('[data-testid="route-tab-roles-menu"]').trigger('contextmenu')
  await wrapper.get('[data-testid="route-tabs-close-others"]').trigger('click')
  expect(wrapper.findAll('[data-testid="route-tab"]')).toHaveLength(2)
  await wrapper.get('[data-testid="route-tabs-close-all"]').trigger('click')
  await flushPromises()
  expect(router.currentRoute.value.path).toBe('/dashboard')
  expect(wrapper.findAll('[data-testid="route-tab"]')).toHaveLength(1)
})

it('emits refresh and fullscreen commands', async () => {
  const { wrapper } = await mountTabs('/dashboard')
  await wrapper.get('[data-testid="route-tabs-refresh"]').trigger('click')
  await wrapper.get('[data-testid="route-tabs-fullscreen"]').trigger('click')
  expect(wrapper.emitted('refresh')).toHaveLength(1)
  expect(wrapper.emitted('toggleFullscreen')).toHaveLength(1)
})
```

Also assert previous/next disabled states, right-click menu commands, Escape/outside dismissal, active `data-active`, and `scrollIntoView` when the route changes. Stub `HTMLElement.prototype.scrollIntoView = vi.fn()` because jsdom has no implementation.

- [ ] **Step 2: Run the tests and verify the missing component failure**

```powershell
pnpm vitest run src/layout/components/RouteTabs.test.ts
```

Expected: FAIL because `RouteTabs.vue` does not exist.

- [ ] **Step 3: Implement the typed tab model and route watcher**

Start `RouteTabs.vue` with this direct state model:

```ts
interface RouteTab {
  path: string
  titleKey: AppMessageKey
  affix: boolean
}

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
```

Find the last matched record with a `titleKey`. If a protected leaf lacks one, throw `new Error('Route <path> must declare titleKey')`; do not invent a label. Watch `route.fullPath`, add `{ path: route.path, titleKey, affix: route.meta.affix === true }` once, and scroll the active tab into view on the next tick. Use smooth scrolling unless `matchMedia('(prefers-reduced-motion: reduce)').matches`.

- [ ] **Step 4: Implement navigation and close commands without a store**

Implement `navigateTo`, `closeTab`, `closeOthers`, `closeAll`, `navigateRelative`, `refreshCurrent`, `toggleFullscreen`, `openContextMenu`, and `dismissContextMenu` in the component. The close rules are explicit:

```ts
function closableTabs(): RouteTab[] {
  return tabs.value.filter((tab) => !tab.affix)
}

async function closeAll(): Promise<void> {
  tabs.value = tabs.value.filter((tab) => tab.affix)
  await router.push('/dashboard')
}
```

`closeTab` ignores affixed tabs, removes the path, and when closing the active tab navigates to the previous index or the next remaining tab, finally `/dashboard`. `closeOthers` retains every affixed tab and the selected/current tab. Context-menu actions call these same functions, then close the menu. Escape, document pointerdown outside the component, and route changes dismiss it. Add a tab only from the successful route watcher, never before `router.push` succeeds.

- [ ] **Step 5: Render stable controls and the context menu**

Render a horizontally scrollable `nav` with one button per tab and stable `data-testid="route-tab-${slug}"`, `data-active`, and `data-affix` attributes. Omit Dashboard's close button. Use Element Plus icon buttons with tooltips/accessible labels for previous, next, refresh, fullscreen, close-others, and close-all. Render the context menu with `role="menu"` and the same commands. Do not create a global registration or event-bus channel.

- [ ] **Step 6: Verify all RouteTabs behavior and strict types**

```powershell
pnpm vitest run src/layout/components/RouteTabs.test.ts
pnpm exec vue-tsc -b --pretty false
```

Expected: all tab tests pass with no TypeScript diagnostics.

## Task 5: Integrate RouteTabs, refresh remounting, and content fullscreen

**Files:**
- Modify: `web/src/layout/index.vue`
- Modify: `web/src/layout/index.test.ts`

- [ ] **Step 1: Add layout integration tests before wiring the component**

Add these tests. The RouterView fixture increments a module-local render counter and exposes it as `data-render`:

```ts
it('renders RouteTabs between Header and Main', async () => {
  const { wrapper } = await mountLayout()
  const order = wrapper.findAll('.admin-layout__workspace > *').map((node) => node.classes())
  expect(order).toEqual([
    ['admin-layout__header'],
    ['admin-layout__tabs'],
    ['admin-layout__main'],
    ['admin-layout__footer'],
  ])
})

it('remounts the current view and hides outer chrome in fullscreen', async () => {
  const { wrapper } = await mountLayout()
  const before = wrapper.get('[data-testid="layout-content"]').attributes('data-render')
  await wrapper.get('[data-testid="route-tabs-refresh"]').trigger('click')
  expect(wrapper.get('[data-testid="layout-content"]').attributes('data-render')).not.toBe(before)
  await wrapper.get('[data-testid="route-tabs-fullscreen"]').trigger('click')
  expect(wrapper.find('.admin-layout__aside').exists()).toBe(false)
  expect(wrapper.find('.admin-layout__header').exists()).toBe(false)
  expect(wrapper.find('.admin-layout__footer').exists()).toBe(false)
  expect(wrapper.find('.admin-layout__tabs').exists()).toBe(true)
  expect(wrapper.find('.admin-layout__main').exists()).toBe(true)
})
```

- [ ] **Step 2: Run layout tests and verify missing integration**

```powershell
pnpm vitest run src/layout/index.test.ts
```

Expected: FAIL because the layout has no tabs, refresh key, or fullscreen state.

- [ ] **Step 3: Wire direct layout state and `RouterView` keying**

In `layout/index.vue`, import `RouteTabs`, define `refreshKey = ref(0)` and `contentFullscreen = ref(false)`, and add:

```ts
function handleRefresh(): void {
  refreshKey.value += 1
}

function handleToggleFullscreen(): void {
  contentFullscreen.value = !contentFullscreen.value
}
```

Render an always-present `RouteTabs` between Header and Main. Render Aside/Header/Footer conditionally on `contentFullscreen`. Key the view as `:key="`${route.fullPath}::${refreshKey}`"`. Keep the existing mobile Drawer, auth/logout, theme, and collapse logic unchanged; do not move this state into Pinia.

- [ ] **Step 4: Verify layout behavior**

```powershell
pnpm vitest run src/layout/index.test.ts src/layout/components/RouteTabs.test.ts
pnpm exec vue-tsc -b --pretty false
```

Expected: existing shell behavior plus refresh/fullscreen integration pass.

## Task 6: Make Main the only vertical scroll owner and hide scrollbars

**Files:**
- Modify: `web/src/styles/index.scss`
- Modify: `web/src/layout/index.vue`
- Modify: `web/src/layout/index.test.ts`

- [ ] **Step 1: Add structural scroll ownership assertions**

Add this test:

```ts
it('exposes Main as the scroll owner and RouteTabs as horizontal scroll', async () => {
  const { wrapper } = await mountLayout()
  expect(wrapper.get('.admin-layout__main').classes()).toContain('admin-layout__scroll-owner')
  expect(wrapper.get('.admin-layout__tabs').classes()).toContain('admin-layout__horizontal-scroll')
})
```

- [ ] **Step 2: Implement fixed viewport and internal scrolling rules**

Update `styles/index.scss` with these rules while retaining existing visual rules:

```scss
html,
body,
#app {
  width: 100%;
  height: 100%;
  min-height: 100%;
  margin: 0;
  overflow: hidden;
}

.admin-layout {
  width: 100%;
  height: 100dvh;
  min-height: 0;
  overflow: hidden;
}

.admin-layout__workspace {
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}

.admin-layout__main {
  flex: 1 1 auto;
  min-width: 0;
  min-height: 0;
  overflow: auto;
}

.admin-layout__scroll-owner,
.admin-layout__horizontal-scroll {
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.admin-layout__scroll-owner::-webkit-scrollbar,
.admin-layout__horizontal-scroll::-webkit-scrollbar {
  display: none;
  width: 0;
  height: 0;
}
```

Apply `admin-layout__scroll-owner` to Main and `admin-layout__horizontal-scroll` to RouteTabs. Keep `min-height: 0` on flex ancestors and allow wheel, touch, keyboard, focus, and programmatic scrolling. Hide the Drawer scrollbar with the same shared rule.

- [ ] **Step 3: Verify the scroll contract**

```powershell
pnpm vitest run src/layout/index.test.ts src/layout/components/RouteTabs.test.ts
rg -n "overflow:|scrollbar-width|::-webkit-scrollbar|100dvh|admin-layout__scroll-owner" web/src/styles/index.scss web/src/layout -g "*.vue" -g "*.scss"
```

Expected: tests pass; the scan shows root overflow hidden, Main overflow auto, and hidden scrollbar rules without a second vertical page container.

## Task 7: Final migration, verification, and scope audit

**Files:**
- Verify every file listed in this plan

- [ ] **Step 1: Run focused application tests**

```powershell
cd D:\admin\web
pnpm vitest run src/i18n/index.test.ts src/router/index.test.ts src/layout/index.test.ts src/layout/components/RouteTabs.test.ts src/views/auth/login/index.test.ts src/views/dashboard/index.test.ts
```

Expected: all focused tests pass.

- [ ] **Step 2: Run the complete frontend suite**

```powershell
pnpm vitest run
```

Expected: every existing and new test passes; auth, health, task, request, theme, and registration-source tests remain green.

- [ ] **Step 3: Run strict types and the production build**

```powershell
pnpm exec vue-tsc -b --pretty false
pnpm build
```

Expected: both commands exit 0. Report any Vite chunk-size warning separately from failures.

- [ ] **Step 4: Scan forbidden TypeScript and copied architecture**

```powershell
cd D:\admin
rg -n "\bas any\b|@ts-ignore|Record<[^>]*,\s*any>|MenuStore|TabTag|TagsView|eventBus|EventBus|adapter|Adapter|manager|Manager|factory|Factory" web/src -g "*.ts" -g "*.vue"
```

Expected: no forbidden TypeScript or newly introduced architecture names. Review any pre-existing unrelated match manually before changing it.

- [ ] **Step 5: Check formatting and exact scope**

```powershell
git diff --check
git status --short --branch
git diff --stat
git diff -- web/package.json web/src/i18n web/src/router web/src/layout web/src/views/auth/login web/src/views/dashboard web/src/styles web/src/main.ts
```

Confirm no service was started, no commit/push occurred, authentication/API/request behavior is unchanged, and only this route-tabs/frontend-i18n slice is present.

## Self-Review Against the Approved Spec

- RouteTabs naming, all tab commands, fixed Dashboard, context menu, active-tab scrolling, reduced motion, and fullscreen are covered by Tasks 4-6.
- Linear data flow and the no-store/no-bus constraint are enforced by Tasks 4-5 and the final scan.
- Chinese default, English switch, persistence, root `lang`, equal typed message keys, and disabled fallback are covered by Tasks 1-2.
- Login, layout, Dashboard, validation, and client-generated errors migrate in Task 2; server messages stay unchanged.
- Typed `titleKey`/`affix` metadata is covered by Task 3.
- Root overflow ownership and hidden-but-active scrolling are covered by Task 6.
- Backend i18n, RBAC localization, authentication changes, and service startup remain outside this slice.

No placeholders, TODO markers, or undefined task references are used. Function names, message keys, route metadata names, selectors, and file paths are consistent across tasks.
