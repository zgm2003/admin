# Search 类型安全与菜单路径统一 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 AppSearch 改为可扩展的强类型搜索模型，并把数据库、接口快照、浏览器路由和源码目录统一为同一套页面路径契约。

**Architecture:** PostgreSQL `permission_menu.path/component_path` 是唯一页面路径事实来源；每个 page 节点满足 `path = /<componentPath>`，前端动态路由直接使用接口值，不再做旧路径映射。AppSearch 使用页面模型泛型和字段值映射，在组件入口校验非法数据，模板只渲染已验证值。

**Tech Stack:** Go 1.26、GORM、PostgreSQL、Vue 3.5、TypeScript 6、Vite 8、Vue Router 4、Element Plus 2.14、Vitest、pnpm。

**Spec:** `docs/superpowers/specs/2026-09-03-search-and-route-contract-design.md`

## Global Constraints

- `menuType=page` 必须满足 `path = /<componentPath>`，源码文件为 `web/src/views/<componentPath>/index.vue`。
- 前端直接消费 Access 快照中的 `path` 和 `componentPath`，禁止 `componentPathMap`、legacy alias、静默转换或根据 URL 猜测源码路径。
- 权限码、API URL、菜单 ID、父子关系、角色授权关系和隐藏语义不变。
- 数据库迁移必须幂等、可审计、冲突失败回滚；仅实际更新时推进 `permission_access_version`。
- 不处理未知 i18n key；不改变脏菜单节点的 fail-closed 策略；不移动测试目录旧 `src` 中间目录。
- AppSearch 暂只支持 `input`、`select-v2`、`date-range`；禁止显式 `any`、Schema DSL、万能表单抽象和类型断言掩盖字段缺失。
- 不操作 Redis；不自动 commit、fetch、pull、push 或重写历史。
- 每一步遵循失败测试 -> 最小实现 -> 定向验证；保留用户现有修改。

---

### Task 1: 建立路径和 AppSearch 的失败测试基线

**Files:**
- Modify: `web/tests/router/permission-routes.test.ts`
- Modify: `web/tests/router/index.test.ts`
- Modify: `web/tests/layout/breadcrumbs.test.ts`
- Modify: `web/tests/layout/components/PermissionMenuNode.test.ts`
- Modify: `web/tests/layout/components/RouteTabs.test.ts`
- Modify: `web/tests/components/AppSearch/index.test.ts`
- Modify: `server/internal/module/permission/menu/domain_rekey_test.go`
- Modify: `server/internal/database/admin_rbac_baseline_migration_test.go`

**Interfaces:**
- Consumes: 当前旧 `path/componentPath` 夹具和现有 AppSearch 公共类型。
- Produces: 明确的新路径断言、AppSearch 非法模型行为断言和数据库迁移回归断言，供后续实现逐项变绿。

- [ ] **Step 1: 记录当前路径契约失败**

将路由夹具中的旧值替换为目标值，并把断言改为：

```ts
page('permission:menu:view', '/permission/menus', 'permission/menus')
page('permission:role:view', '/permission/roles', 'permission/roles')
page('auth:platform:view', '/permission/auth-platforms', 'permission/auth-platforms')
page('account:user:loginlog:view', '/account/login-logs', 'account/login-logs')
page('storage:object:view', '/cloud/storage-object', 'cloud/storage-object')
```

执行：

```powershell
cd D:\admin\web
pnpm vitest run tests/router tests/layout --pool=threads --maxWorkers=1
```

预期：FAIL，原因是当前 `componentPathMap` 和旧测试路径仍只接受旧值。

- [ ] **Step 2: 增加 AppSearch 类型和运行时失败用例**

在 `web/tests/components/AppSearch/index.test.ts` 增加以下行为断言：

```ts
it('rejects a model value that does not match the field type without throwing during render', async () => {
  const wrapper = mount(AppSearch, {
    props: {
      modelValue: { keyword: ['bad'] },
      fields: [{ key: 'keyword', type: 'input', label: 'Keyword' }],
    },
    global: { plugins: [ElementPlus, appI18n] },
  })

  await wrapper.find('form').trigger('submit')
  expect(wrapper.emitted('query')).toBeUndefined()
  expect(wrapper.find('[role="alert"]').exists()).toBe(true)
})
```

同时增加一个 `OperationLogSearchModel` 类型夹具，要求 `field.key` 只能引用该模型的真实键，并执行：

```powershell
cd D:\admin\web
pnpm vitest run tests/components/AppSearch/index.test.ts --pool=threads --maxWorkers=1
```

