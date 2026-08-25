# 菜单 RBAC 与界面回收实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 恢复以 `sys_menu/sys_role_menu` 为唯一事实来源的菜单 RBAC，区分数据库名称与导航翻译键，并完成菜单管理和角色授权界面的样式回收。

**Architecture:** 后端继续使用 `router -> middleware -> handler -> service -> repository -> model -> PostgreSQL`，菜单变更继续通过 access invalidating lease 推进 PostgreSQL version 与 Redis generation。前端保留 `/system/menus` 静态组件挂载点，但菜单树、权限、面包屑和业务名称全部来自 access/menu/role API；其他页面继续按 `path + componentPath` 动态注册。

**Tech Stack:** Go、Gin、GORM、PostgreSQL、Redis、Vue 3、TypeScript、Pinia、Vue Router、vue-i18n、Element Plus、Vitest。

---

## 执行约束

- 在 `D:\admin` 当前工作区的 main 主线程执行，不创建 worktree，不使用 subagent。
- 使用 TDD：失败测试 -> 确认目标原因失败 -> 最小实现 -> 定向测试通过 -> 再重构。
- 不使用 Docker，不升级依赖，不引入 Adapter、Manager、Factory、BaseService 或通用 Repository。
- 不使用显式 `any`、`as any`、`any[]`、`Record<string, any>` 或 `@ts-ignore`。
- 计划中的 Git 标题只是 owner-controlled checkpoint。除非用户再次明确授权，不执行 `git add`、`git commit` 或 `git push`。
- 当前旧计划中的“软删除菜单管理节点、侧栏静态插入菜单、超级管理员硬塞权限”全部作废。

## 文件职责

### 后端

- `server/internal/module/menu/model.go`：`sys_menu/sys_role_menu` 映射，只定义字段。
- `server/internal/module/menu/schema.go`：菜单协议的显式 PostgreSQL 迁移、约束和索引。
- `server/internal/module/menu/foundation.go`：五个基础 RBAC 节点的定义、恢复、校验和保护判断。
- `server/internal/module/menu/request.go`、`response.go`：菜单管理 HTTP DTO。
- `server/internal/module/menu/tree.go`：名称、翻译键、节点形状和树结构校验。
- `server/internal/module/menu/service.go`：菜单写入、基础保护和 access generation 变更顺序。
- `server/internal/module/menu/repository.go`：菜单、历史基础节点和角色关系的 PostgreSQL 操作。
- `server/internal/module/access/repository.go`、`service.go`：从数据库角色关系生成 access 快照。
- `server/internal/module/role/permission.go`、`response.go`：角色授权树及其数据库名称。
- `server/cmd/api/main.go`：PrepareSchema、AutoMigrate、EnsureSchema、EnsureFoundation 的明确启动顺序。

### 前端

- `web/src/api/menu-fields.ts`：菜单名称、权限码、翻译键、路由和页面路径校验。
- `web/src/api/menu.contract.ts`：菜单管理的严格 DTO。
- `web/src/api/role.contract.ts`：角色授权树的严格 DTO。
- `web/src/router/access-routes.ts`：数据库 page 到 Vue route 的映射和静态页面绑定校验。
- `web/src/layout/components/AppAside.vue`：只渲染 Dashboard 与 access 菜单树。
- `web/src/layout/breadcrumbs.ts`：从完整 access 树解析导航层级。
- `web/src/layout/components/RouteTabs.vue`：优先从 access 树读取当前页面翻译键。
- `web/src/views/system/roles/role-permission-matrix.ts`：把数据库权限树转换成角色授权矩阵。
- `web/src/views/system/menus/index.vue`：树状菜单管理页面，不拆成通用 CRUD。

---

### Task 1: 落地菜单名称与可空翻译键的后端协议

**Files:**
- Modify: `server/cmd/api/main.go`
- Modify: `server/internal/module/menu/model.go`
- Modify: `server/internal/module/menu/schema.go`
- Modify: `server/internal/module/menu/request.go`
- Modify: `server/internal/module/menu/response.go`
- Modify: `server/internal/module/menu/tree.go`
- Modify: `server/internal/module/menu/service.go`
- Modify: `server/internal/module/menu/repository.go`
- Modify: `server/internal/module/access/repository.go`
- Modify: `server/internal/module/access/service.go`
- Modify: `server/internal/module/role/permission.go`
- Modify: `server/internal/module/role/response.go`
- Test: `server/internal/module/menu/schema_migration_test.go`
- Test: `server/internal/module/menu/schema_test.go`
- Test: `server/internal/module/menu/protocol_v2_test.go`
- Test: `server/internal/module/menu/tree_test.go`
- Test: `server/internal/module/menu/handler_test.go`
- Test: `server/internal/module/menu/repository_test.go`
- Test: `server/internal/module/menu/service_test.go`
- Test: `server/internal/module/access/repository_test.go`
- Test: `server/internal/module/access/service_test.go`
- Test: `server/internal/module/role/permission_test.go`
- Test: `server/internal/module/role/handler_test.go`
- Test: `server/internal/module/role/service_test.go`

- [ ] **Step 1: 先写 PostgreSQL 迁移失败测试**

把 `schema_migration_test.go` 的旧断言反转并补齐以下场景：

```go
func TestMenuProtocolMigrationAddsNamesAndNullableActionI18nKeys(t *testing.T) {
	connection, ctx := openMenuSchema(t)
	tx := openLegacyMenuSchema(t, connection.GORM, "name_protocol")

	if err := menu.PrepareSchema(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if err := tx.AutoMigrate(&menu.Menu{}, &menu.RoleMenu{}); err != nil {
		t.Fatal(err)
	}
	if err := menu.EnsureSchema(ctx, tx); err != nil {
		t.Fatal(err)
	}

	var rows []struct {
		Code     string
		MenuType menu.Type
		Name     string
		I18nKey  *string `gorm:"column:i18n_key"`
	}
	if err := tx.Unscoped().Table("sys_menu").Order("id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Name == "" {
			t.Fatalf("menu %s has no migrated name", row.Code)
		}
		if row.MenuType == menu.TypeAction && row.I18nKey != nil {
			t.Fatalf("action %s i18n_key = %v, want NULL", row.Code, row.I18nKey)
		}
	}
}
```

