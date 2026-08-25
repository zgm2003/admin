# 菜单协议与动态页面映射 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 删除菜单标题、页面视图和图标的重复注册表，以 `i18nKey + path + componentPath + isHidden` 建立数据库菜单到 Vue 页面文件的显式线性映射。

**Architecture:** 后端继续使用 `router -> middleware -> handler -> service -> repository -> model -> PostgreSQL`，数据库迁移显式转换旧字段并失败关闭；访问快照继续由 PostgreSQL 生成并由 Redis 加速。前端使用严格 DTO 解析 `/api/v1/access`，静态保留 Dashboard 和菜单管理页，其余页面通过 `import.meta.glob('../views/**/index.vue')` 按 `componentPath` 注册；侧边栏只在最终渲染时应用 `isHidden`。

**Tech Stack:** Go、Gin、GORM、PostgreSQL、Redis、Vue 3、Vue Router、TypeScript strict、Pinia、vue-i18n、Element Plus、Vitest、pnpm。

---

## 实施边界与关键决策

- 设计依据：`docs/superpowers/specs/2026-08-24-menu-protocol-dynamic-page-design.md`。
- 不修改认证、会话、操作日志、角色授权树算法或 Redis 故障时回源 PostgreSQL 的现有策略。
- 不增加 Adapter、Manager、Factory、BaseService、BaseRepository、DI 容器或运行时页面注册中心。
- 不接受 `viewKey`、`titleKey` 等旧字段，不提供兼容分支。
- `componentPath` 不唯一；只有 page 的 `path` 保持有效数据唯一。
- `is_hidden` 使用 `SMALLINT`：`0 = 显示`，`1 = 隐藏`；action 固定为 `1`。
- `super_admin` 的 `system:menu:list/create/update/delete` 是菜单管理静态定位页所需的固定授权，由访问快照显式追加，不依赖 `sys_menu` 记录。
- 普通角色的权限仍只来自 `sys_role_menu -> sys_menu`。如果以后需要独立分配静态权限而又不创建菜单节点，另写 spec；本次不增加第二套角色权限表。
- 迁移时保留角色、用户、认证平台、会话和操作日志等现有菜单记录，并将其转换为普通可编辑数据；旧 `system:menu:list` 节点及其 action 子树、对应 `sys_role_menu` 关系使用软删除退出，因为 `/system/menus` 改为唯一静态页面。
- `/system/menus`、`/dashboard`、`/login`、`/register` 是静态路由，不能再次保存为动态 page 的 `path`。
- 不自动执行 Git commit、fetch、pull 或 push。每个任务后的提交命令只是仓库所有者明确授权时的检查点。

## 目标文件结构

```text
server/internal/module/menu/
  model.go                 # sys_menu 新字段
  schema.go                # 旧协议迁移和新约束
  protocol.go              # 权限常量，不再维护白名单/内置集合
  request.go               # componentPath/isHidden 严格请求
  response.go              # 新管理响应
  tree.go                  # 字段和树形规则
  service.go               # 菜单业务规则和缓存失效顺序
  repository.go            # 新列 PostgreSQL 读写

server/internal/module/access/
  repository.go            # 读取 component_path/is_hidden
  service.go               # 生成新快照并追加超级管理员静态权限
  redis.go                 # 新快照缓存 schema version

web/src/api/
  menu-fields.ts           # 前端菜单字段纯校验，无注册表
  menu.contract.ts         # 菜单管理严格 DTO
  access.contract.ts       # 访问快照严格 DTO

web/src/router/
  index.ts                 # Dashboard、菜单管理静态路由
  access-routes.ts         # componentPath -> import.meta.glob 页面

web/src/views/system/menus/index.vue
                              # 文本协议字段、IconSelect、DIcon、隐藏开关
```

### Task 1: 迁移 PostgreSQL 菜单协议

**Files:**
- Modify: `server/internal/module/menu/model.go`
- Modify: `server/internal/module/menu/schema.go`
- Modify: `server/internal/module/menu/schema_test.go`
- Create: `server/internal/module/menu/schema_migration_test.go`

- [ ] **Step 1: 为最终表结构和迁移行为编写失败测试**

在 `schema_test.go` 将 `sys_menu` 预期列改为：

```go
"component_path": {dataType: "character varying", nullable: "YES", length: 255},
"icon":           {dataType: "character varying", nullable: "YES", length: 128},
"is_hidden":      {dataType: "smallint", nullable: "NO"},
```

删除 `view_key` 预期，并增加明确的不存在断言：

```go
assertColumnMissing(t, connection, ctx, "sys_menu", "view_key")
```

新约束断言必须覆盖：

```go
checks := map[string][]string{
    "ck_sys_menu_type":       {"CHECK", "menu_type", "directory", "page", "action"},
    "ck_sys_menu_shape":      {"CHECK", "component_path", "is_hidden", "action", "icon"},
    "ck_sys_menu_sort_order": {"CHECK", "sort_order", "0"},
    "ck_sys_menu_is_enabled": {"CHECK", "is_enabled", "0", "1"},
    "ck_sys_menu_is_hidden":  {"CHECK", "is_hidden", "0", "1"},
}
```

在 `schema_migration_test.go` 使用临时 PostgreSQL schema 建立旧版 `sys_menu`/`sys_role_menu` 表，插入六种旧 `view_key` page 和一个 action，然后调用 `EnsureSchema`。断言：

```go
wantComponentPaths := map[string]string{
    "system:role:list":          "system/roles",
    "system:user:list":          "system/users",
    "system:auth-platform:list": "system/auth-platforms",
    "system:session:list":       "system/sessions",
    "system:operation-log:list": "system/operation-logs",
}
```

同时断言：旧 `system:menu:list` 子树和对应角色关系已软删除；其他 page 的 `is_hidden=0`；action 的 `is_hidden=1`；`view_key` 列不存在；未知旧 `view_key` 使 `EnsureSchema` 返回包含菜单 code 的错误并回滚全部 DDL/DML。

- [ ] **Step 2: 运行测试确认旧实现失败**

```powershell
cd D:\admin\server
go test ./internal/module/menu -run "TestMenuSchema|TestMenuProtocolMigration" -count=1
```

