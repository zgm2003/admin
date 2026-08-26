# Admin 业务域命名、菜单 RBAC 与动态页面协议设计

## 1. 状态

状态：用户已批准，等待按实施计划执行。

本文替代该文件此前以 `system:*`、`sys_*` 和单一“系统管理”目录为核心的设计。后续实施
计划必须以本文为准，不得继续执行旧命名方案。

## 2. 背景与定位

当前项目不是重新制作一个 Admin，而是对以下成熟项目进行革新式重构：

```text
旧前端：D:\github-project\admin_front_ts
旧后端：D:\github-project\admin_back_go
```

老项目已经验证的 RBAC 行为、认证平台能力、会话管理、操作日志、角色授权交互和紧凑型
Admin UI 应优先继承。当前项目负责替换老项目中过重的架构、MySQL 习惯、`is_del`、宽泛
DTO、隐式兜底和历史兼容包袱。

当前菜单实现已经具备多根目录、递归侧栏、动态路由、叶子授权和祖先闭包能力，但现有协议
仍存在以下问题：

- 用户、角色、认证、会话、菜单和日志全部堆在“系统管理”下，业务边界不清楚；
- `sys_*` 表名把所有资源伪装成一个系统模块，代码检索无法体现真实领域；
- 所有权限码使用 `system:*`，权限所有权被集中到 `menu/protocol.go`；
- 前端页面全部位于 `views/system/*`，与后端模块和数据库领域不一致；
- 菜单管理依赖静态路由冷启动，但静态入口被错误扩大成手工菜单和硬编码权限；
- 数据库可配置图标混用 Element Plus、MDI 和 Lucide，并可能依赖运行时网络；
- 菜单树默认全部展开，缺少面向未来 COS、钱包和 AI 模块的检索能力。

本设计把“命名规范”落实为跨数据库、Go、HTTP、JSON、TypeScript、Vue、Redis 和 Asynq 的
完整协议。同一业务概念在每一层采用符合该层语言习惯的名字，但必须共享同一个业务域，
使开发者可以从任意入口顺着命名快速定位整条调用链。

## 3. 设计原则

1. 业务名称优先于历史技术前缀。表、权限和页面目录表达真实资源，不继续滥用 `system`。
2. 菜单、页面和权限节点以 `rbac_menu` 为唯一事实来源，角色授权以 `rbac_role_menu` 为
   唯一事实来源。
3. 超级管理员也只从当前有效数据库节点获得权限，不硬编码追加任何权限码。
4. 正常请求保持 `router -> middleware -> handler -> service -> repository -> model -> PostgreSQL`。
5. 权限常量由使用它的业务模块拥有；菜单模块不充当全项目权限注册中心。
6. 数据库名称、约束、索引、序列、代码常量、目录和测试夹具必须一起改名。
7. 本项目尚未上线，采用一次性破坏性协议切换，不保留旧名称、旧路径或双版本兼容。
8. “破坏性”只用于切断旧协议；已有账号、角色、授权、菜单 ID 和有效开发数据仍原地保留。
9. 老项目的成熟行为和 UI 可以迁移，老项目的重型架构和兼容包袱不得迁移。

## 4. 目标

1. 主导航拆分为“用户与账号”“权限与认证”“系统管理”三个真实业务根目录。
2. 为未来“存储管理”“支付与钱包”“AI 管理”“AI 工作台”保留独立根目录能力，但本次
   不创建这些尚未落地的业务节点、表或页面。
3. 完成九张现有表及其 PostgreSQL 对象的一次性领域化重命名。
4. 完成后端权限常量、原始 SQL、Model、路由中间件和测试的同步改名。
5. 完成前端页面目录、组件路径、浏览器路由、权限判断、i18n 和测试的同步改名。
6. 保留菜单数据库 `name` 与导航 `i18nKey` 的职责分离。
7. 恢复正规的数据库菜单管理基础节点，并删除静态菜单和超级管理员硬编码权限。
8. 使用完全本地的 Lucide 菜单图标协议，提供搜索、预览、选择、清除和明确错误。
9. 菜单管理支持按名称、权限码和路径搜索，匹配结果保留祖先链。
10. 所有迁移、协议冲突和依赖失败均显式报错，不自动猜测或创建兼容别名。

## 5. 非目标

- 不实现 COS、上传、钱包、支付或 AI 业务；它们后续各自使用独立 spec 和 plan。
- 不把三个当前根目录重新包进一个总“平台”或总“系统”目录。
- 不修改 `/api/v1/users`、`/api/v1/roles`、`/api/v1/auth-platforms` 等已经具体、正确的资源 API。
- 不重命名认证协议中含义明确的 `platform` 字段，也不重命名 `authplatform` 业务模块。
- 不重命名语义正确的 `auth:*`、`authz:*` Redis 命名空间。
- 不实现语言包管理页面；开发者继续在前端语言包维护 navigation key。
- 不允许数据库配置任意模块 URL、动态 import 表达式或 JavaScript。
- 不改变角色叶子授权、祖先闭包、软删除和 PostgreSQL 作为权威事实来源的规则。
- 不复制老项目的 tabs 业务拆分、通用 Adapter、Manager、Factory、Base 层或运行时注册器。
- 不为未上线项目设计新旧版本滚动发布、双写、旧任务消费别名或长期迁移开关。