同时增加：

- `sys_menu.name` 是 `VARCHAR(128) NOT NULL`；
- `sys_menu.i18n_key` 允许 NULL；
- directory/page 的 i18nKey 必填，action 必须为 NULL；
- 迁移重复执行不重置 `is_hidden`；
- 首次新增 name 列时，存在未知 code 的历史节点会列出 code 并回滚；
- name 列已经存在时，管理员新增的非空自定义名称不会被映射表覆盖。

- [ ] **Step 2: 运行迁移测试并确认按目标原因失败**

```powershell
cd D:\admin\server
go test ./internal/module/menu -run 'TestMenuProtocolMigration|TestMenuSchema' -count=1
```

预期：因为没有 `PrepareSchema`、没有 `name` 且 action 的 `i18n_key` 仍为非空而失败。

- [ ] **Step 3: 修改模型和启动迁移顺序**

模型精确改为：

```go
type Menu struct {
	ID            int64          `gorm:"column:id;primaryKey;autoIncrement"`
	ParentID      *int64         `gorm:"column:parent_id"`
	MenuType      Type           `gorm:"column:menu_type;type:varchar(16);not null"`
	Name          string         `gorm:"column:name;type:varchar(128);not null"`
	Code          string         `gorm:"column:code;type:varchar(128);not null"`
	I18nKey       *string        `gorm:"column:i18n_key;type:varchar(128)"`
	Path          *string        `gorm:"column:path;type:varchar(255)"`
	ComponentPath *string        `gorm:"column:component_path;type:varchar(255)"`
	Icon          *string        `gorm:"column:icon;type:varchar(128)"`
	SortOrder     int            `gorm:"column:sort_order;type:integer;not null;default:0"`
	IsEnabled     yesno.Value    `gorm:"column:is_enabled;type:smallint;not null;default:1"`
	IsHidden      yesno.Value    `gorm:"column:is_hidden;type:smallint;not null;default:0"`
	CreatedAt     time.Time      `gorm:"column:created_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt     time.Time      `gorm:"column:updated_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt     gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz"`
}
```

在 `main.go` 中把菜单准备放到 AutoMigrate 之前：

```go
if err := menu.PrepareSchema(processContext, postgres.GORM); err != nil {
	return fmt.Errorf("prepare menu schema: %w", err)
}
```

启动顺序固定为：

```text
menu.PrepareSchema
-> database.AutoMigrate（保留 main.go 当前完整模型列表，其中包含 Menu 和 RoleMenu）
-> menu.EnsureSchema
-> 创建 access store/invalidator 和 menu service
-> menuService.EnsureFoundation（Task 2）
```

- [ ] **Step 4: 实现显式名称迁移**

`PrepareSchema` 只在已存在 `sys_menu` 时执行事务迁移。新增列、旧字段迁移和约束前数据整理使用明确 SQL：

```sql
ALTER TABLE sys_menu ADD COLUMN IF NOT EXISTS name VARCHAR(128) NULL;
ALTER TABLE sys_menu ADD COLUMN IF NOT EXISTS component_path VARCHAR(255) NULL;
ALTER TABLE sys_menu ADD COLUMN IF NOT EXISTS is_hidden SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE sys_menu ALTER COLUMN i18n_key DROP NOT NULL;
UPDATE sys_menu SET i18n_key = NULL WHERE menu_type = 'action';
UPDATE sys_menu SET is_hidden = 1 WHERE menu_type = 'action' AND is_hidden <> 1;
ALTER TABLE sys_menu ALTER COLUMN name SET NOT NULL;
```

只有本次新增 `name` 列时才使用下面的完整历史映射；不得用 code 或 i18nKey 兜底：

```go
var legacyMenuNames = map[string]string{
	"system": "系统管理",
	"system:menu:list": "菜单管理",
	"system:menu:create": "新增菜单",
	"system:menu:update": "修改菜单",
	"system:menu:delete": "删除菜单",
	"system:role:list": "角色管理",
	"system:role:create": "新增角色",
	"system:role:update": "修改角色",
	"system:role:status": "修改角色状态",
	"system:role:default": "设置默认角色",
	"system:role:delete": "删除角色",
	"system:role:authorize": "角色授权",
	"system:user:list": "用户管理",
	"system:user:update": "修改用户",
	"system:user:status": "修改用户状态",
	"system:user:delete": "删除用户",
	"system:user:roles": "分配用户角色",
	"system:auth-platform:list": "认证平台",
	"system:auth-platform:create": "新增认证平台",
	"system:auth-platform:update": "修改认证平台",
	"system:auth-platform:status": "修改认证平台状态",
	"system:auth-platform:delete": "删除认证平台",
	"system:session:list": "会话管理",
	"system:session:revoke": "踢出会话",
	"system:operation-log:list": "操作日志",
}
```

完成映射后查询 `name IS NULL OR btrim(name) = ''`，有结果就返回包含全部 code 的错误并让事务回滚。
旧 `migrateMenuProtocol` 保留 `view_key -> component_path` 的明确映射和删列逻辑，但删除
`retireLegacyMenuManagementNode` 调用及函数本身；结构迁移不得再软删除菜单或角色关系。

- [ ] **Step 5: 替换数据库形状约束**

`ck_sys_menu_shape` 精确包含名称和翻译键职责：

```sql
CHECK (
  btrim(name) <> ''
  AND (
    (menu_type = 'directory' AND i18n_key IS NOT NULL AND btrim(i18n_key) <> ''
      AND path IS NULL AND component_path IS NULL)
    OR
    (menu_type = 'page' AND i18n_key IS NOT NULL AND btrim(i18n_key) <> ''
      AND path IS NOT NULL AND btrim(path) <> ''
      AND component_path IS NOT NULL AND btrim(component_path) <> '')
    OR
    (menu_type = 'action' AND i18n_key IS NULL AND path IS NULL
      AND component_path IS NULL AND icon IS NULL AND is_hidden = 1)
  )
)
```

保留现有类型、排序、YesNo、外键和部分唯一索引；不重新创建 `view_key`。

- [ ] **Step 6: 贯通 menu HTTP 与业务类型**

输入和输出字段统一为：

```go
type CreateInput struct {
	ParentID *int64
	MenuType Type
	Name string
	Code string
	I18nKey *string
	Path *string
	ComponentPath *string
	Icon *string
	SortOrder int
	IsEnabled yesno.Value
	IsHidden yesno.Value
}