预期：失败原因是缺少 `component_path`/`is_hidden`、仍存在 `view_key`，且没有协议迁移。

- [ ] **Step 3: 修改 Go Model**

将 `Menu` 的渲染字段改为：

```go
Path          *string     `gorm:"column:path;type:varchar(255)"`
ComponentPath *string     `gorm:"column:component_path;type:varchar(255)"`
Icon          *string     `gorm:"column:icon;type:varchar(128)"`
SortOrder     int         `gorm:"column:sort_order;type:integer;not null;default:0"`
IsEnabled     yesno.Value `gorm:"column:is_enabled;type:smallint;not null;default:1"`
IsHidden      yesno.Value `gorm:"column:is_hidden;type:smallint;not null;default:0"`
```

保留显式 `CreatedAt`、`UpdatedAt`、`DeletedAt`，删除 `ViewKey`。
`is_hidden` 的 CHECK 只由后续 `EnsureSchema` 增加，不能写进新字段的 AutoMigrate tag；否则旧 action 在回填为 1 之前就可能因默认值 0 触发约束失败。

- [ ] **Step 4: 实现幂等、事务化协议迁移**

`EnsureSchema` 必须在一个 PostgreSQL 事务内按以下顺序调用：

```go
func EnsureSchema(ctx context.Context, db *gorm.DB) error {
    if db == nil {
        return fmt.Errorf("ensure menu schema requires a database")
    }
    return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        if err := migrateMenuProtocol(tx); err != nil {
            return err
        }
        if err := replaceMenuConstraints(tx); err != nil {
            return err
        }
        return ensureMenuIndexes(tx)
    })
}
```

`migrateMenuProtocol` 先通过 `information_schema.columns` 判断 `view_key` 是否存在。存在时执行显式映射：

```sql
UPDATE sys_menu
SET component_path = CASE view_key
    WHEN 'system-roles' THEN 'system/roles'
    WHEN 'system-users' THEN 'system/users'
    WHEN 'system-auth-platforms' THEN 'system/auth-platforms'
    WHEN 'system-sessions' THEN 'system/sessions'
    WHEN 'system-operation-logs' THEN 'system/operation-logs'
    WHEN 'system-menus' THEN 'system/menus'
END,
updated_at = CURRENT_TIMESTAMP
WHERE menu_type = 'page'
  AND component_path IS NULL;
```

更新前查询所有 `menu_type='page' AND component_path IS NULL` 的 code/view_key；任何值不在上述映射内就返回：

```go
fmt.Errorf("migrate menu %s: view_key %q has no component path mapping", row.Code, value)
```

然后：

1. 用递归 CTE 找到旧 `system:menu:list` 及其全部后代；先软删除有效 `sys_role_menu`，再软删除这些 `sys_menu`；
2. `UPDATE sys_menu SET is_hidden = CASE WHEN menu_type = 'action' THEN 1 ELSE 0 END`，包括软删除历史行，确保新 CHECK 对全表成立；
3. 删除 `ck_sys_menu_shape`、`ck_sys_menu_render_shape` 等引用 `view_key` 的旧约束；
4. `ALTER TABLE sys_menu DROP COLUMN view_key`；
5. 将 `component_path` 固定为 `VARCHAR(255)`、`icon` 固定为 `VARCHAR(128)`。

`replaceMenuConstraints` 每次按名称删除并重建形状约束，核心 SQL 为：

```sql
ALTER TABLE sys_menu ADD CONSTRAINT ck_sys_menu_shape CHECK (
    (menu_type = 'directory' AND path IS NULL AND component_path IS NULL)
 OR (menu_type = 'page'
     AND path IS NOT NULL AND btrim(path) <> ''
     AND component_path IS NOT NULL AND btrim(component_path) <> '')
 OR (menu_type = 'action'
     AND path IS NULL AND component_path IS NULL AND icon IS NULL AND is_hidden = 1)
);

ALTER TABLE sys_menu ADD CONSTRAINT ck_sys_menu_is_hidden
CHECK (is_hidden IN (0, 1));
```

不要为 `component_path` 创建唯一索引。

- [ ] **Step 5: 运行格式化和数据库定向测试**

```powershell
go fmt ./internal/module/menu
go test ./internal/module/menu -run "TestMenuSchema|TestMenuProtocolMigration" -count=1
```

预期：所有 schema 和迁移测试通过；未知旧页面映射测试明确失败并验证事务回滚。

**Owner-controlled commit checkpoint:** `feat!: 迁移菜单数据库协议`

### Task 2: 更新菜单管理后端协议并删除内置菜单机制

**Files:**
- Delete: `server/internal/module/menu/builtin.go`
- Delete: `server/internal/module/menu/builtin_test.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/internal/module/menu/protocol.go`
- Modify: `server/internal/module/menu/request.go`
- Modify: `server/internal/module/menu/response.go`
- Modify: `server/internal/module/menu/tree.go`
- Modify: `server/internal/module/menu/service.go`
- Modify: `server/internal/module/menu/repository.go`
- Modify: `server/internal/module/menu/errors.go`
- Modify: `server/internal/module/menu/errors_test.go`
- Modify: `server/internal/module/menu/tree_test.go`
- Modify: `server/internal/module/menu/service_test.go`
- Modify: `server/internal/module/menu/repository_test.go`
- Modify: `server/internal/module/menu/handler_test.go`
- Modify: `server/internal/shared/i18n/catalog.go`
- Modify: `server/internal/shared/i18n/catalog_test.go`

- [ ] **Step 1: 把新请求、响应和校验规则写成失败测试**

测试请求必须使用闭合字段集合：

```json
{
  "parentId": 1,
  "menuType": "page",
  "code": "system:user:list",
  "i18nKey": "navigation.systemUsers",
  "path": "/system/users",
  "componentPath": "system/users",
  "icon": "lucide:users",
  "sortOrder": 30,
  "isEnabled": 1,
  "isHidden": 0
}
```

更新请求删除 `code`/`isEnabled`，但必须包含 `componentPath` 和 `isHidden`。为以下行为分别增加测试：