预期：FAIL，当前组件会在渲染期抛出错误且 `SearchFormModel` 未约束真实键。

- [ ] **Step 3: 增加数据库迁移目标断言**

在 `domain_rekey_test.go` 和 `admin_rbac_baseline_migration_test.go` 中将页面断言改为新 `path/componentPath`，并增加重复执行后 access version 只增加一次的断言。先执行：

```powershell
cd D:\admin\server
go test ./internal/module/permission/menu ./internal/database -run 'Rekey|RBACBaseline' -count=1
```

预期：FAIL，当前迁移目标和 SQL 仍写入旧路径。

---

### Task 2: 在后端和 PostgreSQL 强制 path/componentPath 一致

**Files:**
- Modify: `server/internal/module/permission/menu/tree.go`
- Modify: `server/internal/module/permission/menu/schema.go`
- Modify: `server/internal/module/permission/menu/tree_test.go`
- Modify: `server/internal/module/permission/menu/service_test.go`
- Modify: `server/internal/module/permission/menu/schema_test.go`
- Modify: `server/internal/module/permission/menu/schema_migration_test.go`

**Interfaces:**
- Consumes: `CreateInput`、`UpdateInput`、`validateStoredMenu`、`validateInputShape` 和现有 `ck_permission_menu_shape`。
- Produces: 所有新建、更新、读取的 page 节点都拒绝不一致的 `path/componentPath`。

- [ ] **Step 1: 编写一致性校验测试**

为 create、update 和 stored-menu 测试增加三类用例：

```go
path := "/permission/roles"
component := "permission/roles"
valid := CreateInput{
    PlatformID: 1,
    MenuType: TypePage,
    Name: "角色管理",
    Code: "permission:role:view",
    I18nKey: stringPointer("navigation.accessRoles"),
    Path: &path,
    ComponentPath: &component,
    IsEnabled: yesno.Yes,
    IsHidden: yesno.No,
}

invalidPath := "/access/roles"
invalid := valid
invalid.Path = &invalidPath
if _, err := normalizeCreateInput(invalid); err == nil {
    t.Fatal("mismatched path/componentPath accepted")
}
```

同样验证 `componentPath` 以 `/` 开头、路径为空和 page 字段缺失时仍失败。

- [ ] **Step 2: 实现单一一致性判断**

在 `tree.go` 增加窄小的内部函数：

```go
func samePagePath(path, componentPath string) bool {
    return path == "/"+componentPath
}
```

在 `validateStoredMenu`、`validateInputShape` 的 `TypePage` 分支中，先通过现有格式校验，再要求 `samePagePath(*path, *componentPath)`；失败返回现有 `errMenuFields`，不新增兼容分支。

- [ ] **Step 3: 增加数据库 CHECK 约束并保持幂等替换**

将 `ck_permission_menu_shape` 的 page 条件扩展为：

```sql
menu_type = 'page'
AND i18n_key IS NOT NULL
AND path IS NOT NULL
AND component_path IS NOT NULL
AND btrim(path) <> ''
AND btrim(component_path) <> ''
AND path = '/' || component_path
```

`EnsureSchema` 继续通过现有约束替换逻辑安装约束；如果库中存在旧数据，先由 Task 3 的迁移脚本修正，约束安装失败必须回滚。

- [ ] **Step 4: 运行后端定向测试**

```powershell
cd D:\admin\server
gofmt -w internal/module/permission/menu/tree.go internal/module/permission/menu/schema.go
go test ./internal/module/permission/menu -run 'Menu|Tree|Schema|Foundation' -count=1
```

预期：新增一致性测试通过，旧的合法通用夹具（如 `/reports` + `reports`）继续通过。

---

### Task 3: 修正初始化 SQL、运行时旧数据迁移和迁移测试

**Files:**
- Modify: `docs/database/2026-08-29-admin-rbac-baseline.sql`
- Modify: `docs/database/2026-08-29-cos-object-storage.sql`
- Create: `docs/database/2026-09-03-page-path-alignment.sql`
- Modify: `server/internal/module/permission/menu/migration.go`
- Modify: `server/internal/module/permission/menu/domain_rekey_test.go`
- Modify: `server/internal/database/admin_rbac_baseline_migration_test.go`
- Create: `server/internal/database/page_path_alignment_migration_test.go`