type UpdateInput struct {
	ParentID *int64
	MenuType Type
	Name string
	I18nKey *string
	Path *string
	ComponentPath *string
	Icon *string
	SortOrder int
	IsHidden yesno.Value
}
```

`createRequest/updateRequest` 增加必传 `name`，把 `i18nKey` 改为 `nullableString` 且必须显式出现。`managedMenuResponse` 在本 Task 返回：

```go
Name string `json:"name"`
I18nKey *string `json:"i18nKey"`
```

`validMenuName` 使用 `TrimSpace`、UTF-8 rune 计数和 128 字符限制。directory/page 要求合法非空 i18nKey；action 要求 nil。`/system/menus` 从通用禁止路径集合移除，`/login`、`/register`、`/dashboard` 继续禁止。

- [ ] **Step 7: 同步 access/role 的 Go 类型以保持编译期一致**

- `access.SourceMenu.I18nKey` 改为 `*string`；只有 directory/page 在构建 `MenuNode` 时解引用；action 必须为 nil。
- `role.PermissionTreeNode` 增加 `Name string` 并删除 `I18nKey`；`permissionTreeResponse` 同步返回 name；角色权限树校验所有节点的 name 非空。
- 更新所有 Go fixture：每个 `menu.Menu` 都填写 `Name`，directory/page 使用字符串指针 i18nKey，action 使用 nil。
- 不在这一步删除超级管理员硬编码权限；该行为由 Task 3 的独立失败测试驱动。

- [ ] **Step 8: 运行后端三个模块测试**

```powershell
cd D:\admin\server
go fmt ./...
go test ./internal/module/menu ./internal/module/access ./internal/module/role -count=1
go test ./cmd/api -count=1
```

预期：全部通过；数据库测试实际连接 PostgreSQL。不得用 SQLite 或内存数据库替代。

**Owner-controlled commit checkpoint:** `feat!: 调整菜单名称与翻译键协议`

---

### Task 2: 恢复并保护数据库基础 RBAC 节点

**Files:**
- Create: `server/internal/module/menu/foundation.go`
- Modify: `server/internal/module/menu/repository.go`
- Modify: `server/internal/module/menu/service.go`
- Modify: `server/internal/module/menu/response.go`
- Modify: `server/internal/module/menu/errors.go`
- Modify: `server/internal/shared/i18n/catalog.go`
- Modify: `server/cmd/api/main.go`
- Test: `server/internal/module/menu/service_test.go`
- Test: `server/internal/module/menu/repository_test.go`
- Test: `server/internal/module/menu/errors_test.go`
- Test: `server/internal/module/menu/schema_migration_test.go`

- [ ] **Step 1: 写基础节点恢复与保护失败测试**

使用现有 `openMenuTransaction`、真实 PostgreSQL 和 Redis access-state 测试辅助函数新增以下测试：

| 测试名 | 准备数据 | 精确结果 |
| --- | --- | --- |
| `TestEnsureFoundationRestoresDatabaseMenusAndMatchingRoleGrants` | 先用同一时间软删除 menu page/actions 和当前 role_menu，再插入一个更早删除的历史 grant | 恢复四个 menu 节点及同时间 grant；更早 grant 保持删除 |
| `TestEnsureFoundationCreatesMissingNodesAndIsIdempotent` | 只保留空菜单表和有效 access version | 第一次创建五个节点并推进 version；第二次零写入且 version 不变 |
| `TestEnsureFoundationRejectsConflictingActiveShape` | 创建 code 为 `system:menu:list` 但 path 错误的活动 page | 返回结构冲突，事务与 Redis state 均不改变 |
| `TestProtectedMenuAllowsDisplayUpdateOnly` | 完整基础树 | name/i18n/icon/sort 更新成功，结构字段保持原值 |
| `TestProtectedMenuRejectsStructureStatusAndDelete` | 完整基础树 | 改父级/类型/path/componentPath/hidden、禁用、删除均返回 `CodeMenuProtected` |

关键断言：

- 恢复后五个 code 都是活动 `sys_menu` 行；
- `system:menu:list` 的父级是 `system`，path/componentPath 精确匹配；
- action 的 i18nKey/path/componentPath/icon 为 NULL、isHidden=1；
- 只恢复 `sys_role_menu.deleted_at = 对应菜单错误删除时间` 的关系；更早历史关系仍软删除；
- 恢复或创建节点后推进所有受影响用户的 `sys_access_version` 和 Redis ready generation；
- 第二次执行不写数据库、不推进 version；
- 修改 name/i18n/icon/sort 成功，结构字段、状态和删除返回新保护错误。

- [ ] **Step 2: 运行测试并确认失败**

```powershell
cd D:\admin\server
go test ./internal/module/menu -run 'TestEnsureFoundation|TestProtectedMenu' -count=1
```

预期：因为没有基础定义、恢复逻辑和保护错误而失败。

- [ ] **Step 3: 创建唯一基础节点定义**

`foundation.go` 只定义五个真实节点：

```go
const FoundationSystemCode = "system"