## 6. 信息架构与规范映射

### 6.1 导航根目录

根目录按以下顺序显示：

| 排序 | 根目录 | directory code | i18nKey | 建议图标 |
| --- | --- | --- | --- | --- |
| 100 | 用户与账号 | `account` | `navigation.account` | `lucide:users-round` |
| 200 | 权限与认证 | `access` | `navigation.access` | `lucide:shield-check` |
| 300 | 系统管理 | `system` | `navigation.system` | `lucide:settings-2` |

根目录 code 是稳定的树节点标识，不是权限码。directory 不进入 `permissionCodes`。

### 6.2 当前页面跨层映射

| 导航 | 页面 | page code/列表权限 | 浏览器路径 | componentPath | 页面目录 |
| --- | --- | --- | --- | --- | --- |
| 用户与账号 | 用户管理 | `account:user:list` | `/account/users` | `account/users` | `web/src/views/account/users` |
| 用户与账号 | 会话管理 | `auth:session:list` | `/account/sessions` | `account/sessions` | `web/src/views/account/sessions` |
| 权限与认证 | 菜单管理 | `rbac:menu:list` | `/access/menus` | `access/menus` | `web/src/views/access/menus` |
| 权限与认证 | 角色管理 | `rbac:role:list` | `/access/roles` | `access/roles` | `web/src/views/access/roles` |
| 权限与认证 | 认证平台 | `auth:platform:list` | `/access/auth-platforms` | `access/auth-platforms` | `web/src/views/access/auth-platforms` |
| 系统管理 | 操作日志 | `audit:operation-log:list` | `/system/operation-logs` | `system/operation-logs` | `web/src/views/system/operation-logs` |

API URL 继续按资源本身命名，而不是按侧栏目录命名：

```text
/api/v1/users
/api/v1/sessions
/api/v1/menus
/api/v1/roles
/api/v1/auth-platforms
/api/v1/operation-logs
```

侧栏目录是产品信息架构，API URL 是资源接口。二者不要求机械相同，但都必须在第 6.2 节
的规范映射中有唯一、可追踪的关系。

### 6.3 完整权限码

| 所有者模块 | 权限码 |
| --- | --- |
| `user` | `account:user:list`、`account:user:update`、`account:user:status`、`account:user:delete`、`account:user:roles` |
| `auth` | `auth:session:list`、`auth:session:revoke` |
| `menu` | `rbac:menu:list`、`rbac:menu:create`、`rbac:menu:update`、`rbac:menu:delete` |
| `role` | `rbac:role:list`、`rbac:role:create`、`rbac:role:update`、`rbac:role:status`、`rbac:role:default`、`rbac:role:delete`、`rbac:role:authorize` |
| `authplatform` | `auth:platform:list`、`auth:platform:create`、`auth:platform:update`、`auth:platform:status`、`auth:platform:delete` |
| `operationlog` | `audit:operation-log:list` |

权限码格式为：

```text
<business-domain>:<resource>:<action>
```

- 全部使用小写 kebab-case 分段；
- action 使用稳定动词，不使用页面按钮文案；
- page code 等于该资源的 `list` 权限；
- action code 是 page 的直接子节点；
- 禁止恢复 `system:user:*`、`system:role:*` 等旧前缀。

## 7. 全链路命名协议

### 7.1 各层编码规则

| 层 | 规则 | 示例 |
| --- | --- | --- |
| PostgreSQL 表、列、对象 | lower snake case | `rbac_role_menu`、`user_id`、`fk_rbac_role_menu_role` |
| Go 包名 | 小写真实资源名 | `user`、`role`、`authplatform`、`operationlog` |
| Go 导出标识符 | PascalCase，initialism 全大写 | `UserID`、`RBACMenu`、`HTTPServer` |
| Go 权限常量 | 模块内 `Permission<Action>` | `role.PermissionAuthorize` |
| JSON 字段 | lower camel case | `componentPath`、`isProtected` |
| TypeScript 类型与组件 | PascalCase | `ManagedMenuNode`、`RolePermissionMatrix` |
| TypeScript 变量与字段 | lower camel case | `permissionCodes`、`parentId` |
| Vue 页面目录 | lowercase plural path | `views/access/auth-platforms` |
| 浏览器 URL | lowercase plural kebab-case | `/access/auth-platforms` |
| componentPath | 与页面目录相同，不带前后斜杠 | `access/auth-platforms` |
| i18n key | 点号分组 + lower camel case | `navigation.accessAuthPlatforms` |
| 权限码 | 冒号分组 | `auth:platform:update` |
| Asynq type | 冒号分组 + schema version | `audit:operation-log:v2` |
| Redis key | 已确认的技术/协议域 | `authz:access-state:<userId>` |
| 数据库图标 | `lucide:` + kebab-case | `lucide:shield-check` |

