# Admin UI Shell Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move account operations to the sidebar footer, improve the settings drawer, and make content fullscreen always recoverable.

**Architecture:** Retain the existing `Layout -> AppAside/AppHeader/RouteTabs/SettingDrawer` structure and the Pinia UI-preference store. Add only explicit component events and props needed by this shell: the layout owns logout and fullscreen state; the sidebar emits account commands; RouterTabs emits fullscreen toggles. No personal-center route, database API, or reusable framework abstraction is added in this slice.

**Tech Stack:** Vue 3, TypeScript strict mode, Pinia, Vue Router, Element Plus, Vitest, Vite, Playwright CLI.

**Spec:** `docs/learning/future-menu-template.md`

## Global Constraints

- Preserve the frontend flow `view -> api/<module>.ts -> utils/request.ts -> Go API`; this shell slice adds no API calls.
- Preserve existing router, store, and component boundaries; do not introduce Manager, Factory, base classes, or configuration-driven layout engines.
- Use Lucide or Element Plus icons already available in the project, with labels and tooltips for icon-only actions.
- Keep `RouterTabs` rendered during content fullscreen; offer direct exit and `Escape` exits.
- Do not create a personal-center placeholder route; until its module exists, its sidebar action is visibly unavailable rather than navigating to a fake page.
- Use test-first implementation and run focused Vitest tests before each production change.
- Do not commit automatically.

---

## File Structure

- Modify: `web/src/layout/index.vue` - own sidebar account command handling, logout, fullscreen state, and Escape listener.
- Modify: `web/src/layout/components/AppAside.vue` - render the footer account trigger and emit account commands.
- Modify: `web/src/layout/components/AppHeader.vue` - leave only navigation, locale, and settings controls in the header.
- Modify: `web/src/layout/components/RouteTabs.vue` - expose a direct content-fullscreen toggle control and testable state.
- Modify: `web/src/layout/components/SettingDrawer.vue` - reorganize the existing settings UI without changing persistence fields.
- Modify: `web/src/styles/index.scss` - size shell tracks and style the fullscreen recovery control responsively.
- Modify: `web/tests/layout/index.test.ts` - shell integration coverage.
- Modify: `web/tests/layout/components/AccessMenuNode.test.ts` - sidebar footer coverage.
- Modify: `web/tests/layout/components/RouteTabs.test.ts` - direct fullscreen and Escape coverage.

### Task 1: Reproduce and Guard Fullscreen Recovery

**Files:**
- Modify: `web/tests/layout/index.test.ts`
- Modify: `web/tests/layout/components/RouteTabs.test.ts`
- Modify: `web/src/layout/index.vue`
- Modify: `web/src/layout/components/RouteTabs.vue`

**Interfaces:**
- `RouteTabs` consumes `fullscreen: boolean` and emits `toggleFullscreen: []`.
- `Layout` owns `contentFullscreen: Ref<boolean>` and passes the prop/event through.
- Produce: direct control `[data-testid="exit-content-fullscreen"]`, which is present only while fullscreen; pressing `Escape` exits fullscreen unless a descendant popup handles that key.

- [x] **Step 1: Write failing component tests**

```ts
it('shows a direct exit action only during fullscreen', async () => {
  const { wrapper } = await mountTabs('/dashboard')
  expect(wrapper.find('[data-testid="exit-content-fullscreen"]').exists()).toBe(false)
  await wrapper.setProps({ fullscreen: true })
  await wrapper.get('[data-testid="exit-content-fullscreen"]').trigger('click')
  expect(wrapper.emitted('toggleFullscreen')).toHaveLength(1)
})
```

```ts
it('exits content fullscreen with Escape while preserving RouteTabs', async () => {
  const { wrapper } = await mountLayout()
  wrapper.findComponent({ name: 'RouteTabs' }).vm.$emit('toggle-fullscreen')
  await wrapper.vm.$nextTick()
  await wrapper.get('.admin-layout').trigger('keydown', { key: 'Escape' })
  expect(wrapper.find('.admin-layout__header').exists()).toBe(true)
  expect(wrapper.find('.admin-layout__tabs').exists()).toBe(true)
})
```

- [x] **Step 2: Run focused tests and verify they fail for missing recovery behavior**

Run: `pnpm vitest run tests/layout/index.test.ts tests/layout/components/RouteTabs.test.ts --pool=threads --maxWorkers=1`

Expected: failure because the direct exit action and layout Escape handler do not exist.

- [x] **Step 3: Implement the smallest recovery path**

```ts
function handleKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape' && contentFullscreen.value) {
    contentFullscreen.value = false
  }
}
```

Render the existing fullscreen command as a direct `el-button` when `fullscreen` is true, using the existing `toggleFullscreen` event. Register and remove the document key listener from `Layout`.

- [x] **Step 4: Run focused tests and verify they pass**

Run: `pnpm vitest run tests/layout/index.test.ts tests/layout/components/RouteTabs.test.ts --pool=threads --maxWorkers=1`

Expected: PASS.

### Task 2: Move Account Operations to the Sidebar Footer

**Files:**
- Modify: `web/tests/layout/components/AccessMenuNode.test.ts`
- Modify: `web/tests/layout/index.test.ts`
- Modify: `web/src/layout/components/AppAside.vue`
- Modify: `web/src/layout/components/AppHeader.vue`
- Modify: `web/src/layout/index.vue`