var foundationMenus = []foundationDefinition{
	{Code: FoundationSystemCode, Type: TypeDirectory, Name: "系统管理", I18nKey: stringPointer("navigation.system"), Icon: stringPointer("Setting"), SortOrder: 100, IsEnabled: yesno.Yes, IsHidden: yesno.No},
	{Code: PermissionList, ParentCode: FoundationSystemCode, Type: TypePage, Name: "菜单管理", I18nKey: stringPointer("navigation.systemMenus"), Path: stringPointer("/system/menus"), ComponentPath: stringPointer("system/menus"), Icon: stringPointer("Menu"), SortOrder: 10, IsEnabled: yesno.Yes, IsHidden: yesno.No},
	{Code: PermissionCreate, ParentCode: PermissionList, Type: TypeAction, Name: "新增菜单", SortOrder: 10, IsEnabled: yesno.Yes, IsHidden: yesno.Yes},
	{Code: PermissionUpdate, ParentCode: PermissionList, Type: TypeAction, Name: "修改菜单", SortOrder: 20, IsEnabled: yesno.Yes, IsHidden: yesno.Yes},
	{Code: PermissionDelete, ParentCode: PermissionList, Type: TypeAction, Name: "删除菜单", SortOrder: 30, IsEnabled: yesno.Yes, IsHidden: yesno.Yes},
}
```

提供：

```go
func IsProtectedCode(code string) bool
func protectedValue(code string) yesno.Value
func (s *Service) EnsureFoundation(ctx context.Context) error
```

基础定义不是通用注册器，不接受运行时扩展，也不包含角色、用户等普通业务菜单。

- [ ] **Step 4: 实现历史恢复与创建**

Repository 增加明确方法：

```go
func (r *Repository) LockFoundationHistory(ctx context.Context, codes []string) ([]Menu, error)
func (r *Repository) RestoreFoundationRoleMenus(ctx context.Context, menuIDs []int64, deletedAtByMenu map[int64]time.Time, now time.Time) error
func (r *Repository) RestoreFoundationMenus(ctx context.Context, menuIDs []int64, now time.Time) error
```

恢复顺序必须是：

```text
锁 active menus/用户/access versions
-> Unscoped 锁基础节点历史
-> 校验 active/历史记录唯一
-> 先按相同 deleted_at 恢复 sys_role_menu
-> 再恢复 sys_menu
-> 创建完全缺失节点
-> 再构建完整树校验
-> 推进 PostgreSQL access version
-> 提交事务
-> 发布 Redis generation
```

任何唯一索引冲突、重复历史或父节点错误都返回显式错误并回滚。

- [ ] **Step 5: 实现后端保护错误**

新增错误：

```go
const CodeMenuProtected = 14010