同一字段跨语言时只改变语言惯例，不改变语义：

```text
PostgreSQL: component_path
Go:         ComponentPath
JSON/TS:    componentPath
```

禁止使用 `data1`、`info`、`itemInfo`、`manager`、`platformData` 等无法定位业务含义的名称。

### 7.2 后端代码所有权

每个模块在自己的 `protocol.go` 中声明并测试本模块权限常量：

```text
menu          -> rbac:menu:*
role          -> rbac:role:*
user          -> account:user:*
auth          -> auth:session:*
authplatform  -> auth:platform:*
operationlog  -> audit:operation-log:*
```

对应模块的 `route.go` 直接引用自身常量。删除 `menu/protocol.go` 中角色、用户、会话、认证平台
和操作日志的跨模块权限集合，不创建新的 shared permission registry。

基础菜单树由 API composition root 使用各模块导出的常量静态组装，再传给 menu service 校验
和落库。这只是编译期可追踪的启动组合，不是运行时注册器，也不改变模块依赖方向。历史
迁移中的 old code -> new code 字面映射只服务一次性迁移，不构成在线兼容协议。

### 7.3 代码资源名称

Go 使用包名提供领域上下文，Model 不重复堆叠表前缀。以下名称固定：

| 资源 | Go 包/主要 Model | Repository 命名 | TypeScript 命名 | PostgreSQL |
| --- | --- | --- | --- | --- |
| 用户账号 | `user.User` | `user.Repository`，方法使用 `User`/`Account` 真实动作 | `User*` DTO、`api/user.ts` | `user_account` |
| 会话 | `auth.Session` | `auth.SessionRepository` | `Session*` DTO、`api/session.ts` | `auth_session` |
| 菜单 | `menu.Menu` | `menu.Repository` | `ManagedMenu*`、`api/menu.ts` | `rbac_menu` |
| 角色 | `role.Role` | `role.Repository` | `Role*` DTO、`api/role.ts` | `rbac_role` |
| 用户角色关系 | `role.UserRole` | 由 `role`/`user` 的明确方法访问 | 不向页面暴露通用 join DTO | `rbac_user_role` |
| 角色菜单关系 | `menu.RoleMenu` | 由 `menu`/`role` 的明确方法访问 | 使用 `menuIds` 授权 DTO | `rbac_role_menu` |
| 访问版本 | `access.Version` | `access.Repository` 及明确的 version 方法 | `AccessSnapshot.version` | `rbac_access_version` |
| 认证平台 | `authplatform.Platform` | `authplatform.Repository` | `AuthPlatform*` DTO、`api/auth-platform.ts` | `auth_platform` |
| 操作日志 | `operationlog.OperationLog` | `operationlog.Repository` | `OperationLog*` DTO、`api/operation-log.ts` | `audit_operation_log` |

`user_account` 使用 `account` 后缀是为了表达它是登录账号存储，并避免模糊的数据库
`user` 表名；产品资源、Go 模块和 API 仍使用清楚的 `user`。这种差异必须由上表唯一映射，
不能在代码中再创造 `member`、`principal`、`operator` 等同义词。

Repository 方法以可验证的资源和动作命名，例如 `FindUserByID`、`ListSessions`、
`IncrementAccessVersions`。禁止 `GetData`、`SaveInfo`、`CommonQuery` 或依赖调用方猜语义的名称。

### 7.4 PostgreSQL 表名

| 旧表 | 新表 | 领域 |
| --- | --- | --- |
| `sys_user` | `user_account` | 账号 |
| `sys_user_session` | `auth_session` | 认证会话 |
| `sys_menu` | `rbac_menu` | RBAC 菜单/权限节点 |
| `sys_role` | `rbac_role` | RBAC 角色 |
| `sys_user_role` | `rbac_user_role` | RBAC 用户角色关系 |
| `sys_role_menu` | `rbac_role_menu` | RBAC 角色菜单关系 |
| `sys_access_version` | `rbac_access_version` | RBAC 访问快照版本 |
| `sys_auth_platform` | `auth_platform` | 认证平台业务资源 |
| `sys_operation_log` | `audit_operation_log` | 审计日志 |

主键、外键、CHECK、唯一约束、索引和自增序列同步改名。例如：

```text
sys_menu_pkey                 -> rbac_menu_pkey
fk_sys_menu_parent            -> fk_rbac_menu_parent
ux_sys_role_menu_active       -> ux_rbac_role_menu_active
sys_operation_log_id_seq      -> audit_operation_log_id_seq
```