**Interfaces:**
- `AppAside` consumes `username`, `email`, `collapsed`, and `uniqueOpened`.
- `AppAside` emits `logout: []` and `openProfile: []`.
- `Layout` provides identity from `useAuthStore`, owns logout, and handles `openProfile` with an explicit informational feedback until the personal-center module is implemented.

- [x] **Step 1: Write failing sidebar and layout tests**

```ts
it('renders the signed-in account in the sidebar footer and emits logout', async () => {
  const wrapper = mount(AppAside, { props: { collapsed: false, uniqueOpened: true, username: 'admin', email: 'admin@example.com' } })
  expect(wrapper.get('[data-testid="aside-account-name"]').text()).toBe('admin')
  await wrapper.get('[data-testid="aside-account-menu"]').trigger('click')
  getPopupItem('aside-account-logout').click()
  expect(wrapper.emitted('logout')).toHaveLength(1)
})
```

```ts
it('removes account actions from Header and keeps logout available in Aside', async () => {
  const { wrapper } = await mountLayout()
  expect(wrapper.find('[data-testid="current-username"]').exists()).toBe(false)
  expect(wrapper.get('[data-testid="aside-account-trigger"]').exists()).toBe(true)
})
```

- [x] **Step 2: Run focused tests and verify they fail for the absent footer account UI**

Run: `pnpm vitest run tests/layout/index.test.ts tests/layout/components/AccessMenuNode.test.ts --pool=threads --maxWorkers=1`

Expected: failure because `AppAside` has no account props, footer trigger, or emitted commands.

- [x] **Step 3: Implement the focused account footer**

```ts
const emit = defineEmits<{ logout: []; openProfile: [] }>()
```

Use an `el-dropdown` footer trigger with Avatar, username and email. Keep only profile and logout commands. Remove the Header account block and its props. In collapsed mode retain the avatar trigger and tooltips; in mobile Drawer render the full account block.

- [x] **Step 4: Run focused tests and verify they pass**

Run: `pnpm vitest run tests/layout/index.test.ts tests/layout/components/AccessMenuNode.test.ts --pool=threads --maxWorkers=1`

Expected: PASS.

### Task 3: Refresh Settings Presentation Without Changing Its Contract

**Files:**
- Modify: `web/tests/layout/index.test.ts`
- Modify: `web/src/layout/components/SettingDrawer.vue`
- Modify: `web/src/styles/index.scss`

**Interfaces:**
- `SettingDrawer` keeps `modelValue: boolean` and `contentFullscreen: boolean`.
- The persisted `UIPreferences` shape remains unchanged.
- Produce: semantic group hooks `data-testid="settings-theme-section"`, `data-testid="settings-display-section"`, and `data-testid="settings-transition-section"`.

- [x] **Step 1: Write a failing presentation-contract test**

```ts
it('groups theme, display, and transition preferences in the settings drawer', async () => {
  const { wrapper } = await mountLayout()
  await wrapper.get('[data-testid="open-settings"]').trigger('click')
  expect(wrapper.get('[data-testid="settings-theme-section"]').exists()).toBe(true)
  expect(wrapper.get('[data-testid="settings-display-section"]').exists()).toBe(true)
  expect(wrapper.get('[data-testid="settings-transition-section"]').exists()).toBe(true)
})
```

- [x] **Step 2: Run the focused test and verify it fails for missing semantic groups**

Run: `pnpm vitest run tests/layout/index.test.ts --pool=threads --maxWorkers=1`

Expected: failure because the named group hooks do not yet exist.

- [x] **Step 3: Implement the visual hierarchy and shell sizing**

Use the same preference controls and fields, but make section labels and mode choices visually distinct, make theme color swatches rectangular stable controls, and make reset a full-width secondary action at the drawer bottom. Set Header desktop/mobile height and ensure controls cannot overflow on narrow screens.

- [x] **Step 4: Run the focused test and verify it passes**

Run: `pnpm vitest run tests/layout/index.test.ts --pool=threads --maxWorkers=1`

Expected: PASS.

### Task 4: Browser Reproduction and Final Verification

**Files:**
- No production file changes expected.

**Interfaces:**
- Dev server: `pnpm dev -- --host localhost` at `http://localhost:16300`.
- Fullscreen control: `[data-testid="exit-content-fullscreen"]`.

- [x] **Step 1: Run full frontend verification**

Run: `pnpm vitest run --pool=threads --maxWorkers=1` then `pnpm build`

Expected: all tests and type/build validation pass.

- [x] **Step 2: Run browser-level regression at desktop and mobile viewports**

Start the Vite dev server. With Playwright, load a valid authenticated test path or test harness, enter content fullscreen via RouterTabs, confirm the tab strip and direct exit control remain visible, press Escape, and confirm Header and sidebar return. Capture desktop and mobile screenshots and inspect for overlap or clipping.

- [x] **Step 3: Review the diff**

Run: `git diff --check`, `git diff -- web/src/layout web/src/styles/index.scss web/tests/layout`, and `git status --short`.

Expected: only this UI slice and its design/plan documents are changed; do not stage or commit changes automatically.
