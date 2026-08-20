# Role Permission Matrix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the role authorization tree with the approved permission matrix and require a visible permission-diff confirmation before saving.

**Architecture:** Keep the HTTP contract unchanged. Pure functions convert the strict `directory -> page -> action` DTO into matrix groups, expand minimal direct grants into effective UI selections, and normalize effective selections back into minimal direct grants. Local Vue components render the matrix and diff dialog while `views/system/roles/index.vue` remains the linear API and state coordinator.

**Tech Stack:** Vue 3 `<script setup>`, TypeScript strict mode, Element Plus, Vue I18n, Vitest, Vue Test Utils.

**Execution constraints:** Execute in the current `main` workspace because the repository owner explicitly requested it. Do not create a worktree, use subagents, start services, commit, or push.

---

### Task 1: Permission Matrix Pure Functions

**Files:**
- Create: `web/src/views/system/roles/role-permission-matrix.test.ts`
- Create: `web/src/views/system/roles/role-permission-matrix.ts`

- [ ] **Step 1: Write failing tests for matrix construction and grant transformations**

Define a complete directory/page/action fixture using `RolePermissionTreeNode`, including a nested directory and one disabled action. Assert these public functions:

```ts
const groups = buildRolePermissionMatrix(menuTree)
expect(groups[0]?.rows.map((row) => row.pageId)).toEqual([2, 5])

expect(expandDirectMenuIDs(groups, [3])).toEqual([2, 3])
expect(toggleMatrixAction([], groups[0]!.rows[0]!, 3, true)).toEqual([2, 3])
expect(toggleMatrixPage([2, 3], groups[0]!.rows[0]!, false)).toEqual([])
expect(toggleMatrixAction([2, 3], groups[0]!.rows[0]!, 3, false)).toEqual([2])
expect(normalizeDirectMenuIDs(groups, [2, 3])).toEqual([3])
expect(normalizeDirectMenuIDs(groups, [2])).toEqual([2])
expect(diffMenuIDs([2, 3], [2, 4])).toEqual({ added: [4], removed: [3] })
```

Also assert group selection state for empty, partial, and complete selections and assert disabled nodes remain present.

- [ ] **Step 2: Run the pure-function test and verify RED**

Run:

```powershell
cd D:\admin\web
pnpm vitest run src/views/system/roles/role-permission-matrix.test.ts
```

Expected: FAIL because `role-permission-matrix.ts` and its exports do not exist.

- [ ] **Step 3: Implement the typed matrix model and pure functions**

Create explicit types carrying real menu metadata:

```ts
export interface RoleMatrixAction {
  id: number
  code: string
  i18nKey: MenuTitleKey
  isEnabled: YesNo
}

export interface RoleMatrixRow {
  pageId: number
  pageCode: string
  pageI18nKey: MenuTitleKey
  pageIsEnabled: YesNo
  actions: RoleMatrixAction[]
}

export interface RoleMatrixGroup {
  groupId: number
  groupCode: string
  groupI18nKey: MenuTitleKey
  rows: RoleMatrixRow[]
}
```

Implement these exact exports without fallback groups or `any`:

```ts
export function buildRolePermissionMatrix(nodes: readonly RolePermissionTreeNode[]): RoleMatrixGroup[]
export function getRoleMatrixRowMenuIDs(row: RoleMatrixRow): number[]
export function getRoleMatrixGroupMenuIDs(group: RoleMatrixGroup): number[]
export function getRoleMatrixMenuIDs(groups: readonly RoleMatrixGroup[]): number[]
export function getRoleMatrixSelectionState(menuIDs: readonly number[], selected: readonly number[] | ReadonlySet<number>): RoleMatrixSelectionState
export function expandDirectMenuIDs(groups: readonly RoleMatrixGroup[], directMenuIDs: readonly number[]): number[]
export function toggleMatrixPage(selected: readonly number[], row: RoleMatrixRow, checked: boolean): number[]
export function toggleMatrixAction(selected: readonly number[], row: RoleMatrixRow, actionID: number, checked: boolean): number[]
export function toggleMatrixGroup(selected: readonly number[], group: RoleMatrixGroup, checked: boolean): number[]
export function normalizeDirectMenuIDs(groups: readonly RoleMatrixGroup[], effectiveMenuIDs: readonly number[]): number[]
export function diffMenuIDs(before: readonly number[], after: readonly number[]): { added: number[]; removed: number[] }
```

