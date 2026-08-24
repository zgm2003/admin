# Frontend Component Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build strict-typed `AppDialog` and generic `AppTable` components from the existing admin frontend conventions, then migrate the five real system pages without moving business requests or permission logic into shared components.

**Architecture:** Keep the current Vue data flow `view -> api/<module>.ts -> utils/request.ts -> Go API`. `AppDialog` owns only Element Plus dialog structure, responsive sizing, scroll ownership, slots, and focus restoration. `AppTable<Row>` owns only typed table columns, table state presentation, selection, and pagination events; each page keeps its own query state, API calls, permissions, filters, and mutations.

**Tech Stack:** Vue 3.5, TypeScript strict mode, Element Plus, Vitest, Vue Test Utils, Vue I18n, existing CSS variables and layout scroll rules.

---

## Task 1: Capture the Existing Frontend Baseline

**Files:**
- Read: `web/src/styles/index.scss`
- Read: `web/src/styles/variables.scss`
- Read: `web/src/views/system/roles/index.vue`
- Read: `web/src/views/system/users/index.vue`
- Read: `web/src/views/system/auth-platforms/index.vue`
- Read: `web/src/views/system/operation-logs/index.vue`
- Read: `web/src/views/system/sessions/index.vue`
- Test: existing page tests under `web/src/views/system/**/index.test.ts`

- [ ] **Step 1: Run the frontend baseline tests and build**

Run from `D:\admin\web`:

```powershell
pnpm vitest run
pnpm build
```

Expected: the existing test suite and production build pass before component migration begins. Record the test count and do not change page behavior in this task.

- [ ] **Step 2: Record the repeated contracts in the implementation notes**

Confirm the current pages use the same concepts before extracting them: Element Plus table loading/empty state, table pagination, selectable rows, form/detail dialogs, dialog footer actions, and internal page scrolling. Keep `RolePermissionMatrix.vue` out of the generic table migration because it is a permission matrix rather than a CRUD table.

## Task 2: Build AppDialog Utility Contract Test-First

**Files:**
- Create: `web/src/components/AppDialog/src/dialog.ts`
- Create: `web/src/components/AppDialog/src/dialog.test.ts`
- Create: `web/src/components/AppDialog/src/index.vue`
- Create: `web/src/components/AppDialog/src/index.test.ts`
- Create: `web/src/components/AppDialog/index.ts`

- [ ] **Step 1: Write utility tests for sizing and attribute filtering**

Cover these concrete cases in `dialog.test.ts`:

```ts
expect(resolveAppDialogWidth({ isMobile: false })).toBe('720px')
expect(resolveAppDialogWidth({ isMobile: true })).toBe('94vw')
expect(resolveAppDialogWidth({ isMobile: true, mobileWidth: 320 })).toBe('320px')
expect(resolveAppDialogContentHeight(560)).toBe('560px')
expect(resolveAppDialogContentHeight()).toBeUndefined()
expect(filterAppDialogAttrs({ fullscreen: true, id: 'edit-user' })).toEqual({ id: 'edit-user' })
```

Also test that `resolveAppDialogAlignCenter` and `resolveAppDialogDraggable` disable desktop-only behavior on mobile.

- [ ] **Step 2: Run the utility tests and verify the expected red failure**

Run:

```powershell
pnpm vitest run src/components/AppDialog/src/dialog.test.ts
```

Expected: FAIL because the AppDialog utility module does not exist yet. Fix only test/import mistakes if the failure is not a missing implementation.

- [ ] **Step 3: Implement the minimal utility module**

Implement the exact constants and pure functions used by the tests. Use `string | number` for CSS lengths, return `undefined` for empty values, and keep `fullscreen` filtered from Element Plus attrs. Do not add a global dialog service or business-specific props.

- [ ] **Step 4: Write component tests for controlled state, slots, and focus**

Mount `AppDialog` with Element Plus in the existing jsdom test setup and cover:

- `modelValue` renders the dialog and emits `update:modelValue` when Element Plus closes it;
- default content, `header`, and `footer` slots render;
- `height` creates the scroll-body path;
- opening records the active trigger and `closed` restores focus;
- `ariaLabel` remains available when the visible header is disabled.

Use concrete typed fixtures and `unknown` for caught errors. Do not use `any` or `as any` in tests.

- [ ] **Step 5: Run the component tests and verify the expected red failure**

Run:

```powershell
pnpm vitest run src/components/AppDialog/src/dialog.test.ts src/components/AppDialog/src/index.test.ts
```

