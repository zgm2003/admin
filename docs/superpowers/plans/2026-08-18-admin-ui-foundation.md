# Admin UI Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the existing login flow, protected Element Plus shell, and dashboard one responsive light/dark visual system while removing the visible registration entry.

**Architecture:** Element Plus CSS variables remain the source of truth. A small concrete `utils/theme.ts` owns only reading, applying, and persisting `light | dark`; layout components consume that state directly, while all existing Auth/API/router data flows remain unchanged. The reference project contributes visual patterns only, not its application architecture.

**Tech Stack:** Vue 3, TypeScript 6, Element Plus, SCSS, Vue Router, Pinia, Vitest, Vue Test Utils, ECharts.

---

## Execution Rules

- Work directly in the current workspace; do not use a worktree or subagent.
- Do not start or stop the user's API, Worker, or frontend processes.
- Do not commit, fetch, pull, or push. Use diff checkpoints because Git history belongs to the owner.
- Preserve the current login API contract `{ username, password }` and all Auth Store behavior.
- Keep the registration API and source files; remove only the visible route and login entry.
- Do not introduce `any`, broad type assertions, a UI framework wrapper, or copied reference-project architecture.
- Use Element Plus icons and variables. Do not create custom SVG icons or a second independent palette.

## File Map

### Create

```text
web/src/utils/theme.ts
web/src/utils/theme.test.ts
```

### Modify

```text
web/src/main.ts
web/src/styles/variables.scss
web/src/styles/index.scss
web/src/router/index.ts
web/src/router/index.test.ts
web/src/permission.ts
web/src/views/auth/login/index.vue
web/src/views/auth/login/index.test.ts
web/src/layout/index.vue
web/src/layout/index.test.ts
web/src/layout/components/AppAside.vue
web/src/layout/components/AppHeader.vue
web/src/layout/components/AppFooter.vue
web/src/views/dashboard/index.vue
web/src/views/dashboard/index.test.ts
web/src/views/dashboard/components/ReadinessChart.vue
```

## Task 1: Remove the visible registration experience

**Files:**
- Modify: `web/src/router/index.test.ts`
- Modify: `web/src/views/auth/login/index.test.ts`
- Modify: `web/src/router/index.ts`
- Modify: `web/src/permission.ts`
- Modify: `web/src/views/auth/login/index.vue`

- [x] **Step 1: Change the router test to require only Login as the public Auth route**

Replace the public-route assertion with:

```ts
it('declares Login as public, removes Register, and protects Dashboard', () => {
  const router = createAppRouter(createMemoryHistory())
  expect(router.resolve('/login').meta.requiresAuth).toBe(false)
  expect(router.resolve('/register').matched).toHaveLength(0)
  expect(router.resolve('/dashboard').meta.requiresAuth).toBe(true)
})
```

- [x] **Step 2: Add a Login test proving there is no registration entry**

```ts
it('does not expose a registration entry', async () => {
  const { wrapper } = await mountLogin()
  expect(wrapper.find('a[href="/register"]').exists()).toBe(false)
  expect(wrapper.text()).not.toContain('注册新账号')
})
```

- [x] **Step 3: Run the focused tests and confirm the intended failures**

Run:

```powershell
cd D:\admin\web
pnpm vitest run src/router/index.test.ts src/views/auth/login/index.test.ts
```

Expected: FAIL because `/register` is still registered and the login page still renders its link.

- [x] **Step 4: Remove only the visible route and stale route-name branch**

Delete the `/register` route record from `router/index.ts`. Change the authenticated-public-route check in `permission.ts` to:

```ts
if (auth.status === 'authenticated' && to.name === 'login') {
  return { name: 'dashboard' }
}
```

Remove only this element from the Login template:

```vue
<RouterLink class="auth-link" to="/register">注册新账号</RouterLink>
```

Keep `api/auth.ts`, the backend endpoint, and `views/auth/register` unchanged.

- [x] **Step 5: Run the focused tests and inspect the diff**

Run:

```powershell
pnpm vitest run src/router/index.test.ts src/views/auth/login/index.test.ts
git diff --check
```

Expected: both focused test files PASS. The full suite in Task 6 covers the permission guard through router navigation tests.

## Task 2: Add the concrete Element Plus theme controller

**Files:**
- Create: `web/src/utils/theme.test.ts`
- Create: `web/src/utils/theme.ts`
- Modify: `web/src/main.ts`