func menuProtected(code string, cause error) *apperror.Error {
	return newMenuError(
		http.StatusConflict,
		CodeMenuProtected,
		i18n.KeyMenuProtected,
		map[string]string{"code": code},
		cause,
	)
}
```

中英文 catalog 明确为“基础菜单 {{code}} 不允许执行该操作”及对应英文。Service 规则：

- Update 基础节点时，结构字段必须与数据库当前值一致；
- Update 允许 name/i18n/icon/sortOrder；action 只允许 name/sortOrder；
- UpdateStatus 改变基础节点状态直接拒绝；
- Delete 基础节点直接拒绝；
- `ManagedMenu.IsProtected` 由 code 计算，不增加数据库列；`managedMenuResponse` 在本 Task 增加 `IsProtected int16 json:"isProtected"`。

- [ ] **Step 6: 在 API 启动前确保基础节点**

创建 `menuService` 后立即执行：

```go
if err := menuService.EnsureFoundation(processContext); err != nil {
	return fmt.Errorf("ensure foundation menus: %w", err)
}
```

该调用发生在 HTTP Server 启动前；Redis 故障或 generation 发布失败会阻止 API 启动，不允许带旧权限快照运行。

- [ ] **Step 7: 运行定向测试**

```powershell
cd D:\admin\server
go fmt ./...
go test ./internal/module/menu ./cmd/api -count=1
```

预期：全部通过。

**Owner-controlled commit checkpoint:** `fix: 恢复并保护菜单管理数据库节点`

---

### Task 3: 删除 access 中硬编码菜单权限

**Files:**
- Modify: `server/internal/module/access/service.go`
- Modify: `server/internal/module/access/repository.go`
- Test: `server/internal/module/access/service_test.go`
- Test: `server/internal/module/access/repository_test.go`
- Test: `server/internal/module/access/handler_test.go`
- Test: `server/internal/module/access/redis_test.go`

- [ ] **Step 1: 写数据库权限来源失败测试**

替换旧的 `TestBuildSnapshotAddsStaticMenuPermissionsForSuperAdminWithoutMenuRows`：

```go
func TestBuildSnapshotDoesNotInventPermissionsForSuperAdmin(t *testing.T) {
	snapshot, err := buildSnapshot(Source{
		Version: 1,
		RoleCodes: []string{"super_admin"},
		SuperAdmin: true,
		Menus: []SourceMenu{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.PermissionCodes) != 0 || len(snapshot.MenuTree) != 0 {
		t.Fatalf("snapshot invented database permissions: %+v", snapshot)
	}
}
```

另测：

- 超级管理员在基础节点存在时从 page/action 行获得四个 `system:menu:*` code；
- 普通角色只有 `sys_role_menu` 直接授权的 page/action 及有效祖先；
- action 的 nil i18nKey 合法，但 directory/page 的 nil i18nKey 让快照失败；
- access JSON 和 Redis cache 仍只包含 directory/page 且 i18nKey 为 string；
- `/system/menus` 是合法 access page path。

- [ ] **Step 2: 运行 access 测试并确认旧硬编码导致失败**

```powershell
cd D:\admin\server
go test ./internal/module/access -run 'TestBuildSnapshot|TestFindSourceWithVersion' -count=1
```

预期：空菜单超级管理员仍得到四个权限，因此失败。

- [ ] **Step 3: 删除硬编码追加并按节点类型校验 i18nKey**

从 `buildSnapshot` 删除：

```go
if source.SuperAdmin {
	permissionCodes = append(permissionCodes,
		menu.PermissionList,
		menu.PermissionCreate,
		menu.PermissionUpdate,
		menu.PermissionDelete,
	)
}
```

同时删除不再需要的 `menu` import。`validateSelectedMenus` 改为：

```go
switch item.MenuType {
case MenuDirectory, MenuPage:
	if item.I18nKey == nil || !validAccessI18nKey(*item.I18nKey) {
		return fmt.Errorf("menu %d has an invalid i18n key", id)
	}
case MenuAction:
	if item.I18nKey != nil {
		return fmt.Errorf("action menu %d has an i18n key", id)
	}
}
```

`buildMenuNode` 只接收 directory/page，显式检查指针后解引用。`staticAccessPagePaths` 只保留 `/login`、`/register`、`/dashboard`。

- [ ] **Step 4: 运行 access 完整测试**

```powershell
cd D:\admin\server
go fmt ./...
go test ./internal/module/access -count=1
```

预期：全部通过；Redis snapshot schema 不需要增加未使用的 `name`。

**Owner-controlled commit checkpoint:** `fix: 统一从数据库菜单生成访问权限`

---

### Task 4: 让前端角色授权矩阵显示数据库名称

**Files:**
- Modify: `web/src/api/role.contract.ts`
- Modify: `web/src/views/system/roles/role-permission-matrix.ts`
- Modify: `web/src/views/system/roles/components/RolePermissionMatrix.vue`
- Test: `web/tests/api/role.contract.test.ts`
- Test: `web/tests/views/system/roles/role-permission-matrix.test.ts`
- Test: `web/tests/views/system/roles/components/RolePermissionMatrix.test.ts`
- Test: `web/tests/views/system/roles/index.test.ts`

- [ ] **Step 1: 写角色授权名称失败测试**

使用 Task 1 已经落地的后端响应节点夹具：

```json
{
  "id": 3,
  "parentId": 2,
  "menuType": "action",
  "code": "system:menu:create",
  "name": "新增菜单",
  "isEnabled": 1,
  "children": []
}
```

前端测试把 locale 切换为 `en-US`，仍断言矩阵显示“系统管理 / 菜单管理 / 新增菜单”，同时页面控件文案可以变成英文。closed parser 必须拒绝旧 `i18nKey` 字段和缺少 `name` 的节点。

- [ ] **Step 2: 运行角色定向测试并确认失败**

```powershell
cd D:\admin\web
pnpm vitest run tests/api/role.contract.test.ts tests/views/system/roles/role-permission-matrix.test.ts tests/views/system/roles/components/RolePermissionMatrix.test.ts
```

预期：前端协议仍读取 i18nKey，矩阵仍调用 `t(...)`，测试失败。

- [ ] **Step 3: 修改前端角色权限 DTO 和矩阵类型**

```ts
export interface RolePermissionTreeNode {
  id: number
  parentId: number | null
  menuType: RolePermissionMenuType
  code: string
  name: string
  isEnabled: YesNo
  children: RolePermissionTreeNode[]
}
```

矩阵类型统一为 `groupName/pageName/action.name`，模板直接渲染：

```vue
<strong>{{ group.groupName }}</strong>
<strong>{{ row.pageName }}</strong>
<strong>{{ action.name }}</strong>
```

code 继续作为次要信息显示；数据库名称不经过 `t()`。

- [ ] **Step 4: 运行角色前端测试和构建**

```powershell
cd D:\admin\web
pnpm vitest run tests/api/role.contract.test.ts tests/views/system/roles/role-permission-matrix.test.ts tests/views/system/roles/components/RolePermissionMatrix.test.ts tests/views/system/roles/index.test.ts
pnpm build
```

预期：全部通过。

**Owner-controlled commit checkpoint:** `fix: 使用数据库名称展示角色权限`

---

### Task 5: 恢复数据库菜单的路由和布局链

**Files:**
- Modify: `web/src/api/menu-fields.ts`
- Modify: `web/src/router/access-routes.ts`
- Modify: `web/src/layout/components/AppAside.vue`
- Modify: `web/src/layout/breadcrumbs.ts`
- Modify: `web/src/layout/components/RouteTabs.vue`
- Modify: `web/src/layout/index.vue`
- Test: `web/tests/router/access-routes.test.ts`
- Test: `web/tests/router/index.test.ts`
- Test: `web/tests/layout/components/AccessMenuNode.test.ts`
- Test: `web/tests/layout/components/RouteTabs.test.ts`
- Test: `web/tests/layout/breadcrumbs.test.ts`
- Test: `web/tests/layout/index.test.ts`

- [ ] **Step 1: 写静态组件绑定但动态菜单来源的失败测试**

测试覆盖：

1. `AppAside` 不存在手工 `/system/menus` 菜单项；
2. access 树中的 `system -> system:menu:list` 在系统管理下自然渲染；
3. `/system/menus + system/menus + system:menu:list` 复用静态 route record，不执行 `addRoute`；
4. 相同 path 但错误 code/componentPath 明确抛 `ProtocolError`；
5. 其他数据库 page 继续动态 `addRoute`，失败时倒序清理；
6. 菜单管理面包屑为“系统管理 -> 菜单管理”，不走 path 特例；
7. 静态 route meta 即使是固定 key，RouteTabs 仍优先使用 access 节点的实时 i18nKey。

- [ ] **Step 2: 运行路由和布局测试确认失败**

```powershell
cd D:\admin\web
pnpm vitest run tests/router/access-routes.test.ts tests/router/index.test.ts tests/layout/components/AccessMenuNode.test.ts tests/layout/components/RouteTabs.test.ts tests/layout/breadcrumbs.test.ts tests/layout/index.test.ts
```

预期：静态 path 被当作重复、Aside 手工插入菜单、breadcrumb 使用特例而失败。

- [ ] **Step 3: 在动态路由注册器中校验静态绑定**

定义唯一绑定：

```ts
const staticPageBinding = {
  code: 'system:menu:list',
  path: '/system/menus',
  componentPath: 'system/menus',
  routeName: 'system-menus',
} as const
```

收集 page 时先判断 exact binding：

```ts
if (node.path === staticPageBinding.path) {
  if (
    node.code !== staticPageBinding.code
    || node.componentPath !== staticPageBinding.componentPath
    || !existingNames.has(staticPageBinding.routeName)
  ) {
    throw new ProtocolError(`access page ${node.code} conflicts with the static menu binding`)
  }
  paths.add(node.path)
  continue
}
```

精确绑定不加入 `pages`，所以不会重复 addRoute；其他 existing path 仍报错。`menu-fields.ts` 的静态禁止集合删除 `/system/menus`。

- [ ] **Step 4: 删除 Aside 和 breadcrumb 特例**

`AppAside.vue` 删除当前 `index="/system/menus"` 且依赖 `access.hasPermission('system:menu:list')`
的完整 `el-menu-item`，同时删除不再使用的 `DIcon` import。

保留 Dashboard，然后直接递归 `access.menuTree`。`resolveBreadcrumbs` 删除 `/system/menus` 分支，所有非 Dashboard 页面都查树。

- [ ] **Step 5: 让 RouteTabs 从 access 树取当前页面 key**

在 `breadcrumbs.ts` 导出严格查找函数：

```ts
export function findAccessPageByPath(
  routePath: string,
  menuTree: readonly AccessMenuNode[],
): AccessMenuNode | null
```

`RouteTabs.vue` 使用 access store：

```ts
const access = useAccessStore()

function currentI18nKey(): string {
  const accessPage = findAccessPageByPath(route.path, access.menuTree)
  if (accessPage !== null) return accessPage.i18nKey
  const matched = [...route.matched].reverse().find((record) => record.meta.i18nKey !== undefined)
  if (matched?.meta.i18nKey === undefined) {
    throw new Error(`Route ${route.fullPath} must declare i18nKey`)
  }
  return matched.meta.i18nKey
}
```

Dashboard 继续使用静态 meta。数据库页面包括菜单管理都优先使用 access i18nKey。

- [ ] **Step 6: 运行路由和布局测试**

```powershell
cd D:\admin\web
pnpm vitest run tests/router/access-routes.test.ts tests/router/index.test.ts tests/layout/components/AccessMenuNode.test.ts tests/layout/components/RouteTabs.test.ts tests/layout/breadcrumbs.test.ts tests/layout/index.test.ts
```

预期：全部通过。

**Owner-controlled commit checkpoint:** `fix: 恢复数据库菜单的导航链路`

---

### Task 6: 回收菜单管理表格与 Dialog 样式

**Files:**
- Modify: `web/src/api/menu-fields.ts`
- Modify: `web/src/api/menu.contract.ts`
- Modify: `web/src/api/menu.ts`
- Modify: `web/src/views/system/menus/index.vue`
- Modify: `web/src/i18n/messages/zh-CN.ts`
- Modify: `web/src/i18n/messages/en-US.ts`
- Test: `web/tests/api/menu.contract.test.ts`
- Test: `web/tests/api/menu.test.ts`
- Test: `web/tests/views/system/menus/index.test.ts`
- Test: `web/tests/i18n/index.test.ts`
- Test: `web/tests/components/DIcon/src/index.test.ts`
- Test: `web/tests/components/IconSelect/src/index.test.ts`

- [ ] **Step 1: 写菜单 DTO 和页面失败测试**

DTO 测试覆盖三种节点精确形状：

```ts
const action = {
  id: 3,
  parentId: 2,
  menuType: 'action',
  name: '新增菜单',
  code: 'system:menu:create',
  i18nKey: null,
  path: null,
  componentPath: null,
  icon: null,
  sortOrder: 10,
  isEnabled: YesNo.Yes,
  isHidden: YesNo.Yes,
  isProtected: YesNo.Yes,
  createdAt: '2026-08-25T00:00:00Z',
  updatedAt: '2026-08-25T00:00:00Z',
  children: [],
}
```

页面测试必须断言：

- 表格和父节点选项显示 `name`，不是 `t(i18nKey)`；
- 切换语言后数据库名称保持中文；
- directory/page/action 分别显示“目录名称/页面名称/权限名称”；
- action 不渲染 i18nKey、路由、页面路径、图标、是否隐藏；
- action payload 精确提交 `i18nKey:null/isHidden:1`；
- directory/page i18nKey 必须合法；所有类型 name 必填且不超过 128 字符；
- 基础节点可打开编辑，但结构控件禁用并显示保护原因；状态和删除命令禁用；
- `border`、浅色表头、全部列居中、固定操作列和足够宽度真实存在；
- 异步数据加载后全部节点真实展开，展开/收起按钮可切换；
- 使用 `AppDialog`、`DIcon`、`IconSelect`，不出现 Drawer 或手写图标表。

- [ ] **Step 2: 运行菜单前端测试确认失败**

```powershell
cd D:\admin\web
pnpm vitest run tests/api/menu.contract.test.ts tests/api/menu.test.ts tests/views/system/menus/index.test.ts
```

预期：缺少 name/isProtected、action 仍要求 i18nKey、页面仍翻译数据库内容而失败。

- [ ] **Step 3: 修改严格 DTO**

```ts
export interface ManagedMenuNode {
  id: number
  parentId: number | null
  menuType: ManagedMenuType
  name: string
  code: string
  i18nKey: string | null
  path: string | null
  componentPath: string | null
  icon: string | null
  sortOrder: number
  isEnabled: YesNo
  isHidden: YesNo
  isProtected: YesNo
  createdAt: string
  updatedAt: string
  children: ManagedMenuNode[]
}
```

Create/Update 输入增加 `name`，i18nKey 改为 `string | null`。closed keys 加入 name/isProtected；directory/page 校验字符串 key，action 校验 null。`menu-fields.ts` 增加：

```ts
export function isMenuName(value: string): boolean {
  return value.length > 0 && value.length <= 128 && value.trim() === value
}
```

- [ ] **Step 4: 重写表单状态和按类型字段矩阵**

```ts
interface MenuFormState {
  parentId: number | null
  menuType: ManagedMenuType
  name: string
  code: string
  i18nKey: string | null
  path: string | null
  componentPath: string | null
  icon: string | null
  sortOrder: number
  isEnabled: YesNo
  isHidden: YesNo
}
```

类型切换规则：

```ts
if (nextType === 'directory') {
  form.value.i18nKey ??= ''
  form.value.path = null
  form.value.componentPath = null
  form.value.isHidden = YesNo.No
} else if (nextType === 'page') {
  form.value.i18nKey ??= ''
  form.value.path ??= ''
  form.value.componentPath ??= ''
  form.value.isHidden = YesNo.No
} else {
  form.value.i18nKey = null
  form.value.path = null
  form.value.componentPath = null
  form.value.icon = null
  form.value.isHidden = YesNo.Yes
}
```

不要从 path 自动生成 componentPath。编辑 code 只读；受保护节点的 parent/type/path/componentPath/status/hidden 额外禁用。

- [ ] **Step 5: 按老项目基线整理树表格**

使用 Element Plus 官方属性：

```vue
<el-table
  ref="tableRef"
  v-loading="loading"
  :data="menus"
  row-key="id"
  border
  default-expand-all
  :header-cell-style="{ background: 'var(--el-fill-color-light)' }"
  :tree-props="{ children: 'children' }"
  table-layout="fixed"
>
```

每个 column 明确 `align="center" header-align="center"`。名称列显示 `row.name`，权限码/路由/页面路径用等宽文本。操作列 `fixed="right"`，宽度以四个可见命令不截断为准。

异步展开使用 `TableInstance`：

```ts
const tableRef = ref<TableInstance>()
const expanded = ref(true)

function applyExpansion(nodes: readonly ManagedMenuNode[], value: boolean): void {
  void nextTick(() => {
    const visit = (items: readonly ManagedMenuNode[]): void => {
      for (const item of items) {
        tableRef.value?.toggleRowExpansion(item, value)
        visit(item.children)
      }
    }
    visit(nodes)
  })
}

watch(menus, (nodes) => applyExpansion(nodes, expanded.value))
```

工具栏只保留清楚命令：新增根目录、全部展开/收起、刷新。不增加页面标题或嵌套 Card。

- [ ] **Step 6: 使用 AppDialog 和全宽协议输入**

- 父节点、类型使用 `el-select-v2`，不覆盖官方内部样式；
- 名称、i18nKey、code、path、componentPath 使用全宽 `el-input`；
- 图标使用已有 `DIcon + IconSelect`；
- 二元设置使用 Switch/Radio；
- Dialog 桌面 800-900px、移动端由 AppDialog 自适应；不设置固定高度；
- CSS 只包含双列/单列网格、间距、全宽和帮助文本：

```css
.menu-form__grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 16px; }
.menu-form__wide { grid-column: 1 / -1; }
.menu-form__control, .menu-icon-picker { width: 100%; }
.menu-form__hint { width: 100%; margin: 4px 0 0; color: var(--el-text-color-secondary); font-size: 12px; line-height: 1.5; }
@media (max-width: 768px) { .menu-form__grid { grid-template-columns: 1fr; } .menu-form__wide { grid-column: auto; } }
```

不得新增硬编码主题色、el-select-v2 深层样式、渐变或装饰卡片。

- [ ] **Step 7: 更新中英文界面文案**

两个语言包保持键集合完全一致，至少新增：

```text
menu.column.name
menu.form.directoryName
menu.form.pageName
menu.form.actionName
menu.form.nameRequired
menu.form.protectedHint
menu.expandAll
menu.collapseAll
```

保留并校对协议提示：

```text
i18nKey：至少两段点号路径，例如 navigation.systemUsers
权限码：小写冒号分段，例如 system:user:list
路由：必须以 / 开头，例如 /system/users
页面路径：不能以 / 开头，页面文件为 web/src/views/<页面路径>/index.vue
```

- [ ] **Step 8: 运行菜单 UI 和类型构建**

```powershell
cd D:\admin\web
pnpm vitest run tests/api/menu.contract.test.ts tests/api/menu.test.ts tests/views/system/menus/index.test.ts tests/i18n/index.test.ts tests/components/DIcon/src/index.test.ts tests/components/IconSelect/src/index.test.ts
pnpm build
```

预期：测试和 `vue-tsc -b`/Vite build 全部成功。

**Owner-controlled commit checkpoint:** `feat!: 重构数据库菜单管理界面`

---

### Task 7: 清理残留、全量验证与浏览器验收

**Files:**
- No planned runtime files; stale search results must be fixed back in their owning Task before continuing
- Inspect: `server/internal/module/menu`
- Inspect: `server/internal/module/access`
- Inspect: `server/internal/module/role`
- Inspect: `web/src`
- Inspect: `web/tests`
- Inspect: `docs/superpowers/specs/2026-08-24-menu-protocol-dynamic-page-design.md`
- Inspect: `docs/superpowers/plans/2026-08-25-menu-protocol-dynamic-page.md`

- [ ] **Step 1: 搜索旧错误协议和显示逻辑**

```powershell
cd D:\admin
rg -n "retireLegacyMenuManagementNode|AddsStaticMenuPermissions|static menu management|static-menu-management" server web\src web\tests -g "*.go" -g "*.ts" -g "*.vue"
rg -n "t\((row|node|action)\.i18nKey\)|groupI18nKey|pageI18nKey|action\.i18nKey" web\src\views\system\menus web\src\views\system\roles web\tests\views\system -g "*.ts" -g "*.vue"
rg -n "system:menu:(list|create|update|delete)" server\internal\module\access web\src\layout -g "*.go" -g "*.ts" -g "*.vue"
```

预期：

- 第一条无输出；
- 第二条无业务数据翻译残留；导航组件使用 `t(i18nKey)` 不属于该搜索范围；
- 第三条只允许静态 route 的 `requiredPermission`、静态绑定校验和后端测试数据，不能存在权限追加或 Aside 手工菜单。

- [ ] **Step 2: 搜索字段和 TypeScript 禁止写法**

```powershell
rg -n "viewKey|view_key|menuTitleKeys|routeViews|menuIcons" server web\src web\tests -g "*.go" -g "*.ts" -g "*.vue"
rg -n "\bas any\b|\bany\[\]|Record<[^>]*,\s*any>|@ts-ignore" web\src web\tests -g "*.ts" -g "*.vue"
```

预期：第一条只允许一次性历史迁移测试/SQL 中的 `view_key`；第二条本次修改文件无输出。

- [ ] **Step 3: 运行后端完整验证**

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

预期：四条命令全部成功。任何 PostgreSQL/Redis 环境失败如实报告，不得跳过后宣称通过。

- [ ] **Step 4: 运行前端完整验证**

```powershell
cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
```

预期：全部 Vitest 和生产构建成功。已有 Vue Router warning 或 chunk size warning 单独记录，不掩盖失败。

- [ ] **Step 5: 检查真实 PostgreSQL 结果**

```sql
SELECT column_name, data_type, is_nullable, character_maximum_length
FROM information_schema.columns
WHERE table_schema = current_schema() AND table_name = 'sys_menu'
ORDER BY ordinal_position;

SELECT id, parent_id, menu_type, name, code, i18n_key, path,
       component_path, is_enabled, is_hidden, deleted_at
FROM sys_menu
ORDER BY id;

SELECT role_id, menu_id, deleted_at
FROM sys_role_menu
WHERE menu_id IN (
  SELECT id FROM sys_menu
  WHERE code IN ('system:menu:list', 'system:menu:create', 'system:menu:update', 'system:menu:delete')
)
ORDER BY role_id, menu_id, id;
```

预期：

- `name` 非空，`i18n_key` 可空；
- 五个基础节点活动且结构正确；
- action 的 i18nKey 为 NULL、isHidden=1；
- 没有重新出现 `view_key`；
- 角色关系只恢复目标错误迁移删除的记录。

- [ ] **Step 6: 浏览器按两个用户视角验收**

超级管理员：

1. 侧边栏显示“系统管理 -> 菜单管理”，不存在独立一级菜单管理；
2. 菜单页显示系统管理、菜单管理和三个 action；
3. 可以编辑基础节点展示字段，不能修改结构、禁用或删除；
4. 新建普通 directory/page/action 后能在角色授权矩阵看到数据库名称；
5. 切换语言只改变导航与界面控件，菜单表和角色矩阵的数据库名称不变。

普通角色：

1. 无 `system:menu:list` 时侧栏不可见，直接访问 `/system/menus` 跳 Dashboard，API 返回 403；
2. 只授予 list 后可以查看，但没有 create/update/delete 命令；
3. 分别授予 action 后只出现对应命令；
4. 修改授权后按现有提示手动刷新，下次请求使用新 generation；
5. 隐藏 page 不出现在侧栏，但有权限时可以直接访问。

- [ ] **Step 7: 最终差异审查**

```powershell
cd D:\admin
git diff --check
git status --short
git diff --stat
git diff -- server/internal/module/menu server/internal/module/access server/internal/module/role web/src/api web/src/router web/src/layout web/src/views/system/menus web/src/views/system/roles docs/superpowers
```

确认：

- 没有 `.env`、密钥、`web/dist`、日志、测试缓存或临时截图；
- 没有无关格式化和业务改动；
- spec、plan、代码、测试使用相同字段和相同基础节点定义；
- 未经用户明确授权没有创建提交或推送。

- [ ] **Step 8: 交付报告**

报告分别列出：数据库迁移结果、基础节点/角色关系结果、后端四条命令、前端两条命令、浏览器双角色验收、warning、未运行项和剩余风险。

## 实施完成标准

- `sys_menu.name` 是真实非空数据库字段，action 的 `i18n_key` 为 NULL。
- 菜单管理五个基础节点和可恢复角色关系存在于 PostgreSQL，并受后端结构保护。
- access 不硬编码追加 `system:menu:*`，普通角色权限来自 `sys_role_menu`，超级管理员权限来自活动 `sys_menu`。
- `/system/menus` 只静态挂载组件；菜单、权限、breadcrumb 和 tab 标题仍从 access 数据生成。
- 菜单管理和角色授权显示数据库 name，导航显示 `t(i18nKey)`。
- 菜单页使用 Element Plus border/浅色表头/居中列、真实异步展开、AppDialog、DIcon 和 IconSelect。
- 后端 fmt/vet/test/build、前端全量 Vitest/build、真实 PostgreSQL 检查和双角色浏览器验收全部完成。
- Git 历史仍由仓库所有者控制。