迁移完成后，与这九张表关联的 PostgreSQL 对象名称不得残留 `sys_` 前缀。字段仍遵守：

- 每个 Model 显式声明非空 `CreatedAt`、`UpdatedAt`，数据库为 `TIMESTAMPTZ NOT NULL`；
- 有真实删除行为的表只使用 `gorm.DeletedAt`/`deleted_at`；
- 不创建 `is_del`、`is_deleted`、`BaseModel` 或 `gorm.Model`；
- Yes/No 使用 `SMALLINT` 和 `CHECK (value IN (0, 1))`，语义固定 `0 = No`、`1 = Yes`。

### 7.5 禁止兼容名称

本次切换后明确禁止：

- 旧表视图、旧表同义词、双表写入和旧字段别名；
- 旧权限码与新权限码同时授权；
- `/system/users`、`/system/roles`、`/system/menus` 等旧页面重定向；
- `system/*` 旧 componentPath 兼容表；
- 旧 Asynq type 的双重消费者；
- 前端同时判断 `system:*` 和新权限码；
- 为迁移成功而在代码中长期保留 `legacy` 分支。

## 8. `rbac_menu` 数据协议

### 8.1 字段

`rbac_menu` 最终字段为：

```text
id
parent_id
menu_type
name
code
i18n_key
path
component_path
icon
sort_order
is_enabled
is_hidden
created_at
updated_at
deleted_at
```

| 字段 | 规则 | 用途 |
| --- | --- | --- |
| `name` | 所有类型必填，去除首尾空格后 1-128 字符 | 菜单管理、角色授权等业务界面 |
| `code` | 所有类型必填，活动数据唯一，创建后不可修改 | 稳定标识；page/action 同时是权限码 |
| `i18n_key` | directory/page 必填，action 必须为 `NULL` | 导航翻译键 |
| `path` | 只有 page 必填 | 浏览器绝对路径 |
| `component_path` | 只有 page 必填 | `web/src/views/<componentPath>/index.vue` |
| `icon` | directory/page 可选，action 必须为 `NULL` | 本地 Lucide 图标名 |
| `is_hidden` | directory/page 可配置，action 固定 `1` | 只控制侧栏显示 |
| `is_enabled` | `0/1` | 控制节点及授权是否生效 |

节点形状：

| 类型 | 父级 | `i18n_key` | `path` | `component_path` | `icon` | `is_hidden` |
| --- | --- | --- | --- | --- | --- | --- |
| directory | 根为 `NULL`，否则必须是 directory | 必填 | `NULL` | `NULL` | 可选 | 可配置 |
| page | 必须是 directory | 必填 | 必填 | 必填 | 可选 | 可配置 |
| action | 必须是 page | `NULL` | `NULL` | `NULL` | `NULL` | `1` |

Service 和 PostgreSQL CHECK 两层均拒绝非法形状。action 不使用空字符串冒充 `NULL`。

### 8.2 名称、翻译键与路径

`name` 是数据库业务名称，例如“权限与认证”“菜单管理”“新增菜单”。菜单管理和角色授权
直接展示 `name`，切换语言不会改变这些业务数据。

`i18nKey` 只用于 directory/page 的侧栏、面包屑和页签，例如：

```text
navigation.access
navigation.accessMenus
```

格式为：

```text
^[a-z][a-z0-9]*(?:\.[a-z][a-zA-Z0-9]*)+$
```

page 的 `path` 和 `componentPath` 必须同时显式填写：

```text
path:          /access/menus
componentPath: access/menus
页面文件:      web/src/views/access/menus/index.vue
```

- `path` 以 `/` 开头，`componentPath` 不以 `/` 开头；
- 路径段使用小写 kebab-case；
- 不允许 query、hash、动态参数、`.`、`..`、空段或末尾斜杠；
- 不从一个字段自动猜测另一个字段；
- 页面文件缺失时动态路由安装明确失败，不加载空页面或 Dashboard 兜底。

### 8.3 受保护基础节点

菜单系统管理自身所需的基础节点为：

```text
access
└── rbac:menu:list
    ├── rbac:menu:create
    ├── rbac:menu:update
    └── rbac:menu:delete
```

它们仍是 `rbac_menu` 中的真实数据，不是静态权限。保护规则：

- 禁止禁用和删除；
- 禁止修改 `code/menuType/parentId/path/componentPath/isEnabled/isHidden`；
- directory/page 允许修改 `name/i18nKey/icon/sortOrder`；
- action 允许修改 `name/sortOrder`；
- 管理接口返回明确的 `isProtected`；
- 后端始终执行最终保护，不能依赖前端控件禁用。

其他页面和操作节点仍是普通数据库菜单。空数据库首次初始化时可以按第 6 节完整目录创建
当前内置 Admin 菜单；非空数据库启动时不得重新创建被管理员主动删除的普通节点。