- 自定义 `reports.orders.list` i18nKey 可创建和更新；
- `navigation`、`Navigation.users`、`navigation.system_users`、首尾空格和 129 字符被拒绝；
- `/reports/order-items` 与 `reports/order-items` 合法；
- path 缺 `/`、componentPath 带 `/`、扩展名、query、hash、动态参数、`.`、`..`、空段、末尾 `/` 被拒绝；
- `/login`、`/register`、`/dashboard`、`/system/menus` 被拒绝为动态 page 路径；
- 两个 page 可使用同一 `componentPath`，不能使用同一 `path`；
- icon 接受 `Setting`、`mdi:shield`，拒绝空字符串、首尾空格和超过 128 字符；
- directory/page 接受 `isHidden=0/1`，action 只接受 `isHidden=1`；
- 任何 active 菜单都能编辑、禁用和软删除，不再出现内置保护；
- 响应只有 `componentPath`、`isHidden`，没有 `viewKey`、`isBuiltin`。

- [ ] **Step 2: 运行菜单模块测试确认失败**

```powershell
cd D:\admin\server
go test ./internal/module/menu -run "Test.*(Request|Response|Create|Update|Status|Delete|Validation|Repository)" -count=1
```

预期：失败原因集中在旧 `viewKey`、白名单、`isBuiltin` 和缺少 `isHidden`。

- [ ] **Step 3: 收窄 protocol.go 到真实权限常量**

保留现有 `PermissionList` 至 `PermissionOperationLogList` 常量。删除：

```go
BuiltinSystemCode
BuiltinMenuListCode
menuTitleKeys
menuViewKeys
menuIconKeys
builtinCodes
IsMenuTitleKey
IsMenuViewKey
IsMenuIconKey
IsBuiltinCode
```

删除 `builtin.go`，并从 `cmd/api/main.go` 删除：

```go
if err := menuService.EnsureBuiltin(processContext); err != nil {
    return fmt.Errorf("ensure builtin menus: %w", err)
}
```

- [ ] **Step 4: 定义新菜单输入和响应结构**

`CreateInput`、`UpdateInput`、`ManagedMenu` 和 `UpdateValues` 使用一致字段：

```go
type CreateInput struct {
    ParentID      *int64
    MenuType      Type
    Code          string
    I18nKey       string
    Path          *string
    ComponentPath *string
    Icon          *string
    SortOrder     int
    IsEnabled     yesno.Value
    IsHidden      yesno.Value
}

type UpdateInput struct {
    ParentID      *int64
    MenuType      Type
    I18nKey       string
    Path          *string
    ComponentPath *string
    Icon          *string
    SortOrder     int
    IsHidden      yesno.Value
}
```

管理响应字段精确为：

```go
type managedMenuResponse struct {
    ID            int64                 `json:"id"`
    ParentID      *int64                `json:"parentId"`
    MenuType      Type                  `json:"menuType"`
    Code          string                `json:"code"`
    I18nKey       string                `json:"i18nKey"`
    Path          *string               `json:"path"`
    ComponentPath *string               `json:"componentPath"`
    Icon          *string               `json:"icon"`
    SortOrder     int                   `json:"sortOrder"`
    IsEnabled     int16                 `json:"isEnabled"`
    IsHidden      int16                 `json:"isHidden"`
    CreatedAt     string                `json:"createdAt"`
    UpdatedAt     string                `json:"updatedAt"`
    Children      []managedMenuResponse `json:"children"`
}
```

create/update request 的所有字段继续使用 pointer/`nullableString` 检测“缺失”和显式 `null`；旧 `viewKey` 或额外字段由现有严格 JSON 绑定直接拒绝。

- [ ] **Step 5: 实现不静默修正的字段校验**

在 `tree.go` 定义：

```go
var menuI18nKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:\.[a-z][a-zA-Z0-9]*)+$`)
var menuPathPattern = regexp.MustCompile(`^/[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?:/[a-z][a-z0-9]*(?:-[a-z0-9]+)*)*$`)
var menuComponentPathPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?:/[a-z][a-z0-9]*(?:-[a-z0-9]+)*)*$`)

var staticPagePaths = map[string]struct{}{
    "/login": {}, "/register": {}, "/dashboard": {}, "/system/menus": {},
}
```

校验函数必须检查原值，不调用 `TrimSpace` 后继续保存：

```go
func validMenuI18nKey(value string) bool {
    return utf8.RuneCountInString(value) <= 128 &&
        value == strings.TrimSpace(value) && menuI18nKeyPattern.MatchString(value)
}

func validMenuPath(value string) bool {
    _, reserved := staticPagePaths[value]
    return !reserved && utf8.RuneCountInString(value) <= 255 &&
        value == strings.TrimSpace(value) && menuPathPattern.MatchString(value)
}

func validMenuComponentPath(value string) bool {
    return utf8.RuneCountInString(value) <= 255 &&
        value == strings.TrimSpace(value) && menuComponentPathPattern.MatchString(value)
}
```

`validateInputShape` 接受 `(menuType, path, componentPath, icon, isHidden)`，规则为：

```text
directory: path/componentPath 必须为 null，icon 可选，isHidden 为 0 或 1
page:      path/componentPath 必须合法非空，icon 可选，isHidden 为 0 或 1
action:    path/componentPath/icon 必须为 null，isHidden 必须为 1
```

删除 `normalizeOptionalString` 的 trim 行为；非法首尾空格直接返回 `CodeMenuInvalidFields`。

- [ ] **Step 6: 更新 Service 和 Repository 的线性读写**

`Create`、`Update`、`sameMenuUpdate`、`buildManagedTree` 全部使用 `ComponentPath`/`IsHidden`。Repository SQL 明确列出：

```sql
INSERT INTO sys_menu (
    parent_id, menu_type, code, i18n_key, path, component_path,
    icon, sort_order, is_enabled, is_hidden
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
```

更新 map 使用：

```go
map[string]any{
    "parent_id": input.ParentID, "menu_type": input.MenuType,
    "i18n_key": input.I18nKey, "path": input.Path,
    "component_path": input.ComponentPath, "icon": input.Icon,
    "sort_order": input.SortOrder, "is_hidden": input.IsHidden,
    "updated_at": updatedAt.UTC(),
}
```

删除 `LockMenuTableForBuiltin`、`FindBuiltinRecords`、`validateBuiltinUpdate` 以及 Update/Status/Delete 中所有 `IsBuiltinCode` 分支。保留原有事务、祖先/后代校验、角色关系软删除和 access version 失效顺序。