**Interfaces:**
- Consumes: 当前菜单权限码、旧路径值、`permission_access_version` 和现有 SQL migration 测试辅助函数。
- Produces: 新装数据库和已有本地数据库都使用统一路径；重复执行安全且不改变授权关系。

- [ ] **Step 1: 先写迁移脚本测试和冲突测试**

在 `page_path_alignment_migration_test.go` 建立现有 PostgreSQL 测试 schema，插入旧页面行及 `permission_role_menu` 授权，读取并执行新 SQL，断言：

```go
want := map[string]struct{ path, componentPath string }{
    "permission:menu:view": {"/permission/menus", "permission/menus"},
    "permission:role:view": {"/permission/roles", "permission/roles"},
    "auth:platform:view": {"/permission/auth-platforms", "permission/auth-platforms"},
    "account:user:loginlog:view": {"/account/login-logs", "account/login-logs"},
    "storage:object:view": {"/cloud/storage-object", "cloud/storage-object"},
}
```

测试同时断言菜单 ID、父级、role-menu 行不变；目标路径被其他 page 占用时执行失败且旧值仍保留；再次执行不再增加 version。

运行并确认失败：

```powershell
cd D:\admin\server
go test ./internal/database -run 'PagePathAlignment' -count=1
```

- [ ] **Step 2: 编写幂等 SQL migration**

`docs/database/2026-09-03-page-path-alignment.sql` 使用 `BEGIN`/`COMMIT` 和一个 `DO $$` 块：

1. 对五个权限码逐一检查当前值只能是旧值或目标值；其他值立即 `RAISE EXCEPTION`。
2. 检查同一平台是否已有其他有效 page 占用目标 `path`，冲突时失败。
3. 只把旧值更新为目标值，并显式更新 `updated_at`。
4. 用 `GET DIAGNOSTICS ... ROW_COUNT` 汇总变更；变更大于 0 时执行一次 `UPDATE permission_access_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP`。
5. 不删除、重建菜单或 role-menu 行。

迁移脚本使用以下完整结构（不使用动态 SQL 或模糊匹配）：

```sql
BEGIN;

DO $$
DECLARE
  target RECORD;
  changed_rows BIGINT := 0;
  row_count BIGINT;
BEGIN
  FOR target IN
    SELECT *
    FROM (VALUES
      ('permission:menu:view', '/access/menus', 'access/menus', '/permission/menus', 'permission/menus'),
      ('permission:role:view', '/access/roles', 'access/roles', '/permission/roles', 'permission/roles'),
      ('auth:platform:view', '/access/auth-platforms', 'access/auth-platforms', '/permission/auth-platforms', 'permission/auth-platforms'),
      ('account:user:loginlog:view', '/account/login-logs', 'user/login-logs', '/account/login-logs', 'account/login-logs'),
      ('storage:object:view', '/cloud/object-storage', 'storage/object', '/cloud/storage-object', 'cloud/storage-object')
    ) AS mapping(code, old_path, old_component_path, new_path, new_component_path)
  LOOP
    IF EXISTS (
      SELECT 1
      FROM permission_menu
      WHERE deleted_at IS NULL
        AND code = target.code
        AND NOT (
          (path IS NOT DISTINCT FROM target.old_path AND component_path IS NOT DISTINCT FROM target.old_component_path)
          OR (path IS NOT DISTINCT FROM target.new_path AND component_path IS NOT DISTINCT FROM target.new_component_path)
        )
    ) THEN
      RAISE EXCEPTION 'menu % has an unexpected path shape', target.code;
    END IF;

    IF EXISTS (
      SELECT 1
      FROM permission_menu source
      JOIN permission_menu occupied
        ON occupied.platform_id = source.platform_id
       AND occupied.id <> source.id
       AND occupied.deleted_at IS NULL
       AND occupied.menu_type = 'page'
       AND occupied.path = target.new_path
      WHERE source.deleted_at IS NULL
        AND source.code = target.code
    ) THEN
      RAISE EXCEPTION 'menu % target path is occupied', target.code;
    END IF;

    UPDATE permission_menu
    SET path = target.new_path,
        component_path = target.new_component_path,
        updated_at = CURRENT_TIMESTAMP
    WHERE deleted_at IS NULL
      AND code = target.code
      AND path = target.old_path
      AND component_path = target.old_component_path;
    GET DIAGNOSTICS row_count = ROW_COUNT;
    changed_rows := changed_rows + row_count;
  END LOOP;

  IF changed_rows > 0 THEN
    UPDATE permission_access_version
    SET version = version + 1, updated_at = CURRENT_TIMESTAMP;
  END IF;
END $$;

COMMIT;
```