## 9. 一次性迁移设计

### 9.1 未上线项目边界

本项目未上线，迁移在开发环境维护窗口执行：

```text
停止 API/Worker
-> 备份 PostgreSQL
-> 执行一次性数据库迁移
-> 清理项目 authz 缓存和旧 operation-log 任务
-> 启动新 API/Worker
-> 运行全量验证
```

不实现不停机迁移、旧二进制回滚兼容或新旧协议并存。不得使用 `FLUSHDB`；Redis 清理必须
限定到本项目 `authz:access-state:*`、`authz:access:*` 以及旧 operation-log task type。
认证会话和其他 Redis 数据不因本次 RBAC 改名被无关删除。

### 9.2 启动顺序

新的数据库启动顺序固定为：

```text
open PostgreSQL
-> run explicit domain-name migration
-> module PrepareSchema
-> AutoMigrate using new Model table names
-> module EnsureSchema using new table/object names
-> initialize Redis/queue dependencies
-> ensure protected menu foundation
-> build router/start server
```

当前 `AutoMigrate` 之前必须新增一个职责明确的一次性 domain-name migration。它不是通用
migration framework，只处理本文列出的九张表及其对象。这样更新后的 Model 不会提前创建
新表并造成新旧表并存。

### 9.3 表和对象重命名

重命名在一个 PostgreSQL 事务中完成，并使用显式映射：

1. 迁移前检查九组 old/new 表状态；
2. 全部旧表存在且新表不存在时执行迁移；
3. 全部新表存在且旧表不存在时视为已完成并只验证；
4. 空数据库由新版 AutoMigrate 创建新表；
5. 任何 old/new 同时存在、部分旧部分新或缺少关键关联表的状态都明确失败；
6. 按映射重命名表、主键、外键、CHECK、唯一约束、索引和序列；
7. 查询 PostgreSQL catalog，确认相关对象不再残留 `sys_` 名称；
8. 任一步失败时依靠事务整体回滚。

禁止用全仓字符串替换生成 SQL，也禁止用名称前缀猜测未知对象。实现计划必须先从当前
schema 和测试夹具枚举真实对象，再写明确映射和 PostgreSQL 集成测试。

### 9.4 菜单节点原地迁移

domain-name migration 完成表和对象改名后，`menu.PrepareSchema` 在 AutoMigrate 之前使用
独立的第二个 PostgreSQL 事务完成 `name` 列准备、节点重组和 access version 推进。这样
新版 Model 不会在旧数据尚未回填时尝试收紧非空约束。已有 menu row 按 ID 原地更新：

1. 复用当前 `system` 根节点 ID，将其更新为 `access`；
2. 新增 `account` 和新的 `system` 根节点；
3. 复用所有已有 page/action ID，按第 6.2、6.3 节更新 code、parent_id、path、
   component_path、i18n_key、icon 和 sort_order；
4. `rbac_role_menu.menu_id` 不变，因此已有叶子授权关系不需要重建；
5. 恢复此前被错误软删除的菜单管理子树时，只恢复与该错误删除时间匹配的角色关系；
6. 同一 old code 出现多个候选、目标 code 已被其他 ID 占用或父子形状冲突时整体失败；
7. 不创建旧 code 别名，不复制一份新 menu row；
8. 所有 `rbac_access_version` 行统一递增，保证旧权限快照不可继续使用。

`name` 字段继续按明确 old code -> 中文业务名称映射回填。无法映射的历史节点必须列出
code 并终止迁移，不能用 code 或 i18nKey 静默充当名称。

### 9.5 缓存与 Asynq 切换

- `accessstate.SchemaVersion` 和 access snapshot schema version 同步递增；
- 旧 access-state 解码失败后必须按 PostgreSQL 当前 version 重建；
- 旧 snapshot 因 schema/version 不同不可命中；
- 实施时定向删除现有 `authz:access-state:*` 和 `authz:access:*` 开发缓存；
- Redis key 继续使用语义正确的 `authz:*`，不为外观改名；
- operation log Asynq type 从 `system:operation-log:v2` 直接改为
  `audit:operation-log:v2`；
- payload 仍是已经确认的完整、脱敏、闭合、`schemaVersion = 2` 审计 DTO；
- 停止旧 Worker 后定向清除旧 type 的开发任务，不注册旧 handler 或任务别名；
- 新 Worker 只注册 `audit:operation-log:v2`。

## 10. 后端架构与数据流

### 10.1 菜单管理

```text
router -> auth/access middleware -> menu handler -> menu service
       -> menu repository -> rbac_menu/rbac_role_menu -> PostgreSQL
```