- [x] **Step 1: Write theme behavior tests**

Create tests covering the default, persisted dark mode, DOM class, `color-scheme`, and toggle:

```ts
import { beforeEach, describe, expect, it } from 'vitest'

import { applyTheme, initializeTheme, readTheme, toggleTheme } from './theme'

describe('theme', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    document.documentElement.style.removeProperty('color-scheme')
  })

  it('defaults to the light Element Plus theme', () => {
    expect(initializeTheme()).toBe('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(document.documentElement.style.colorScheme).toBe('light')
  })

  it('restores and applies a stored dark theme', () => {
    localStorage.setItem('admin:theme', 'dark')
    expect(initializeTheme()).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })

  it('persists an explicit toggle', () => {
    expect(toggleTheme('light')).toBe('dark')
    expect(readTheme()).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    applyTheme('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })
})
```

- [x] **Step 2: Run the test and confirm the missing module failure**

Run:

```powershell
pnpm vitest run src/utils/theme.test.ts
```

Expected: FAIL because `utils/theme.ts` does not exist.

- [x] **Step 3: Implement the exact two-state theme controller**

Create `utils/theme.ts` with this public surface:

```ts
export type ThemeMode = 'light' | 'dark'

const themeStorageKey = 'admin:theme'

export function readTheme(): ThemeMode {
  return window.localStorage.getItem(themeStorageKey) === 'dark' ? 'dark' : 'light'
}

export function applyTheme(theme: ThemeMode): void {
  document.documentElement.classList.toggle('dark', theme === 'dark')
  document.documentElement.style.colorScheme = theme
}

export function initializeTheme(): ThemeMode {
  const theme = readTheme()
  applyTheme(theme)
  return theme
}

export function toggleTheme(current: ThemeMode): ThemeMode {
  const nextTheme: ThemeMode = current === 'light' ? 'dark' : 'light'
  window.localStorage.setItem(themeStorageKey, nextTheme)
  applyTheme(nextTheme)
  return nextTheme
}
```

This preference is optional UI state, so an absent or unknown stored value deliberately resolves to the documented light default.

- [x] **Step 4: Initialize Element Plus dark variables before mounting Vue**

In `main.ts`, import the official dark variables and call initialization before `createApp`:

```ts
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'
import { initializeTheme } from './utils/theme'

initializeTheme()
```

- [x] **Step 5: Verify theme behavior**

Run:

```powershell
pnpm vitest run src/utils/theme.test.ts
pnpm exec vue-tsc -b --pretty false
```

Expected: PASS with no TypeScript diagnostics.

## Task 3: Build the login visual surface without changing Auth behavior

**Files:**
- Modify: `web/src/views/auth/login/index.test.ts`
- Modify: `web/src/views/auth/login/index.vue`
- Modify: `web/src/styles/variables.scss`
- Modify: `web/src/styles/index.scss`

- [x] **Step 1: Add stable Login structure assertions**

Add a test requiring one brand region and one form region while preserving existing form hooks:

```ts
it('renders the product identity and the existing login form', async () => {
  const { wrapper } = await mountLogin()
  expect(wrapper.get('[data-testid="login-brand"]').text()).toContain('Admin')
  expect(wrapper.get('[data-testid="login-panel"]').exists()).toBe(true)
  expect(wrapper.get('[data-testid="login-username"]').exists()).toBe(true)
  expect(wrapper.get('[data-testid="login-password"]').exists()).toBe(true)
})
```

- [x] **Step 2: Run the Login test and confirm missing structure**

Run:

```powershell
pnpm vitest run src/views/auth/login/index.test.ts
```

Expected: FAIL because the new brand and panel hooks do not exist.

- [x] **Step 3: Replace hardcoded project colors with Element Plus semantic aliases**

Keep SCSS constants only for stable dimensions. Resolve colors in `variables.scss` through Element Plus variables:

```scss
$color-ink: var(--el-text-color-primary);
$color-workspace: var(--el-bg-color-page);
$color-surface: var(--el-bg-color);
$color-text: var(--el-text-color-primary);
$color-muted: var(--el-text-color-secondary);
$color-border: var(--el-border-color-light);
$color-success: var(--el-color-success);
$color-warning: var(--el-color-warning);
$color-danger: var(--el-color-danger);
$color-action: var(--el-color-primary);
$header-height: 56px;
$footer-height: 40px;
$panel-radius: 8px;
```