- [ ] **Step 7: 删除内置保护错误和双语文案**

删除 `CodeMenuBuiltinProtected`、`menuBuiltinProtected`、`i18n.KeyMenuBuiltinProtected` 以及中英文 catalog 项和对应测试。不要重排其他已公开错误码；保留 `14006` 空位，现有 `14007+` 编号不变。

- [ ] **Step 8: 运行菜单模块和后端 i18n 测试**

```powershell
go fmt ./cmd/api ./internal/module/menu ./internal/shared/i18n
go test ./internal/module/menu ./internal/shared/i18n -count=1
```

预期：菜单和 i18n 测试全部通过，启动路径不再创建或校验内置菜单。

**Owner-controlled commit checkpoint:** `feat!: 更新菜单管理接口协议`

### Task 3: 更新访问快照并提供静态菜单管理授权

**Files:**
- Modify: `server/internal/module/access/repository.go`
- Modify: `server/internal/module/access/service.go`
- Modify: `server/internal/module/access/response.go`
- Modify: `server/internal/module/access/redis.go`
- Modify: `server/internal/module/access/repository_test.go`
- Modify: `server/internal/module/access/service_test.go`
- Modify: `server/internal/module/access/handler_test.go`
- Modify: `server/internal/module/access/redis_test.go`

- [ ] **Step 1: 编写新快照和超级管理员授权失败测试**

将 access node 的 JSON 键断言改为：

```go
want := []string{"code", "menuType", "path", "componentPath", "i18nKey", "icon", "isHidden", "children"}
```

增加以下测试：

```go
func TestBuildSnapshotAddsStaticMenuPermissionsForSuperAdminWithoutMenuRows(t *testing.T) {
    snapshot, err := buildSnapshot(Source{
        Version: 1, RoleCodes: []string{"super_admin"}, SuperAdmin: true, Menus: []SourceMenu{},
    })
    if err != nil {
        t.Fatal(err)
    }
    want := []string{
        "system:menu:create", "system:menu:delete", "system:menu:list", "system:menu:update",
    }
    if !reflect.DeepEqual(snapshot.PermissionCodes, want) {
        t.Fatalf("permissions = %v, want %v", snapshot.PermissionCodes, want)
    }
}
```

同时覆盖：隐藏 directory/page 仍在 `MenuTree`；action 不进入 `MenuTree` 但 code 进入 `PermissionCodes`；普通角色不会自动获得四个静态权限；重复 `componentPath` 合法，重复 `path` 失败；旧 Redis JSON 字段被拒绝。

- [ ] **Step 2: 运行 access 测试确认失败**

```powershell
cd D:\admin\server
go test ./internal/module/access -count=1
```

预期：失败原因是 Source/JSON 仍使用 `ViewKey`/`TitleKey`，且超级管理员空菜单源没有静态菜单权限。

- [ ] **Step 3: 修改 PostgreSQL Source 和查询**

`SourceMenu` 使用：

```go
type SourceMenu struct {
    ID            int64
    ParentID      *int64
    MenuType      MenuType
    Code          string
    I18nKey       string
    Path          *string
    ComponentPath *string
    Icon          *string
    SortOrder     int
    IsEnabled     yesno.Value
    IsHidden      yesno.Value
}
```

查询列精确改为：

```sql
SELECT id, parent_id, menu_type, code, i18n_key, path,
       component_path, icon, sort_order, is_enabled, is_hidden
FROM sys_menu
WHERE is_enabled = ? AND deleted_at IS NULL
ORDER BY sort_order, code, id
```

- [ ] **Step 4: 修改快照节点并严格校验**

`MenuNode` 改为：

```go
type MenuNode struct {
    Code          string     `json:"code"`
    MenuType      MenuType   `json:"menuType"`
    Path          *string    `json:"path"`
    ComponentPath *string    `json:"componentPath"`
    I18nKey       string     `json:"i18nKey"`
    Icon          *string    `json:"icon"`
    IsHidden      int16      `json:"isHidden"`
    Children      []MenuNode `json:"children"`
}
```

`validateSelectedMenus` 使用与菜单模块相同的字面规则校验 i18nKey、path、componentPath、icon 和 YesNo，不用白名单；只对 path 建唯一集合。`buildMenuTree` 继续跳过 action，且不因 `isHidden=1` 删除 directory/page。

- [ ] **Step 5: 显式追加超级管理员静态权限**

在收集 selected page/action code 后追加：

```go
if source.SuperAdmin {
    permissionCodes = append(permissionCodes,
        menu.PermissionList,
        menu.PermissionCreate,
        menu.PermissionUpdate,
        menu.PermissionDelete,
    )
}
permissionCodes, err = sortUniqueStrings(permissionCodes)
```

`access` 可以直接导入 `internal/module/menu` 的权限常量；不要复制四个字符串，不要给普通角色增加特殊分支。

- [ ] **Step 6: 升级 Redis 快照 schema version**

将缓存 schema version 从 `1` 升为 `2`，closed-record 键改为新 MenuNode 字段。旧缓存读取必须返回协议错误，`loadSnapshot` 按现有逻辑记录 cache error、回源 PostgreSQL 并发布 schema version 2；不要删除 PostgreSQL 保真回源。

- [ ] **Step 7: 运行 access 定向测试**

```powershell
go fmt ./internal/module/access
go test ./internal/module/access -count=1
```

预期：全部通过；Redis 命中、损坏回源、PostgreSQL 权威数据和 access generation 测试保持原行为。

**Owner-controlled commit checkpoint:** `feat!: 更新访问快照菜单协议`

### Task 4: 建立前端菜单字段校验和严格 DTO

**Files:**
- Create: `web/src/api/menu-fields.ts`
- Create: `web/tests/api/menu-fields.test.ts`
- Modify: `web/src/api/menu.contract.ts`
- Modify: `web/src/api/access.contract.ts`
- Modify: `web/tests/api/menu.contract.test.ts`
- Modify: `web/tests/api/access.contract.test.ts`
- Modify: `web/tests/api/menu.test.ts`
- Modify: `web/tests/store/access.test.ts`

- [ ] **Step 1: 编写字段校验与 closed-record 失败测试**