SQL 中的五个页面映射必须全部保留；路径检查使用能正确处理 NULL 的 `IS DISTINCT FROM` 条件组合。

- [ ] **Step 3: 修正初始化脚本和 Go 迁移目标**

将 SQL 初始值改为：

```text
permission:menu:view       /permission/menus       permission/menus
permission:role:view       /permission/roles       permission/roles
auth:platform:view         /permission/auth-platforms permission/auth-platforms
account:user:loginlog:view /account/login-logs     account/login-logs
storage:object:view        /cloud/storage-object   cloud/storage-object
```

在 `server/internal/module/permission/menu/migration.go` 的 `migratedPages` 中同步目标值。旧数据迁移仍保留“识别旧 catalog 后一次性改写”的能力，但写入的新 page 值必须满足统一契约。

- [ ] **Step 4: 运行迁移测试**

```powershell
cd D:\admin\server
go test ./internal/module/permission/menu ./internal/database -run 'Rekey|RBACBaseline|PagePathAlignment' -count=1
```

预期：旧数据可迁移、目标冲突回滚、重复执行幂等、授权关系保持。

---

### Task 4: 删除前端 componentPath 映射并统一动态路由 URL

**Files:**
- Modify: `web/src/router/permission-routes.ts`
- Modify: `web/tests/router/permission-routes.test.ts`
- Modify: `web/tests/router/index.test.ts`
- Modify: `web/tests/layout/breadcrumbs.test.ts`
- Modify: `web/tests/layout/components/PermissionMenuNode.test.ts`
- Modify: `web/tests/layout/components/RouteTabs.test.ts`
- Modify: `web/tests/layout/index.test.ts`
- Modify: `web/tests/views/permission/menus/filter-menu-tree.test.ts`
- Modify: `web/tests/views/permission/menus/index.test.ts`
- Modify: `docs/superpowers/specs/2026-08-24-menu-protocol-dynamic-page-design.md`
- Modify: `docs/superpowers/specs/2026-08-29-cos-object-storage-design.md`
- Modify: `docs/superpowers/plans/2026-09-02-frontend-code-quality-remediation.md`

**Interfaces:**
- Consumes: `PageModuleMap` 和 `PermissionMenuNode.path/componentPath`。
- Produces: `moduleKey(componentPath)` 直接返回 `../views/${componentPath}/index.vue`；动态路由使用接口 `path`，不再存在路径别名。

- [ ] **Step 1: 删除映射表并保留未知模块 fail-fast**

删除 `permission-routes.ts` 中的 `componentPathMap`，将函数固定为：

```ts
function moduleKey(componentPath: string): string {
  return `../views/${componentPath}/index.vue`
}
```

保留现有未知模块、重复 path、重复 route name 和字段不完整的 `ProtocolError`。

- [ ] **Step 2: 更新所有运行时和测试路径**

将 `/access/menus`、`/access/roles`、`/access/auth-platforms` 的运行时地址改为 `/permission/...`；将对象存储地址改为 `/cloud/storage-object`；登录日志的 componentPath 改为 `account/login-logs`。同步面包屑、RouteTabs、菜单管理和路由测试夹具。

不要修改权限码或 API URL。

- [ ] **Step 3: 删除旧路径声明**

更新当前有效 spec/plan 和数据库文档中的旧 componentPath/path 示例；历史文档只在仍被当前门禁或运行时引用时修改。确认运行时代码中不再出现：

```powershell
cd D:\admin
rg -n "componentPathMap|user/login-logs|storage/object|/access/(menus|roles|auth-platforms)|/cloud/object-storage" web/src server docs/database
```

允许唯一的旧值只能出现在新 migration 的“旧值检查”中及明确标注的迁移测试 fixture 中。

- [ ] **Step 4: 运行路由定向测试**

```powershell
cd D:\admin\web
pnpm vitest run tests/router tests/layout tests/views/permission/menus --pool=threads --maxWorkers=1
```

预期：所有页面按接口新值注册；旧 URL 不再注册或 redirect。

---

### Task 5: 建立 AppSearch 泛型模型和入口校验