Add only semantic shell aliases in `index.scss`, each backed by `--el-*` values:

```scss
:root {
  --admin-page: var(--el-bg-color-page);
  --admin-surface: var(--el-bg-color);
  --admin-surface-soft: var(--el-fill-color-light);
  --admin-border: var(--el-border-color-light);
  --admin-text: var(--el-text-color-primary);
  --admin-text-soft: var(--el-text-color-secondary);
  --admin-shadow: var(--el-box-shadow-lighter);
}
```

- [x] **Step 4: Recompose the Login template with existing logic unchanged**

Use a two-column `auth-page` with:

```vue
<section class="auth-brand" data-testid="login-brand" aria-label="Admin 管理系统">
  <div class="auth-brand__mark" aria-hidden="true">A</div>
  <p class="auth-brand__eyebrow">ADMIN CONSOLE</p>
  <h1>让系统状态清楚可见</h1>
  <p>统一管理服务状态、任务与后续权限配置。</p>
</section>

<section class="auth-panel" data-testid="login-panel" aria-labelledby="login-title">
  <!-- existing error paragraphs and Element Plus form -->
</section>
```

Use Element Plus inputs/buttons/icons. Preserve `submit`, form rules, safe redirect, all `data-testid` values, and the current username/password payload exactly. Do not add a register link, feature carousel, policy checkbox, or fake social login.

- [x] **Step 5: Implement responsive and dark-mode styles**

Desktop uses a constrained two-column grid. At `max-width: 760px`, hide the descriptive brand copy but retain a compact mark above the form. Use `--admin-*` and `--el-*` variables for every theme-dependent color. Keep radius at 8px or less and disable nonessential transitions under `prefers-reduced-motion`.

- [x] **Step 6: Verify Login behavior and types**

Run:

```powershell
pnpm vitest run src/views/auth/login/index.test.ts
pnpm exec vue-tsc -b --pretty false
```

Expected: all Login tests PASS and the exact API payload test remains unchanged.

## Task 4: Apply the theme to the protected shell

**Files:**
- Modify: `web/src/layout/index.test.ts`
- Modify: `web/src/layout/index.vue`
- Modify: `web/src/layout/components/AppAside.vue`
- Modify: `web/src/layout/components/AppHeader.vue`
- Modify: `web/src/layout/components/AppFooter.vue`
- Modify: `web/src/styles/index.scss`

- [x] **Step 1: Add a layout theme-toggle test**

Reset local storage and the root class in `beforeEach`, then add:

```ts
it('toggles the Element Plus dark theme from the Header', async () => {
  const { wrapper } = await mountLayout()
  expect(document.documentElement.classList.contains('dark')).toBe(false)
  await wrapper.get('[data-testid="toggle-theme"]').trigger('click')
  expect(document.documentElement.classList.contains('dark')).toBe(true)
  expect(localStorage.getItem('admin:theme')).toBe('dark')
})
```

- [x] **Step 2: Run the layout test and confirm the missing control failure**

Run:

```powershell
pnpm vitest run src/layout/index.test.ts
```

Expected: FAIL because the Header has no theme control.

- [x] **Step 3: Own theme state at the layout boundary**

In `layout/index.vue`:

```ts
import { readTheme, toggleTheme, type ThemeMode } from '../utils/theme'

const theme = ref<ThemeMode>(readTheme())

function handleToggleTheme(): void {
  theme.value = toggleTheme(theme.value)
}
```

Pass `theme` to `AppHeader` and handle `toggle-theme`. Keep collapse, drawer, logout, and RouterView behavior unchanged.

- [x] **Step 4: Add the accessible theme button to Header**

Add `Moon` and `Sunny` icons, a typed `theme: ThemeMode` prop, and a `toggleTheme` emit. Render one square text button:

```vue
<el-tooltip :content="theme === 'dark' ? '切换为浅色主题' : '切换为深色主题'">
  <el-button
    data-testid="toggle-theme"
    text
    :icon="theme === 'dark' ? Sunny : Moon"
    :aria-label="theme === 'dark' ? '切换为浅色主题' : '切换为深色主题'"
    @click="$emit('toggleTheme')"
  />
</el-tooltip>
```

- [x] **Step 5: Restyle Aside, Header, Footer, Drawer, and workspace**

Use the reference project's visual ideas without its stores or architecture:

- aside width stays `224px` expanded and `64px` collapsed;
- logo, menu item, and collapse states use Element Plus fill/text/primary variables;
- Header uses a bottom border, breadcrumb/current location, theme button, username, and icon logout command;
- mobile keeps the existing `el-drawer` behavior;
- Footer remains one line and uses secondary text;
- stable heights and widths prevent layout shift.

No additional menus or settings drawer are created.

- [x] **Step 6: Verify the shell**

Run:

```powershell
pnpm vitest run src/layout/index.test.ts
pnpm exec vue-tsc -b --pretty false
```

Expected: existing collapse, mobile drawer, logout, RouterView tests and the new theme test all PASS.

## Task 5: Rework the dashboard as an Element Plus operations surface

**Files:**
- Modify: `web/src/views/dashboard/index.test.ts`
- Modify: `web/src/views/dashboard/index.vue`
- Modify: `web/src/views/dashboard/components/ReadinessChart.vue`
- Modify: `web/src/styles/index.scss`

- [x] **Step 1: Add a test for the three real status summaries**

Add:

```ts
it('uses the three real dependency results as dashboard summaries', async () => {
  const wrapper = mountDashboard()
  await flushPromises()
  expect(wrapper.findAll('[data-testid$="-status"]')).toHaveLength(3)
  expect(wrapper.get('[data-testid="dashboard-summary"]').text()).toContain('API')
  expect(wrapper.get('[data-testid="dashboard-summary"]').text()).toContain('PostgreSQL')
  expect(wrapper.get('[data-testid="dashboard-summary"]').text()).toContain('Redis')
})
```

- [x] **Step 2: Run the test and confirm the missing summary container**

Run:

```powershell
pnpm vitest run src/views/dashboard/index.test.ts
```

Expected: FAIL because `dashboard-summary` is not present.

- [x] **Step 3: Recompose existing real data without adding fake metrics**

Keep `refreshHealth`, `submitTask`, status values, and error behavior unchanged. Recompose the template into:

```text
page header: 工作台 / refresh action
status summary: API / PostgreSQL / Redis using existing results
content grid: readiness chart / example Asynq task form
```

Add `data-testid="dashboard-summary"` to the summary group. Use Element Plus tags/buttons and the current icons. Do not add invented user counts, audit rows, or RBAC cards.

- [x] **Step 4: Make the chart consume Element Plus theme colors**

Replace the hardcoded color map with values read from the document root:

```ts
function themeColor(name: string, fallback: string): string {
  const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return value === '' ? fallback : value
}
```

Use `--el-color-warning`, `--el-color-success`, `--el-color-danger`,
`--el-bg-color`, and `--el-text-color-regular`. Observe the root `class`
attribute with one `MutationObserver` so a theme toggle rerenders the chart,
and disconnect it in `onBeforeUnmount`.

- [x] **Step 5: Implement responsive styles**

Use a two-column content grid above `840px` and one column below it. Keep status summaries stable at three columns on desktop and stack or horizontally scroll them on narrow screens. Use full-width unframed page sections and only frame the two genuine tools.

- [x] **Step 6: Verify dashboard behavior**

Run:

```powershell
pnpm vitest run src/views/dashboard/index.test.ts
pnpm exec vue-tsc -b --pretty false
```

Expected: all health failure, queue failure, task payload, and summary tests PASS.

## Task 6: Full frontend verification and diff audit

**Files:**
- Verify all files in this plan

- [x] **Step 1: Run formatting-sensitive and AnyScript scans**

Run:

```powershell
cd D:\admin
git diff --check
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>|@ts-ignore" web\src -g "*.ts" -g "*.vue"
```

Expected: `git diff --check` exits 0 and the scan prints no matches.

- [x] **Step 2: Run the complete frontend tests**

Run:

```powershell
cd D:\admin\web
pnpm vitest run
```

Expected: all test files PASS with zero failures.

- [x] **Step 3: Run strict type checking and production build**

Run:

```powershell
pnpm exec vue-tsc -b --pretty false
pnpm build
```

Expected: both commands exit 0. Record the existing bundle-size warning separately if it remains.

- [x] **Step 4: Audit the final scope**

Run:

```powershell
cd D:\admin
git status --short --branch
git diff --stat
git diff -- web/src/router web/src/permission.ts web/src/utils/theme.ts web/src/layout web/src/views/auth/login web/src/views/dashboard web/src/styles web/src/main.ts
```

Confirm that Auth API/Store/request behavior and registration source/API files are unchanged, no service was started, and no commit or push was made.