`menu-fields.test.ts` 使用表驱动覆盖与后端完全相同的合法/非法值。两个 contract 测试必须断言：

```ts
expect(() => parseManagedMenus([{ ...page, viewKey: 'system-users' }])).toThrow(ProtocolError)
expect(() => parseAccessSnapshot({ ...snapshotWithTitleKey })).toThrow(ProtocolError)
```

并断言新字段：

```ts
expect(parseManagedMenus([page])[0]).toMatchObject({
  i18nKey: 'navigation.systemUsers',
  path: '/system/users',
  componentPath: 'system/users',
  isHidden: YesNo.No,
})
```

- [ ] **Step 2: 运行 DTO 测试确认失败**

```powershell
cd D:\admin\web
pnpm vitest run tests/api/menu-fields.test.ts tests/api/menu.contract.test.ts tests/api/access.contract.test.ts tests/api/menu.test.ts tests/store/access.test.ts
```

预期：新测试因校验文件不存在和旧字段契约失败。

- [ ] **Step 3: 实现纯字段校验文件**

`menu-fields.ts` 不包含组件或页面映射，只导出：

```ts
export const menuI18nKeyPattern = /^[a-z][a-z0-9]*(?:\.[a-z][a-zA-Z0-9]*)+$/
export const menuCodePattern = /^[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?::[a-z][a-z0-9]*(?:-[a-z0-9]+)*)*$/
export const menuPathPattern = /^\/[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?:\/[a-z][a-z0-9]*(?:-[a-z0-9]+)*)*$/
export const componentPathPattern = /^[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?:\/[a-z][a-z0-9]*(?:-[a-z0-9]+)*)*$/

const staticPaths: ReadonlySet<string> = new Set(['/login', '/register', '/dashboard', '/system/menus'])

export function isMenuI18nKey(value: string): boolean {
  return value.length <= 128 && menuI18nKeyPattern.test(value)
}

export function isMenuPath(value: string): boolean {
  return value.length <= 255 && !staticPaths.has(value) && menuPathPattern.test(value)
}

export function isComponentPath(value: string): boolean {
  return value.length <= 255 && componentPathPattern.test(value)
}

export function isMenuIcon(value: string): boolean {
  return value.length > 0 && value.length <= 128 && value.trim() === value
}
```

JS length 和 Go rune count 对非 ASCII 的差异不影响这些 ASCII 格式字段；icon 的后端 rune 限制更宽松，前端仍不得截断或改写用户输入。

- [ ] **Step 4: 修改 menu.contract.ts**

DTO 使用普通 string 和 YesNo：

```ts
export interface ManagedMenuNode {
  id: number
  parentId: number | null
  menuType: ManagedMenuType
  code: string
  i18nKey: string
  path: string | null
  componentPath: string | null
  icon: string | null
  sortOrder: number
  isEnabled: YesNo
  isHidden: YesNo
  createdAt: string
  updatedAt: string
  children: ManagedMenuNode[]
}
```

`CreateMenuInput`/`UpdateMenuInput` 使用同名字段，删除 `viewKey`/`isBuiltin`。`parseMenuNode` 严格校验形状、类型、父子关系、排序和 path 唯一性；不要对 `componentPath` 建 Set。

- [ ] **Step 5: 修改 access.contract.ts**

```ts
export interface AccessMenuNode {
  code: string
  menuType: 'directory' | 'page'
  path: string | null
  componentPath: string | null
  i18nKey: string
  icon: string | null
  isHidden: YesNo
  children: AccessMenuNode[]
}
```

closed keys 精确为 `children, code, componentPath, i18nKey, icon, isHidden, menuType, path`。directory 的 path/componentPath 必须为 null；page 两者必须合法；icon 只做字符串规则校验；`isHidden` 必须为 YesNo。缺失翻译 key 不属于协议错误。

- [ ] **Step 6: 运行前端 DTO 测试**

```powershell
pnpm vitest run tests/api/menu-fields.test.ts tests/api/menu.contract.test.ts tests/api/access.contract.test.ts tests/api/menu.test.ts tests/store/access.test.ts
```

预期：全部通过，无旧字段兼容。

**Owner-controlled commit checkpoint:** `feat!: 更新前端菜单数据契约`

### Task 5: 静态注册菜单管理页并按 componentPath 加载动态页面

**Files:**
- Modify: `web/src/router/index.ts`
- Modify: `web/src/router/access-routes.ts`
- Modify: `web/src/permission.ts`
- Modify: `web/tests/router/index.test.ts`
- Modify: `web/tests/router/access-routes.test.ts`

- [ ] **Step 1: 编写静态路由、权限和动态加载失败测试**

测试必须覆盖：

1. 未加载 access 前，router 已有 `/dashboard` 和 `/system/menus`；
2. `/system/menus` 的 `requiredPermission` 为 `system:menu:list`；
3. 已认证但无 list 权限访问静态页跳 Dashboard，有权限时进入；
4. `/system/users` + `componentPath='system/users'` 读取 `../views/system/users/index.vue`；
5. 两个 URL 可复用一个 componentPath；
6. 缺失页面文件、重复 URL、重复 route name、静态路径冲突明确抛 `ProtocolError`；
7. 任一 `router.addRoute` 失败时倒序清理本批次已安装路由；
8. `isHidden=1` 不影响动态路由安装；
9. 冷启动直接访问动态 URL 仍完成认证、access 加载、路由安装和原 URL 恢复。

测试注入的页面 map 使用真实类型：

```ts
const views: PageModuleMap = {
  '../views/system/users/index.vue': async () => ({ default: { template: '<div>Users</div>' } }),
}
```

- [ ] **Step 2: 运行路由测试确认失败**

```powershell
cd D:\admin\web
pnpm vitest run tests/router/access-routes.test.ts tests/router/index.test.ts
```

预期：旧 `routeViews/viewKey` 实现无法满足 componentPath 和静态权限路由断言。

- [ ] **Step 3: 注册静态菜单管理路由**

Vue Router meta 改为：

```ts
declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth: boolean
    i18nKey?: string
    requiredPermission?: string
    affix?: boolean
  }
}
```

在 `admin-layout.children` 增加：