**Files:**
- Modify: `web/src/components/AppSearch/types.ts`
- Modify: `web/src/components/AppSearch/index.vue`
- Modify: `web/src/components/AppSearch/index.ts`
- Modify: `web/tests/components/AppSearch/index.test.ts`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`

**Interfaces:**
- Consumes: `SearchFieldType = 'input' | 'select-v2' | 'date-range'` 和当前三种控件事件。
- Produces: `SearchField<TModel>`、`SearchFormModel<TModel>`、同模型类型的 `update:modelValue/query/reset`，以及可观察的非法模型状态。

- [ ] **Step 1: 定义可复用的值类型和键匹配类型**

在 `types.ts` 保留：

```ts
export type SearchDateRange = [] | [string, string]
export type SearchScalar = string | number | null | undefined
export type SearchFormValue = SearchScalar | SearchDateRange
```

增加键匹配辅助类型（不使用 `any`）：

```ts
type KeysMatching<T extends object, V> = {
  [K in keyof T]-?: Exclude<T[K], undefined> extends V ? K : never
}[keyof T] & string
```

`InputSearchField<T>` 和 `SelectSearchField<T>` 的 key 使用标量匹配键，`DateRangeSearchField<T>` 的 key 使用 `SearchDateRange` 匹配键；`SearchField<T>` 是三者联合。

`SearchFormModel<T extends object>` 保留模型键和值，不再默认让业务页面使用 `Record<string, SearchFormValue>`。

- [ ] **Step 2: 把 AppSearch 改为泛型 SFC**

使用 Vue 泛型 SFC（或等价的显式类型辅助）：

```vue
<script setup lang="ts" generic="TModel extends object">
const props = defineProps<{
  modelValue: TModel
  fields: SearchField<TModel>[]
  collapseCount?: number
}>()

const emit = defineEmits<{
  'update:modelValue': [value: TModel]
  query: [value: TModel]
  reset: [value: TModel]
}>()
</script>
```

内部编辑副本通过 `Partial<TModel>` 或明确的键值辅助类型维护；不得用 `Record<string, any>` 或 `as any`。

- [ ] **Step 3: 将校验移到入口并建立显式错误状态**

实现 `validateModel(fields, model): string | null`：

- 检查字段 key 存在；
- `input/select-v2` 只接受字符串、数字、`null`、`undefined`；
- `date-range` 只接受空数组或两个字符串；
- 非法模型设置 `validationError`，显示一个带 `role="alert"` 的本地化错误提示，并阻止 query/reset；
- 控件事件收到非法值时不写入模型；
- `inputValue/dateRangeValue/selectValue` 只读取已验证值，不再在模板执行 `throw`。

新增唯一通用文案：

```text
search.invalidModel = 搜索条件格式无效，请刷新页面后重试
search.invalidModel = Search criteria are invalid. Refresh and try again.
```

- [ ] **Step 4: 更新公共导出和组件测试**

从 `web/src/components/AppSearch/index.ts` 导出新的泛型类型。测试覆盖：三种字段、复制 emit、重置、折叠、非法初始值、非法控件事件和 `field.key` 类型约束。

运行：

```powershell
cd D:\admin\web
pnpm vitest run tests/components/AppSearch/index.test.ts --pool=threads --maxWorkers=1
pnpm typecheck
```

---

### Task 6: 迁移所有 AppSearch 页面到明确模型

**Files:**
- Modify: `web/src/views/account/users/index.vue`
- Modify: `web/src/views/account/sessions/index.vue`
- Modify: `web/src/views/account/login-logs/index.vue`
- Modify: `web/src/views/permission/auth-platforms/index.vue`
- Modify: `web/src/views/permission/roles/index.vue`
- Modify: `web/src/views/system/operation-logs/index.vue`
- Modify: `web/src/views/cloud/storage-object/components/ConfigTable/index.vue`
- Modify: `web/src/views/cloud/storage-object/components/RuleTable/index.vue`
- Create: `web/tests/views/account/login-logs/index.test.ts`
- Modify: `web/tests/views/account/users/index.test.ts`
- Modify: `web/tests/views/account/sessions/index.test.ts`
- Modify: `web/tests/views/permission/auth-platforms/index.test.ts`
- Modify: `web/tests/views/permission/roles/index.test.ts`
- Modify: `web/tests/views/system/operation-logs/index.test.ts`
- Modify: `web/tests/views/cloud/storage-object/index.test.ts`

**Interfaces:**
- Consumes: Task 5 的 `SearchField<TModel>` 和 `AppSearch` 泛型 emits。
- Produces: 页面拥有明确的 `*SearchModel`，不再通过宽泛 `SearchFormModel` 和重复 setter narrowing 适配字段。

- [ ] **Step 1: 为每页定义模型和默认值**

例如用户页：

```ts
interface UserSearchModel {
  keyword: string
  status: '' | YesNo
  role: number | ''
}