- Handler 只绑定 DTO、传递 request context 和输出 `code/data/message`；
- Service 校验名称、节点形状、基础保护、树关系和缓存失效顺序；
- Repository 只执行 PostgreSQL 查询、事务、锁和软删除；
- 正常在线菜单变更继续使用 access invalidating lease；
- PostgreSQL 提交后发布新 access state，失败时明确返回依赖错误；
- 不把迁移 SQL塞入 Handler/Service 的普通请求路径。

### 10.2 当前用户访问快照

```text
GET /api/v1/access
-> auth middleware
-> access handler
-> access service
-> access repository
-> rbac_user_role + rbac_role_menu + rbac_menu + rbac_access_version
-> PostgreSQL
-> Redis snapshot cache
-> response
```

普通角色从 `rbac_role_menu` 的直接叶子授权出发，补齐有效祖先闭包。page/action code 进入
`permissionCodes`，directory 只进入菜单树。超级管理员从所有有效 page/action 行获得权限，
必须删除当前 access service 硬编码追加菜单权限的逻辑。

数据库缺少 `rbac:menu:list` 时，任何用户都不会凭空获得该权限；启动基础节点校验应更早
阻止 API 启动。

### 10.3 权限路由

每个模块的 route 只使用本模块权限常量，例如：

```text
user routes          -> user.PermissionList
role routes          -> role.PermissionAuthorize
auth session routes  -> auth.PermissionSessionRevoke
authplatform routes  -> authplatform.PermissionList
operationlog routes  -> operationlog.PermissionList
menu routes          -> menu.PermissionCreate
```

禁止 route 中继续散落权限字符串，也禁止从 menu 模块引用其他业务权限。

## 11. 前端动态页面与导航

### 11.1 页面移动

现有页面按第 6.2 节移动物理目录，并同步更新：

- `import.meta.glob('../views/**/index.vue')` 的 componentPath 匹配；
- 静态菜单管理 route record；
- route name、meta i18nKey 和 requiredPermission；
- API 调用页面 import；
- 单元测试目录和 fixture；
- 面包屑、RouteTabs、权限按钮和直接访问测试。

API 客户端文件仍按资源命名，例如 `api/menu.ts`、`api/role.ts`，不按侧栏目录复制一层
account/access/system API wrapper。

### 11.2 静态冷启动边界

Vue Router 静态注册：

```text
/login
/register
/dashboard
/access/menus
```

`/access/menus` 只为菜单管理组件提供冷启动挂载点，meta 使用：

```text
i18nKey: navigation.accessMenus
requiredPermission: rbac:menu:list
```

动态安装遇到数据库节点
`rbac:menu:list + /access/menus + access/menus` 时，必须逐字段验证后复用静态 record，不执行
重复 `addRoute`。其他节点占用该 path、componentPath 或 route name 时返回 `ProtocolError`。

静态 route 不负责插入侧栏、权限、面包屑或 Tab。`AppAside` 删除手工菜单管理项，只递归
渲染 `/api/v1/access.menuTree`。Dashboard 保持产品明确要求的静态入口。

### 11.3 展示来源

| 位置 | 数据来源 |
| --- | --- |
| 菜单管理树表格 | `ManagedMenuNode.name` |
| 父节点选择器 | `name + code` |
| 角色授权矩阵/差异确认 | `name + code` |
| 侧边栏 | `t(i18nKey)` |
| 面包屑 | access tree 中的 `t(i18nKey)` |
| RouteTabs | access tree 中的 `t(i18nKey)` |

access DTO 只包含 directory/page，因此继续使用非空 `i18nKey: string`，不增加未使用的
`name`。菜单管理和角色授权 DTO 使用数据库 `name`；action 的 `i18nKey` 必须为 `null`。

缺少语言包 key 时明确显示 key，不使用 name/code/path 兜底。协议字段缺失或类型错误时抛
`ProtocolError`，不使用可选链、默认值或类型断言继续运行。

### 11.4 旧路径行为

以下旧路径不注册、不重定向：

```text
/system/users
/system/sessions
/system/menus
/system/roles
/system/auth-platforms
```

新路径未授权时，后端 API 返回 403，前端路由守卫按现有产品规则回到 Dashboard。旧路径
属于不存在的页面，而不是兼容入口。

## 12. 菜单管理界面

### 12.1 页面结构

页面继续沿用老项目和当前实现中成熟的紧凑树表格、Dialog、状态切换、确认框和错误反馈，
不重新发明重型管理界面。

```text
+----------------------------------------------------------------------------+
| [关键词：名称 / 权限码 / 路径] [清除]  [新增根目录] [展开/收起] [刷新]      |
+----------------------------------------------------------------------------+
| 名称 | 类型 | 权限码 | 路径 | 页面路径 | 图标 | 排序 | 可见 | 状态 | 操作 |
| 用户与账号                                                                 |
|   用户管理                                                                 |
| 权限与认证                                                                 |
|   菜单管理                                                                 |
| 系统管理                                                                   |
+----------------------------------------------------------------------------+
```