```ts
{
  path: 'system/menus',
  name: 'system-menus',
  component: () => import('../views/system/menus/index.vue'),
  meta: {
    requiresAuth: true,
    i18nKey: 'navigation.systemMenus',
    requiredPermission: 'system:menu:list',
  },
}
```

Dashboard meta 的 `titleKey` 同步改为 `i18nKey`。

- [ ] **Step 4: 实现构建期页面索引**

`access-routes.ts` 使用：

```ts
import type { Component } from 'vue'

export interface PageModule { default: Component }
export type PageModuleLoader = () => Promise<PageModule>
export type PageModuleMap = Readonly<Record<string, PageModuleLoader>>

const pageModules: PageModuleMap = import.meta.glob<PageModule>('../views/**/index.vue')

function moduleKey(componentPath: string): string {
  return `../views/${componentPath}/index.vue`
}
```

`collectPages` 用 `node.path` 生成 URL，用 `node.componentPath` 查 `pageModules[moduleKey]`，路由 name 继续是 `access:${node.code}`，meta 保存 `i18nKey: node.i18nKey`。不从 path 推导 componentPath，也不从 componentPath 推导 path。

- [ ] **Step 5: 在 guard 中执行静态页面权限判断**

access ready 且动态路由安装完成后，从最深 matched record 读取 `requiredPermission`：

```ts
const protectedRecord = [...to.matched]
  .reverse()
  .find((record) => record.meta.requiredPermission !== undefined)
const requiredPermission = protectedRecord?.meta.requiredPermission
if (requiredPermission !== undefined && !access.hasPermission(requiredPermission)) {
  return { name: 'dashboard' }
}
```

权限检查必须发生在 `access.load()` 之后；access 加载失败继续沿用当前明确 error 状态并只允许 Dashboard，不增加空权限兜底。

- [ ] **Step 6: 运行路由测试**

```powershell
pnpm vitest run tests/router/access-routes.test.ts tests/router/index.test.ts
```

预期：全部通过，动态页面缺失时没有残留 route。

**Owner-controlled commit checkpoint:** `feat: 按页面路径注册动态路由`

### Task 6: 更新侧边栏、面包屑和 RouteTabs 的动态标题与隐藏行为

**Files:**
- Modify: `web/src/layout/components/AppAside.vue`
- Modify: `web/src/layout/components/AccessMenuNode.vue`
- Modify: `web/src/layout/components/AppHeader.vue`
- Modify: `web/src/layout/components/RouteTabs.vue`
- Modify: `web/src/layout/breadcrumbs.ts`
- Modify: `web/src/layout/index.vue`
- Modify: `web/tests/layout/components/AccessMenuNode.test.ts`
- Modify: `web/tests/layout/components/RouteTabs.test.ts`
- Modify: `web/tests/layout/breadcrumbs.test.ts`
- Modify: `web/tests/layout/index.test.ts`

- [ ] **Step 1: 编写隐藏、图标和任意 i18nKey 的失败测试**

覆盖：

- AppAside 在 Dashboard 后显示静态“菜单管理”，但仅当 access 有 `system:menu:list`；
- hidden directory 整棵子树不渲染，子节点不提升；
- hidden page 不渲染；
- hidden page 仍能通过完整 access tree 生成面包屑；
- `DIcon` 接收 `Setting` 和 `mdi:shield` 字符串；无 `menuIcons` 组件映射；
- `reports.orders.list` 缺少语言包项时，侧栏、面包屑和 tab 明确显示 key；
- RouteTabs 和 Header 读取 route meta 的 `i18nKey`。

- [ ] **Step 2: 运行布局测试确认失败**

```powershell
cd D:\admin\web
pnpm vitest run tests/layout/components/AccessMenuNode.test.ts tests/layout/components/RouteTabs.test.ts tests/layout/breadcrumbs.test.ts tests/layout/index.test.ts
```

预期：旧 `titleKey/menuIcons` 和没有 `isHidden` 过滤导致失败。

- [ ] **Step 3: 使用 DIcon 并只在侧栏过滤隐藏节点**

`AccessMenuNode.vue` 删除 computed icon 和 `menuIcons` import，使用：

```vue
<template v-if="node.isHidden === YesNo.No">
  <el-sub-menu v-if="node.menuType === 'directory'" :index="node.code">
    <template #title>
      <DIcon v-if="node.icon !== null" :icon="node.icon" />
      <span>{{ t(node.i18nKey) }}</span>
    </template>
    <AccessMenuNode v-for="child in node.children" :key="child.code" :node="child" />
  </el-sub-menu>
  <el-menu-item v-else-if="node.path !== null" :index="node.path">
    <DIcon v-if="node.icon !== null" :icon="node.icon" />
    <template #title>{{ t(node.i18nKey) }}</template>
  </el-menu-item>
</template>
```

递归组件不提升隐藏目录的 children。`AppAside` 在 Dashboard 后增加静态菜单项：

```vue
<el-menu-item v-if="access.hasPermission('system:menu:list')" index="/system/menus">
  <DIcon icon="Menu" />
  <template #title>{{ t('navigation.systemMenus') }}</template>
</el-menu-item>
```

- [ ] **Step 4: 将展示链统一为 i18nKey:string**

`HeaderBreadcrumb`、RouteTabs tab model 和 Vue Router meta 统一使用：

```ts
i18nKey: string
```

静态 key 仍受 `zh-CN.ts`/`en-US.ts` 编译期约束；数据库动态 key 不强行断言为 `AppMessageKey`。`vue-i18n` 的 `fallbackLocale:false` 保持不变，让缺失 key 直接显示原 key。

- [ ] **Step 5: 运行布局测试**

```powershell
pnpm vitest run tests/layout/components/AccessMenuNode.test.ts tests/layout/components/RouteTabs.test.ts tests/layout/breadcrumbs.test.ts tests/layout/index.test.ts
```

预期：全部通过；隐藏只影响 sidebar，面包屑、tab 和直接路由仍使用完整 access tree。

**Owner-controlled commit checkpoint:** `feat: 支持隐藏菜单和动态标题`

### Task 7: 重构菜单管理页面表单和图标交互