const searchModel = ref<UserSearchModel>({ keyword: '', status: '', role: '' })
```

为 sessions、login logs、auth platforms、roles、operation logs、COS config 和 upload rule 分别定义同页字段模型，保留现有默认值和查询参数语义。

- [ ] **Step 2: 删除重复 computed setter narrowing**

将以下模式删除：

```ts
set: (value) => {
  keyword.value = typeof value.keyword === 'string' ? value.keyword : ''
}
```

改为直接消费 `SearchModel`，在发起 API 查询前只做业务需要的空值/trim 归一化；字段类型错误由 AppSearch 入口校验负责。

- [ ] **Step 3: 给 fields 标注对应模型**

将 `SearchField[]` 改为 `SearchField<UserSearchModel>[]` 等具体类型，使不存在的 key 或错误控件类型在 `vue-tsc` 阶段失败。

- [ ] **Step 4: 回归查询、重置和分页**

```powershell
cd D:\admin\web
pnpm vitest run tests/views/account tests/views/permission tests/views/system tests/views/cloud/storage-object --pool=threads --maxWorkers=1
pnpm typecheck
```

确认查询参数、分页、重置、折叠和权限按钮行为与迁移前一致。

---

### Task 7: 更新当前文档和架构门禁

**Files:**
- Modify: `docs/superpowers/specs/2026-09-03-search-and-route-contract-design.md`
- Modify: `docs/superpowers/specs/2026-08-24-menu-protocol-dynamic-page-design.md`
- Modify: `docs/superpowers/specs/2026-08-29-cos-object-storage-design.md`
- Modify: `docs/superpowers/plans/2026-09-02-frontend-code-quality-remediation.md`
- Modify: `web/scripts/check-frontend-architecture.mjs`

**Interfaces:**
- Consumes: 已实现的路径契约、AppSearch 公共类型和动态路由文件。
- Produces: 文档与门禁不再要求旧路径或 `componentPathMap`，后续 AI 删除统一规则时会被检查发现。

- [ ] **Step 1: 清理当前有效文档中的矛盾示例**

将当前规范中的页面示例统一为 `path = /<componentPath>`；保留旧值只在迁移 SQL 的旧值表和迁移测试中。明确未知 i18n key、脏菜单 fail-closed、测试目录清理均不属于本次任务。

- [ ] **Step 2: 增加路径契约架构检查**

在架构脚本中扫描源码页面目录和运行时测试夹具，发现以下内容即报告新 finding：

```text
componentPathMap
旧运行时 path /access/menus、/access/roles、/access/auth-platforms、/cloud/object-storage
旧 componentPath user/login-logs、storage/object
```

同时保留现有 `request-error-notification-owner` 等规则，不把合法迁移 SQL 的旧值误报为运行时代码。

- [ ] **Step 3: 运行门禁**

```powershell
cd D:\admin\web
pnpm check:architecture
pnpm format:check
pnpm lint
```

---

### Task 8: 完整验证和交付检查

**Files:**
- No new source files; verify all files from Tasks 2–7。

**Interfaces:**
- Consumes: 完成的 PostgreSQL migration、后端菜单校验、前端 AppSearch 和动态路由。
- Produces: 可交付的路径协议和搜索类型安全实现，未自动提交 Git。

- [ ] **Step 1: 运行后端完整定向验证**

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

- [ ] **Step 2: 运行前端完整验证**

```powershell
cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
pnpm format:check
pnpm lint
pnpm check:architecture
pnpm typecheck
pnpm build
```

- [ ] **Step 3: 检查旧值和工作区**

```powershell
cd D:\admin
rg -n "componentPathMap|user/login-logs|storage/object|/access/(menus|roles|auth-platforms)|/cloud/object-storage" web/src server docs/database

git diff --check
git status --short
```

运行时代码不得出现旧路径；迁移脚本和明确标注的迁移测试可以保留旧值。确认没有回退用户改动、没有 Redis 操作、没有自动提交。

- [ ] **Step 4: 用户授权后的提交边界**

验证全部通过且用户明确要求提交后，拆成两个目的清晰的提交：

```powershell
git add server docs/database docs/superpowers/specs docs/superpowers/plans
git commit -m "refactor: 统一菜单页面路径契约"

git add web/src/components/AppSearch web/src/views web/src/router web/tests

git commit -m "refactor: 收紧 AppSearch 搜索模型类型"
```

未获授权时保持未提交状态。