Expected: the tests fail because `index.vue` has not been implemented.

- [ ] **Step 6: Implement AppDialog as a thin Element Plus wrapper**

Use `<script setup lang="ts">`, `defineProps`, `defineEmits`, `useAttrs`, `useSlots`, `useId`, `computed`, `nextTick`, and `watch`. Implement responsive desktop/mobile width, typed props from the spec, controlled `update:modelValue` emission, default/header/footer slots, optional body scroll using `el-scrollbar` only when `height` is provided, focus capture/restoration, scoped classes for hidden/custom header/footer padding, and attrs forwarding after filtering `fullscreen`.

Export the component from `web/src/components/AppDialog/index.ts`.

- [ ] **Step 7: Run AppDialog tests and TypeScript build checks**

Run:

```powershell
pnpm vitest run src/components/AppDialog/src/dialog.test.ts src/components/AppDialog/src/index.test.ts
pnpm build
```

Expected: PASS with no explicit `any`, `as any`, or TypeScript suppression.

## Task 3: Build AppTable Types and Pure Column Helpers

**Files:**
- Create: `web/src/components/AppTable/src/types.ts`
- Create: `web/src/components/AppTable/src/types.test.ts`
- Create: `web/src/components/AppTable/index.ts`

- [ ] **Step 1: Write failing tests for typed column identity and values**

Define a fixture:

```ts
interface UserRow {
  id: number
  username: string
  enabled: boolean
}
```

Test that property columns resolve their key/value, derived columns require an explicit key, formatter receives the typed property value, invalid columns throw a clear error, and `TablePaginationState` uses `currentPage`, `pageSize`, and `total`.

- [ ] **Step 2: Run the type helper tests and verify red**

Run:

```powershell
pnpm vitest run src/components/AppTable/src/types.test.ts
```

Expected: FAIL because the AppTable types module does not exist.

- [ ] **Step 3: Implement the typed table contract**

Implement `TableRow = object`, `TableColumn<Row extends TableRow>` as property or derived column, limited `TableColumnElementProps<Row>`, `TablePaginationState { currentPage: number; pageSize: number; total: number }`, and `tableColumnKey`, `tableColumnProp`, `tableColumnValue`, `formatTableColumnValue`. Use `unknown` for derived values and formatter returns. Do not use `Record<string, any>`.

- [ ] **Step 4: Run the helper tests**

Run:

```powershell
pnpm vitest run src/components/AppTable/src/types.test.ts
```

Expected: PASS.

## Task 4: Build AppTable Component Test-First

**Files:**
- Create: `web/src/components/AppTable/src/index.vue`
- Create: `web/src/components/AppTable/src/index.test.ts`
- Modify: `web/src/components/AppTable/index.ts`

- [ ] **Step 1: Write failing component tests**

Mount a typed `AppTable<UserRow>` and cover property and derived columns, hidden columns, `loading`/`aria-busy`, explicit empty/error states without fake rows, pagination events with `currentPage`/`pageSize`, typed selection events, toolbar/empty/cell slots, and the absence of API calls or Pinia access.

- [ ] **Step 2: Run the component tests and verify red**

Run:

```powershell
pnpm vitest run src/components/AppTable/src/types.test.ts src/components/AppTable/src/index.test.ts
```

Expected: component tests fail because `index.vue` is not implemented.

- [ ] **Step 3: Implement the minimal generic table**

Implement a Vue generic SFC with the props and events from the spec, typed property/derived column rendering, `cell-<key>` slots, toolbar and empty slots, Element Plus table/column/pagination primitives, mobile pagination layout, existing layout scroll ownership, and only the fixed-footer behavior required by migrated pages. The component must not own an API state machine.

Keep CSS component-scoped and use current Element Plus CSS variables. Do not introduce a new palette or global reset.

- [ ] **Step 4: Run AppTable tests and build**

Run:

```powershell
pnpm vitest run src/components/AppTable/src/types.test.ts src/components/AppTable/src/index.test.ts
pnpm build
```

Expected: PASS.

## Task 5: Migrate Roles Page

**Files:**
- Modify: `web/src/views/system/roles/index.vue`
- Modify: `web/src/views/system/roles/index.test.ts`
- Keep: `web/src/views/system/roles/components/RolePermissionMatrix.vue`
- Keep: `web/src/views/system/roles/components/RolePermissionDiffDialog.vue`

- [ ] **Step 1: Write migration assertions**