**Files:**
- Modify: `web/src/views/system/menus/index.vue`
- Modify: `web/tests/views/system/menus/index.test.ts`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`

- [ ] **Step 1: 编写表单、提示、图标和提交 payload 失败测试**

测试必须断言：

- i18nKey 是 `el-input`，不是 `el-select-v2`；
- page 同时出现“路由”与“页面路径”文本输入，不出现“页面视图”；
- code、i18nKey、path、componentPath 下方提示与实际规则一致；
- directory/page 显示菜单显示开关，action 不显示且提交 `isHidden:1`；
- 类型切换会清空不兼容字段，但不自动把 path 生成 componentPath；
- `IconSelect` 选择字符串后使用 `DIcon` 回显并原样提交；
- 编辑任意旧记录不再受 `isBuiltin` 禁用；
- create/update payload 精确包含 componentPath/isHidden，不包含 viewKey/isBuiltin；
- 表格显示路由、页面路径、隐藏状态，删除“内置”列；
- 现有 `border`、`default-expand-all` 和列居中行为保留。

- [ ] **Step 2: 运行菜单页面测试确认失败**

```powershell
cd D:\admin\web
pnpm vitest run tests/views/system/menus/index.test.ts
```

预期：旧标题/视图选择器、菜单图标 map 和内置保护断言失败。

- [ ] **Step 3: 修改表单状态和类型切换**

```ts
interface MenuFormState {
  parentId: number | null
  menuType: ManagedMenuType
  code: string
  i18nKey: string
  path: string | null
  componentPath: string | null
  icon: string | null
  sortOrder: number
  isEnabled: YesNo
  isHidden: YesNo
}
```

`handleFormTypeChange` 规则：

```ts
if (nextType === 'directory') {
  form.value.path = null
  form.value.componentPath = null
} else if (nextType === 'page') {
  form.value.path = form.value.path ?? ''
  form.value.componentPath = form.value.componentPath ?? ''
} else {
  form.value.path = null
  form.value.componentPath = null
  form.value.icon = null
  form.value.isHidden = YesNo.Yes
}
```

directory/page 切换时保留用户明确选择的 `isHidden`；action 切回 directory/page 时显式恢复 `YesNo.No`，不把 action 的强制隐藏误当成用户选择。

- [ ] **Step 4: 实现字段布局和帮助文本**

桌面双列、移动单列不变；i18nKey、path、componentPath 添加 `menu-form__wide` 跨两列。控件结构使用：

```vue
<el-form-item class="menu-form__wide" :label="t('menu.form.i18nKey')">
  <div class="menu-form__control">
    <el-input v-model="form.i18nKey" data-testid="menu-form-i18n-key" />
    <p class="menu-form__hint">{{ t('menu.form.i18nKeyHint') }}</p>
  </div>
</el-form-item>
```

CSS 只负责布局：

```css
.menu-form__wide { grid-column: 1 / -1; }
.menu-form__control,
.menu-icon-picker { width: 100%; }
.menu-form__grid :deep(.el-input),
.menu-form__grid :deep(.el-select),
.menu-form__grid :deep(.el-input-number) { width: 100%; }
.menu-form__hint { margin: 6px 0 0; color: var(--el-text-color-secondary); font-size: 12px; line-height: 1.5; }
```

不设置固定 Dialog 高度，不新增嵌套 card。

- [ ] **Step 5: 使用 DIcon 和 IconSelect**

删除 `menuIcons`、图标 key cast 和页面级 `menuIconOptions`。表格和按钮使用：

```vue
<DIcon v-if="row.icon !== null" :icon="row.icon" />
<DIcon v-if="form.icon !== null" :icon="form.icon" />
<IconSelect v-model="iconSelectVisible" @select-icon="selectMenuIcon" />
```

选择函数只保存字符串：

```ts
function selectMenuIcon(value: string): void {
  form.value.icon = value
}
```

- [ ] **Step 6: 更新中英文文案**

至少新增或替换以下键，两个语言包保持完全同构：

```text
menu.column.route
menu.column.componentPath
menu.column.visibility
menu.visibility.visible
menu.visibility.hidden
menu.form.i18nKey
menu.form.path
menu.form.componentPath
menu.form.isHidden
menu.form.i18nKeyHint
menu.form.codeHint
menu.form.pathHint
menu.form.componentPathHint
```

中文提示精确为：

```text
i18nKey：至少两段点号路径，例如 navigation.systemUsers
权限码：小写冒号分段，例如 system:user:list
路由：必须以 / 开头，例如 /system/users
页面路径：不能以 / 开头，页面文件为 web/src/views/<页面路径>/index.vue
```

- [ ] **Step 7: 运行菜单页面和 i18n 测试**

```powershell
pnpm vitest run tests/views/system/menus/index.test.ts tests/i18n/index.test.ts tests/components/DIcon/src/index.test.ts tests/components/IconSelect/src/index.test.ts
```

预期：全部通过，菜单 UI 不再知道图标组件来源或页面组件映射。

**Owner-controlled commit checkpoint:** `feat: 重构菜单协议配置界面`

### Task 8: 删除旧注册文件并更新全部测试夹具

**Files:**
- Delete: `web/src/access/menu-title-keys.ts`
- Delete: `web/src/access/route-views.ts`
- Delete: `web/src/access/protocol.ts`
- Delete: `web/src/access/menu-icons.ts`
- Delete: `web/tests/access/menu-title-keys.test.ts`
- Delete: `web/tests/access/route-views.test.ts`
- Modify: `web/tests/layout/index.test.ts`
- Modify: `web/tests/layout/breadcrumbs.test.ts`
- Modify: `web/tests/layout/components/AccessMenuNode.test.ts`
- Modify: `web/tests/router/index.test.ts`
- Modify: `web/tests/router/access-routes.test.ts`
- Modify: `web/tests/store/access.test.ts`
- Modify: every remaining backend and frontend fixture returned by the searches below

- [ ] **Step 1: 搜索全部旧协议引用并记录清单**

```powershell
cd D:\admin
rg -n "viewKey|titleKey|menuTitleKeys|routeViews|hasRouteViewKey|menuIcons|IsMenuTitleKey|IsMenuViewKey|IsMenuIconKey|EnsureBuiltin|BuiltinMenu" server web\src web\tests -g "*.go" -g "*.ts" -g "*.vue"
rg -n "isBuiltin|IsBuiltin" server\internal\module\menu server\internal\module\access web\src\api\menu.contract.ts web\src\api\access.contract.ts web\src\views\system\menus web\tests\api\menu.contract.test.ts web\tests\api\access.contract.test.ts web\tests\views\system\menus
```

预期：执行前仍能看到待迁移的测试夹具和四个 `web/src/access` 文件；把每个结果归入本任务或前述任务，不忽略残留。认证平台自己的 `isBuiltin` 不在定向搜索路径中，也不属于菜单协议。

- [ ] **Step 2: 删除四个注册文件和两份注册表测试**

删除后如果 `web/src/access` 与 `web/tests/access` 为空，则删除空目录。不要把这些职责合并进 store 或万能 helper。

- [ ] **Step 3: 更新剩余夹具到统一节点形状**

所有前端 access page fixture 使用：

```ts
{
  code: 'system:user:list',
  menuType: 'page',
  path: '/system/users',
  componentPath: 'system/users',
  i18nKey: 'navigation.systemUsers',
  icon: 'User',
  isHidden: YesNo.No,
  children: [],
}
```

directory 使用 `path:null`、`componentPath:null`。后端 fixture 使用同名 Go 字段和 `yesno.No/Yes`，不使用零值来模糊 action 的固定隐藏规则。

- [ ] **Step 4: 确认旧协议完全消失**

```powershell
rg -n "viewKey|titleKey|menuTitleKeys|routeViews|hasRouteViewKey|menuIcons|IsMenuTitleKey|IsMenuViewKey|IsMenuIconKey|EnsureBuiltin|BuiltinMenu" server web\src web\tests -g "*.go" -g "*.ts" -g "*.vue"
rg -n "isBuiltin|IsBuiltin" server\internal\module\menu server\internal\module\access web\src\api\menu.contract.ts web\src\api\access.contract.ts web\src\views\system\menus web\tests\api\menu.contract.test.ts web\tests\api\access.contract.test.ts web\tests\views\system\menus
rg -n "view_key" server\internal\module\menu server\internal\module\access -g "*.go"
```

预期：前两条命令无输出。第三条只允许命中 `schema.go` 的一次性迁移 SQL 和 `schema_migration_test.go` 的旧表夹具；Model、Repository、Service、HTTP 和 access 快照不得再出现 `view_key`。认证平台自己的 `isBuiltin` 是不同业务协议，保留不动。

- [ ] **Step 5: 扫描 TypeScript 禁止写法**

```powershell
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>|@ts-ignore" web\src web\tests -g "*.ts" -g "*.vue"
```

预期：本次修改文件无输出；第三方测试类型也不使用显式 any 绕过。

**Owner-controlled commit checkpoint:** `refactor: 删除前端菜单协议注册表`

### Task 9: 全量验证、浏览器验收和差异审查

**Files:**
- No runtime files; inspect all changed files and tests

- [ ] **Step 1: 运行后端完整验证**

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

预期：四条命令全部成功。PostgreSQL 集成测试必须实际连接测试配置的 PostgreSQL，不能用内存数据库替代。

- [ ] **Step 2: 运行前端完整验证**

```powershell
cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
```

预期：全部 Vitest 通过，`vue-tsc -b` 和 Vite 生产构建成功；chunk size 警告可记录但不能掩盖测试或构建失败。

- [ ] **Step 3: 验证数据库真实迁移结果**

连接开发 PostgreSQL 后执行只读检查：

```sql
SELECT column_name, data_type, is_nullable, character_maximum_length
FROM information_schema.columns
WHERE table_schema = current_schema() AND table_name = 'sys_menu'
ORDER BY ordinal_position;