- 使用 Element Plus tree table、`border` 和现有主题变量；
- 不增加重复页面标题、卡片嵌套、营销文案或装饰背景；
- 名称列保留树缩进，其他列保持易扫描的稳定宽度；
- 操作列固定右侧，按钮不能被遮挡或挤出；
- 默认只展开三个根目录，不再 `default-expand-all`；
- 保留“全部展开/全部收起”和刷新命令；
- 展开状态由 row key 驱动，异步刷新后不能意外全部展开；
- loading、空态、读取失败、修改失败和重试入口明确可见。

### 12.2 搜索

搜索在前端对已经加载的完整树执行，不新增后端分页或搜索接口：

- 关键词去除首尾空格并按不区分英文大小写匹配；
- 匹配数据库 `name`、`code` 和非空 `path`；
- 结果包含所有匹配节点及其完整祖先链；
- directory 或 page 自身匹配时保留它的完整活动后代；只有后代匹配时仅保留该匹配节点、
  它的后代和祖先链，不带入不相关兄弟节点；
- 搜索状态自动展开匹配路径；
- 清空搜索后恢复搜索前的用户展开状态；
- 无结果显示明确空态，不把原树作为兜底继续显示。

### 12.3 Dialog

继续使用 `AppDialog`。桌面双列、移动端单列，输入控件占满可用宽度。

| 类型 | 字段 |
| --- | --- |
| directory | 父节点、类型、目录名称、i18nKey、code、图标、排序、启用、隐藏 |
| page | 父节点、类型、页面名称、i18nKey、code、路径、页面路径、图标、排序、启用、隐藏 |
| action | 父页面、类型、权限名称、code、排序、启用 |

- code 创建时可填，编辑时只读；
- action 不渲染 i18nKey、path、componentPath、icon 和隐藏控件；
- 类型切换立即清空不兼容字段；
- action 提交 `i18nKey = null`、`isHidden = 1`；
- 父节点和类型使用 `el-select-v2`；
- Yes/No 使用 Switch/Radio；
- 基础节点锁定结构字段并显示明确原因；
- Dialog 不设置固定高度，超出视口时使用公共滚动区。

帮助文本使用新协议示例：

```text
i18nKey：至少两段点号路径，例如 navigation.accountUsers
权限码：业务域:资源:动作，例如 account:user:list
路由：必须以 / 开头，例如 /account/users
页面路径：不能以 / 开头，例如 account/users
```

## 13. 本地图标协议

数据库可配置菜单图标统一使用 Lucide：

```text
lucide:<kebab-case-icon-name>
```

实现要求：

1. 使用本地 Vue Lucide 图标包，不从 Iconify API 或其他网络服务加载图标；
2. `DIcon` 对数据库字符串只解析 `lucide:*`，不再混合解析 Element Plus 名称和 `mdi:*`；
3. `IconSelect` 使用一个可检索的本地图标目录，展示图形、语义名称和稳定 code；
4. 支持关键词搜索、当前值预览、选择、清除和无结果空态；
5. 目录和页面可选图标，action 始终为 `NULL`；
6. 无效或不存在的 Lucide 名称必须显示明确协议错误，不尝试联网或显示静默替代图标；
7. 迁移对项目当前已知默认图标使用明确映射；遇到未知历史值时列出值并终止迁移；
8. 本次至少包含用户、账号、会话、盾牌、菜单、角色、钥匙、设置、日志、存储、上传、
   钱包和 AI 等常用语义图标，方便后续独立模块直接选用，但不因此创建未来业务节点。

菜单和导航范围内不再出现 `Setting`、`Menu`、`mdi:shield`、`lucide:database` 混合存储。
静态命令按钮优先使用 Lucide 组件；不要求本次无关页面为了换图标发生额外业务重构。

## 14. 错误处理

- 数据库处于新旧表并存、半迁移或对象名称冲突状态：启动失败；
- 表重命名、菜单 rekey 或版本推进失败：PostgreSQL 事务回滚；
- 基础节点缺失、重复或结构冲突：API 启动失败并记录具体 code/cause；
- 未知历史菜单 code、componentPath 或 icon：迁移列出值后失败；
- 非法 name、i18nKey、权限码、路径或图标：返回明确业务错误；
- 普通用户无 `rbac:menu:list`：菜单 API 返回 403；
- access 快照缺少菜单管理节点：不插入假节点、不授予假权限；
- 页面文件缺失或静态绑定不一致：动态路由安装进入明确错误状态；
- Redis 缓存错误：按已确认缓存设计记录错误并从 PostgreSQL 重建，不返回旧快照或假成功；
- 在线菜单写入后 Redis generation 发布失败：请求明确失败；
- 前端协议字段缺失或类型错误：抛 `ProtocolError`，不猜默认值；
- Lucide 图标解析失败：明确报告错误，不访问网络兜底。

## 15. 测试与验收