Assert that the page renders `AppTable` for the role list and `AppDialog` for the role form and permission dialog while preserving existing create/update/default/status/delete/permission assertions.

- [ ] **Step 2: Replace only the list and dialog shells**

Define a typed `TableColumn<RoleListItem>[]` configuration and move existing cell templates to `cell-<key>` slots. Keep query state, permission computed values, API calls, confirmation messages, and error handling in the page. Replace direct dialog shells with `AppDialog`; keep `RolePermissionMatrix` as the dialog body.

- [ ] **Step 3: Run the roles tests**

Run:

```powershell
pnpm vitest run src/views/system/roles/index.test.ts
```

Expected: PASS with unchanged business assertions.

## Task 6: Migrate Users Page

**Files:**
- Modify: `web/src/views/system/users/index.vue`
- Modify: `web/src/views/system/users/index.test.ts`

- [ ] **Step 1: Add AppTable/AppDialog migration assertions**

Assert that the list uses `AppTable`, username edit uses `AppDialog`, and role assignment uses `AppDialog`; preserve role selection, protected super-admin behavior, loading, empty, and error assertions.

- [ ] **Step 2: Migrate the table and both dialogs**

Use `TableColumn<UserListItem>[]` with typed cell slots for roles, status, and actions. Keep role selection and all mutation guards in the page. Use `AppDialog` for edit and role assignment; keep Element Plus message-box confirmation for destructive operations.

- [ ] **Step 3: Run the users tests**

Run:

```powershell
pnpm vitest run src/views/system/users/index.test.ts
```

Expected: PASS.

## Task 7: Migrate Authentication Platforms Page

**Files:**
- Modify: `web/src/views/system/auth-platforms/index.vue`
- Modify: `web/src/views/system/auth-platforms/index.test.ts`

- [ ] **Step 1: Add component usage assertions**

Assert that platform rows use `AppTable` and the create/edit form uses `AppDialog`. Preserve strict form validation, status and delete permissions, built-in protection, and pagination behavior.

- [ ] **Step 2: Migrate the platform table and form dialog**

Use typed columns and cell slots for policy/security/session summaries. Keep all API calls, form state, access permissions, and error notifications in the page.

- [ ] **Step 3: Run the authentication platform tests**

Run:

```powershell
pnpm vitest run src/views/system/auth-platforms/index.test.ts
```

Expected: PASS.

## Task 8: Migrate Operation Logs and Sessions Pages

**Files:**
- Modify: `web/src/views/system/operation-logs/index.vue`
- Modify: `web/src/views/system/operation-logs/index.test.ts`
- Modify: `web/src/views/system/sessions/index.vue`
- Modify: `web/src/views/system/sessions/index.test.ts`

- [ ] **Step 1: Migrate operation logs to AppTable**

Keep the expand-row slot for request/response details, typed status and latency cells, and existing loading/empty/error test selectors. Do not introduce an AppDialog because this page has no repeated form/detail dialog requirement.

- [ ] **Step 2: Run operation-log tests**

Run:

```powershell
pnpm vitest run src/views/system/operation-logs/index.test.ts
```

Expected: PASS.

- [ ] **Step 3: Migrate sessions to AppTable**

Keep selection constraints, bulk revoke, single revoke, session statistics, and permission checks in the page. Use typed selection and action cell slots; do not move session mutations into the table.

- [ ] **Step 4: Run session tests**

Run:

```powershell
pnpm vitest run src/views/system/sessions/index.test.ts
```

Expected: PASS.

## Task 9: Public Component Contract and Full Frontend Verification

**Files:**
- Create or modify: `web/src/components/index.ts` only if direct component imports need a stable local export
- Modify: `web/src/styles/index.scss` only for component styles that cannot remain scoped
- Test: `web/src/components/AppDialog/src/*.test.ts`
- Test: `web/src/components/AppTable/src/*.test.ts`

- [ ] **Step 1: Check component imports and strict TypeScript usage**

Run:

```powershell
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>|@ts-ignore" web/src/components web/src/views/system
```

Expected: no output.

- [ ] **Step 2: Run the full frontend verification**

Run from `D:\admin\web`:

```powershell
pnpm vitest run
pnpm build
```

Expected: all existing and new tests pass and the production build succeeds.

- [ ] **Step 3: Run a final diff and scope check**

Run from `D:\admin`:

```powershell
git diff --check
git status --short
```

Confirm only AppDialog/AppTable files, the five migrated pages/tests, the approved spec/plan, and necessary style/export files changed. Do not commit, fetch, pull, or push.