Walk nested directories into their root group, preserve API order for rendering, validate action membership by lookup rather than accepting unknown IDs, and return sorted unique ID arrays for state mutations and persistence.

- [ ] **Step 4: Run the pure-function test and verify GREEN**

Run the Task 1 command again. Expected: all matrix tests PASS.

### Task 2: Matrix Component and Internationalization

**Files:**
- Create: `web/src/views/system/roles/components/RolePermissionMatrix.vue`
- Create: `web/src/views/system/roles/components/RolePermissionMatrix.css`
- Create: `web/src/views/system/roles/components/RolePermissionMatrix.test.ts`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`

- [ ] **Step 1: Write failing matrix component expectations**

Mount the real component with Element Plus and app i18n. Assert the matrix renders:

```ts
expect(document.body.textContent).toContain('页面权限')
expect(document.body.textContent).toContain('操作权限')
expect(document.body.textContent).toContain('角色管理')
expect(document.body.textContent).toContain('新增角色')
expect(document.body.textContent).toContain('已禁用')
expect(checkedCheckboxLabels()).toEqual(expect.arrayContaining(['角色管理']))
```

Add user-facing assertions that selecting an action emits action plus page, clearing a page emits neither, and the group checkbox exposes the correct half-selected state.

- [ ] **Step 2: Run the role page test and verify RED**

Run:

```powershell
cd D:\admin\web
pnpm vitest run src/views/system/roles/components/RolePermissionMatrix.test.ts
```

Expected: FAIL because the matrix labels and controls are absent.

- [ ] **Step 3: Add exact bilingual matrix keys**

Add matching keys in both locale files:

```ts
'role.permission.page': '页面权限',
'role.permission.action': '操作权限',
'role.permission.selectAll': '全选',
'role.permission.clear': '清空',
'role.permission.selected': '权限',
'role.permission.pages': '页面',
'role.permission.actions': '操作',
'role.permission.noActions': '无操作权限',
'role.permission.expand': '展开',
'role.permission.collapse': '收起',
'role.permission.added': '新增权限',
'role.permission.removed': '移除权限',
'role.permission.confirmTitle': '确认权限变更',
```

Use equivalent concise English values so `enUS satisfies Record<AppMessageKey, string>` remains exhaustive.

- [ ] **Step 4: Implement the local matrix component**

Use `defineModel<number[]>({ required: true })`, typed `groups`, `useI18n`, and the Task 1 functions. Match the reference component structure:

```vue
<section v-for="group in groups" :key="group.groupId" class="role-permission-matrix__group">
  <div class="role-permission-matrix__group-header">
    <el-checkbox
      :model-value="groupState(group).checked"
      :indeterminate="groupState(group).indeterminate"
      @update:model-value="(value: unknown) => setGroupChecked(group, Boolean(value))"
    >
      {{ t(group.groupI18nKey) }}
    </el-checkbox>
  </div>
  <el-table v-if="!isGroupCollapsed(group)" :data="group.rows" border row-key="pageId">
    <!-- page checkbox column and wrapping action checkbox column -->
  </el-table>
</section>
```

Render permission codes and disabled tags. Keep collapse state local. Use the approved reference CSS with Element Plus variables, stable two-column widths, wrapping actions, and no nested cards.

- [ ] **Step 5: Run the matrix component test and verify GREEN**

Run the Task 2 test command. Expected: all matrix component tests PASS.

### Task 3: Permission Diff Dialog and Linear Save Flow

**Files:**
- Create: `web/src/views/system/roles/components/RolePermissionDiffDialog.vue`
- Modify: `web/src/views/system/roles/index.vue`
- Modify: `web/src/views/system/roles/index.test.ts`

- [ ] **Step 1: Write failing tests for diff confirmation and normalized save**

Cover these observable behaviors with real local components:

```ts
await bodyButton('全选').trigger('click')
await bodyButton('保存授权').trigger('click')
expect(updateRolePermissionsMock).not.toHaveBeenCalled()
expect(document.body.textContent).toContain('确认权限变更')
expect(document.body.textContent).toContain('新增权限')