### 15.1 PostgreSQL 与迁移

真实 PostgreSQL 集成测试必须覆盖：

1. 九张旧表完整迁移为九张新表；
2. 主键、外键、CHECK、索引和序列名称同步更新；
3. 相关 PostgreSQL catalog 不残留 `sys_` 对象；
4. old/new 表并存、部分迁移和目标名称冲突均失败；
5. 迁移事务失败时不留下半完成结构；
6. 迁移重复执行只验证、不重复改写数据；
7. 所有 menu row ID 和 role-menu menu_id 保持；
8. 三个根目录、六个页面及 action code、父级、路径和 componentPath 精确；
9. 未知 code 和 icon 明确失败；
10. 所有 access version 推进，旧权限快照不可达；
11. 每张表和 Model 的 TIMESTAMPTZ、deleted_at 与 Yes/No 规则符合项目规范。

### 15.2 后端

1. 每个业务模块拥有并使用自己的权限常量；
2. `menu` 不再导出其他模块权限；
3. 所有 repository 原始 SQL 使用新表名；
4. menu 三种节点 DTO 和 PostgreSQL shape 约束一致；
5. 基础节点保护、首次创建、重复验证和冲突失败正确；
6. 普通角色权限只来自 `rbac_role_menu`；
7. 超级管理员权限只来自有效 `rbac_menu` page/action；
8. 菜单修改继续推进 access version 和 Redis generation；
9. access-state/snapshot schema 升级后拒绝旧缓存；
10. Worker 只注册 `audit:operation-log:v2`，审计 DTO 严格校验保持不变。

### 15.3 前端

1. 页面文件、import、componentPath 和测试目录使用 account/access/system 新路径；
2. 多根菜单树在侧栏、角色授权、面包屑和 RouteTabs 中正确处理；
3. `AppAside` 不再手工插入菜单管理；
4. `/access/menus` 精确复用静态 route record，不一致时失败；
5. 旧 `/system/*` 页面路径没有 route 或 redirect；
6. 权限按钮只判断新权限码；
7. 菜单管理和角色授权显示数据库 name，导航显示 i18n 翻译；
8. 搜索匹配 name/code/path、保留祖先、正确展开并可恢复状态；
9. 默认只展开根目录，全部展开/收起在异步数据后生效；
10. IconSelect 搜索、预览、选择、清除和空态正确；
11. 菜单图标在禁止网络请求时仍正常渲染，未知图标明确失败；
12. 不引入显式 `any`、`any[]`、`as any`、`Record<string, any>` 或 `@ts-ignore`。

### 15.4 定向名称扫描

实施完成后至少执行：

```powershell
rg -n "sys_(user|user_session|menu|role|user_role|role_menu|access_version|auth_platform|operation_log)" server web
rg -n "system:(user|session|menu|role|auth-platform|operation-log):" server web
rg -n '"/system/(users|sessions|menus|roles|auth-platforms)' web
rg -n "system/(users|sessions|menus|roles|auth-platforms)" web
rg -n "system:operation-log:v2|mdi:|iconify" server web
```

预期仅允许明确标注的一次性历史迁移映射或迁移测试出现旧值。运行时代码、当前 fixture、
前端业务代码和新 schema 断言不得出现旧协议。

### 15.5 完整验证

```powershell
cd D:\admin\server
go fmt ./...
go vet ./...
go test ./...
go build ./...

cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
```

浏览器至少使用超级管理员和一个普通角色验收：

- 三个根目录顺序、名称、图标和页面归属正确；
- 超级管理员看到全部有效数据库菜单；
- 普通角色未授权时不可见且 API 返回 403；
- list/create/update/delete 等 action 独立控制；
- 新浏览器路径、面包屑和 RouteTabs 一致；
- 旧路径不跳转到新页面；
- 搜索、展开、Dialog、图标、移动端和主题行为正常；
- 修改菜单或角色后，下次请求使用新 access version。

## 16. 完成标准

- 九张表及相关 PostgreSQL 对象全部使用真实业务域名称；
- Model、Repository SQL、权限常量、路由、页面目录、componentPath、i18n 和测试名称一致；
- 运行时代码不存在旧表别名、旧权限码、旧页面路径或旧 Asynq type；
- `rbac_menu` 是菜单和权限节点唯一事实来源，`rbac_role_menu` 是角色授权唯一事实来源；
- 菜单 ID 和已有角色授权关系保持；
- 超级管理员没有硬编码权限；
- 菜单管理位于“权限与认证”，用户与会话位于“用户与账号”，操作日志位于“系统管理”；
- 菜单管理支持紧凑树表格、搜索、祖先保留、稳定展开和完整 Dialog；
- 数据库菜单图标完全本地、统一为 Lucide 且不存在网络兜底；
- 后端与前端完整验证全部通过；
- 未自动创建、重写或推送 Git 提交。