SELECT code, path, component_path, is_hidden, deleted_at
FROM sys_menu
ORDER BY id;
```

预期：无 `view_key`；存在 `component_path`/`is_hidden`；action 为 1；旧菜单管理节点已软删除；其他旧业务页面有明确 componentPath。

- [ ] **Step 4: 浏览器验收关键数据流**

使用超级管理员和普通用户分别验证：

1. 超级管理员无 `system:menu:list` 数据节点时仍看到并可进入静态菜单管理页；
2. 普通用户没有 list 权限时直接访问 `/system/menus` 返回 Dashboard；
3. 新建 directory/page/action，刷新当前用户后 access 快照按新字段返回；
4. page 的 path 与 componentPath 可不同，URL 和组件文件分别按各自字段工作；
5. 两个 URL 可共用一个页面文件；
6. 隐藏 page 可直接访问但不出现在侧栏；隐藏 directory 的子树不提升；
7. 缺失页面文件使整次动态路由安装失败并显示 access 协议错误；
8. 缺失 i18n 文案时界面明确显示 key；非法图标显示 DIcon 的问号空状态和开发日志；
9. 编辑、禁用、删除原有普通业务菜单不再出现“内置菜单”限制。

- [ ] **Step 5: 审查工作区和文档一致性**

```powershell
cd D:\admin
git diff --check
git status --short
git diff --stat
```

预期：`git diff --check` 无输出；变更只包含本计划、对应 spec、菜单/访问协议实现、前端页面与测试。确认没有 `.env`、密钥、`web/dist`、测试缓存、日志或无关格式化。

- [ ] **Step 6: 交付报告**

报告必须分别列出：数据库迁移结果、后端命令结果、前端命令结果、浏览器验收账号视角、未运行项目和剩余风险。不要自动提交；只有仓库所有者再次明确授权后，才按本计划的单一目的检查点创建提交。

## 实施完成标准

- PostgreSQL 不存在 `view_key`，新字段和约束与 spec 一致。
- Go 菜单管理和 access 快照不包含内置保护、标题/视图/图标白名单。
- `/system/menus` 是固定路由，超级管理员固定获得四个菜单管理权限。
- 其他页面只按 `path + componentPath` 动态注册，页面缺失明确失败。
- 隐藏仅发生在侧边栏渲染层，不改变快照、路由、面包屑或后端权限。
- 菜单页面使用普通 i18nKey/path/componentPath 输入、DIcon、IconSelect 和 YesNo 隐藏状态。
- 四个 `web/src/access` 注册文件和全部旧协议引用消失。
- 后端 `go vet/test/build`、前端 Vitest/build 和定向浏览器验收全部通过。