await bodyButton('确认').trigger('click')
expect(updateRolePermissionsMock).toHaveBeenCalledWith(3, { menuIds: [3] })
```

Add separate tests for removed labels, cancelling the diff while retaining selection, no-op save without PUT, empty selection `{ menuIds: [] }`, and failed confirmation retaining both dialogs, error text, and selection.

- [ ] **Step 2: Run the role page test and verify RED**

Run the Task 2 command. Expected: FAIL because saving still sends immediately and no diff Dialog exists.

- [ ] **Step 3: Implement the diff confirmation component**

Create a local 560px `el-dialog` with `append-to-body`, `defineModel<boolean>`, explicit string-array props, saving/error props, and a typed `confirm` emit. Render success tags for additions, danger tags for removals, separate empty states, an error alert, cancel, and loading confirm button.

- [ ] **Step 4: Replace tree state with matrix state in the role page**

Remove `ElTree`, `permissionTree`, `selectedMenuIDs`, tree check handlers, and `normalizedCheckedIDs`. Add:

```ts
const permissionGroups = computed(() => buildRolePermissionMatrix(permissionData.value?.menuTree ?? []))
const originalEffectiveMenuIDs = ref<number[]>([])
const selectedEffectiveMenuIDs = ref<number[]>([])
const permissionDiffVisible = ref(false)
const permissionDiff = ref<{ added: number[]; removed: number[] }>({ added: [], removed: [] })
```

During load, call `expandDirectMenuIDs(groups, data.menuIds)` and copy the result into both effective arrays. Provide global select/clear commands with `getRoleMatrixMenuIDs(groups)`.

Change the main save button to calculate `diffMenuIDs`. Close without PUT when unchanged; otherwise open the diff Dialog. Confirmation calls:

```ts
await updateRolePermissions(permissionData.value.role.id, {
  menuIds: normalizeDirectMenuIDs(permissionGroups.value, selectedEffectiveMenuIDs.value),
})
```

Only close both dialogs after save and list refresh succeed. On failure keep state and expose `permissionError` inside the diff Dialog. Do not reload access state or force-refresh the browser.

- [ ] **Step 5: Replace the tree template and add the diff Dialog**

Inside `.permission-scroll`, render a toolbar with global all/clear buttons followed by:

```vue
<RolePermissionMatrix
  v-model="selectedEffectiveMenuIDs"
  :groups="permissionGroups"
/>
```

Render `RolePermissionDiffDialog` as a sibling of the main authorization Dialog and derive exact diff labels from known matrix nodes as `translated name + code`; missing IDs are explicit internal errors, never `#ID` fallbacks.

- [ ] **Step 6: Run role page and matrix tests and verify GREEN**

Run:

```powershell
cd D:\admin\web
pnpm vitest run src/views/system/roles/role-permission-matrix.test.ts src/views/system/roles/index.test.ts
```

Expected: both test files PASS.

### Task 4: Verification and Scope Audit

**Files:**
- Verify all files changed in Tasks 1-3.

- [ ] **Step 1: Run forbidden-any check**

```powershell
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>" D:\admin\web\src -g "*.ts" -g "*.vue"
```

Expected: no output, exit code 1 from `rg` meaning no forbidden matches.

- [ ] **Step 2: Run all frontend tests**

```powershell
cd D:\admin\web
pnpm vitest run
```

Expected: all test files and tests PASS with exit code 0.

- [ ] **Step 3: Run the production build**

```powershell
cd D:\admin\web
pnpm build
```

Expected: `vue-tsc -b` and `vite build` both complete with exit code 0. Existing chunk-size warnings are non-blocking.

- [ ] **Step 4: Audit diff and formatting**

```powershell
cd D:\admin
git diff --check
git status --short
```

Expected: no whitespace errors. Only the approved spec/plan and role permission frontend files are changed; no backend, database, service, lockfile, or generated `dist` files are included.

No commit step is included because `AGENTS.md` reserves Git history operations for the repository owner.
