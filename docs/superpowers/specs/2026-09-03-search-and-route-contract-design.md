# Search 类型安全与菜单路径契约统一设计

> 状态：用户已确认设计方向，待编写 implementation plan 后执行。
> 范围：前端 AppSearch、RBAC 菜单路径协议、数据库菜单迁移。

## 1. 背景与目标

当前前端页面已经迁移到新的目录结构，但部分 PostgreSQL 菜单数据仍保存旧的 `path/component_path`。
前端因此增加了 `componentPathMap`，把接口值转换为本地文件路径。项目尚未上线，不需要长期保留这层兼容逻辑，
应直接修正数据库协议。

同时，AppSearch 目前使用宽泛的 `Record<string, SearchFormValue>`。页面需要重复编写 `typeof`、数组形状判断和类型断言，
且组件在模板渲染阶段遇到错误值会直接抛异常。

本设计目标：

1. `path` 与 `componentPath` 使用同一套页面目录命名；
2. 前端直接消费接口返回的 `componentPath`，不再维护映射表；
3. AppSearch 的字段类型与页面搜索模型建立明确关系；
4. 错误值在组件入口或页面构造阶段被发现，不在模板渲染中途抛错；
5. 减少各业务页面的重复 narrowing 和类型断言。

## 2. 非目标

- 不修改权限码、API URL、业务接口语义或权限校验逻辑；
- 不修改菜单 `code`、`name`、`i18nKey`、图标、排序、隐藏状态或角色授权关系；
- 不处理未知 i18n key 策略；当前继续保留现有行为；
- 不改变脏菜单节点的 fail-closed 路由策略；
- 不移动测试目录中的旧 `src` 中间目录，本项另行处理；
- 不操作 Redis。数据库迁移只推进 PostgreSQL 中的 access version，运行时按既有流程重新读取。

## 3. 菜单路径统一契约

### 3.1 规则

每个 `menuType=page` 节点必须满足：

```text
componentPath = 页面源码相对目录
path = / + componentPath
源码文件 = web/src/views/<componentPath>/index.vue
```

`path` 仍然是浏览器访问地址，`componentPath` 仍然是源码模块路径；本次由于项目未上线，两者统一为同一目录名称，
避免维护两套路径协议。

动态路由只做以下操作：

1. 从 Access 快照读取 `node.path`、`node.componentPath`；
2. 使用 `componentPath` 拼接受限 glob 的模块 key；
3. 使用 `path` 注册浏览器路由；
4. 模块不存在、路径重复或字段不完整时明确抛出 `ProtocolError`。

禁止根据 `path` 猜测 `componentPath`、根据旧业务目录自动转换路径，或长期保留 `componentPathMap`、legacy alias 和静默 fallback。

### 3.2 统一后的页面值

| 页面权限码 | `path` | `componentPath` | 源码文件 |
| --- | --- | --- | --- |
| `account:user:view` | `/account/users` | `account/users` | `views/account/users/index.vue` |
| `account:profile:view` | `/account/profile` | `account/profile` | `views/account/profile/index.vue` |
| `auth:session:view` | `/account/sessions` | `account/sessions` | `views/account/sessions/index.vue` |
| `account:user:loginlog:view` | `/account/login-logs` | `account/login-logs` | `views/account/login-logs/index.vue` |
| `permission:menu:view` | `/permission/menus` | `permission/menus` | `views/permission/menus/index.vue` |
| `permission:role:view` | `/permission/roles` | `permission/roles` | `views/permission/roles/index.vue` |
| `auth:platform:view` | `/permission/auth-platforms` | `permission/auth-platforms` | `views/permission/auth-platforms/index.vue` |
| `system:operation-log:view` | `/system/operation-logs` | `system/operation-logs` | `views/system/operation-logs/index.vue` |
| `storage:object:view` | `/cloud/storage-object` | `cloud/storage-object` | `views/cloud/storage-object/index.vue` |
| `message:mail:view` | `/message/mail` | `message/mail` | `views/message/mail/index.vue` |

权限码和 API URL 不随页面路径调整而改变。

### 3.3 数据库迁移要求

新增幂等、可审计的 PostgreSQL migration，按页面权限码更新旧值：

| 权限码 | 旧 `path` | 旧 `component_path` | 新 `path` | 新 `component_path` |
| --- | --- | --- | --- | --- |
| `permission:menu:view` | `/access/menus` | `access/menus` | `/permission/menus` | `permission/menus` |
| `permission:role:view` | `/access/roles` | `access/roles` | `/permission/roles` | `permission/roles` |
| `auth:platform:view` | `/access/auth-platforms` | `access/auth-platforms` | `/permission/auth-platforms` | `permission/auth-platforms` |
| `account:user:loginlog:view` | `/account/login-logs` | `user/login-logs` | `/account/login-logs` | `account/login-logs` |
| `storage:object:view` | `/cloud/object-storage` | `storage/object` | `/cloud/storage-object` | `cloud/storage-object` |

迁移必须：

- 只更新有效菜单行（`deleted_at IS NULL`）；
- 先检查目标值冲突，冲突时整个事务失败；
- 保留菜单 ID、父子关系和 `permission_role_menu` 授权关系；
- 仅发生实际更新时推进 `permission_access_version`；
- 重复执行不重复改写、不重复推进版本；
- 更新初始化 SQL 和后端旧数据迁移目标，避免新装数据库再次生成旧值；
- 不新增旧路径 redirect，不保留兼容别名。

## 4. AppSearch 类型契约

### 4.1 当前字段类型

暂时只支持三种字段：

```ts
export type SearchFieldType = 'input' | 'select-v2' | 'date-range'
```

新增字段类型时，必须同时补充字段值类型映射、Props/Emits 类型、运行时值校验、渲染分支、组件测试和至少一个页面使用样例。

### 4.2 页面模型

页面定义自己的搜索模型，不再直接使用无键约束的宽泛 Record。例如：

```ts
interface OperationLogSearchModel {
  userID: string
  action: string
  route: string
  success: '' | YesNo
  timeRange: SearchDateRange
}
```

AppSearch 应使用泛型或等价的键值映射，使以下关系在编译期成立：

- `input` 和 `select-v2` 只能绑定字符串、数字或明确的空值；
- `date-range` 只能绑定 `[] | [string, string]`；
- `field.key` 必须是模型中的真实 key；
- `update:modelValue`、`query`、`reset` 返回同一模型类型；
- 页面不再为每个字段手写 `typeof` narrowing 或 `as string`。

实现可以使用 Vue 泛型 SFC，也可以使用等价的显式类型辅助；不得引入 `any`、Schema DSL 或万能表单抽象。

### 4.3 运行时行为

- Props/model 初次进入和变化时，在组件入口校验字段 key 与值形状；
- 用户控件产生的未知值不得静默转换或写入模型；
- 非法值进入明确的组件错误状态，并阻止 query/reset 发出错误模型；
- 模板中的 `inputValue`、`dateRangeValue`、`selectValue` 不再直接 `throw`；
- 正常合法数据的渲染和事件语义保持不变；
- 错误状态必须可测试、可观察，不能只吞掉异常。

### 4.4 页面迁移范围

迁移所有当前使用 AppSearch 的页面和组件：

- `account/users`
- `account/sessions`
- `account/login-logs`
- `permission/auth-platforms`
- `permission/roles`
- `system/operation-logs`
- `cloud/storage-object/components/ConfigTable`
- `cloud/storage-object/components/RuleTable`

页面保留现有查询字段、分页、权限、默认值和 UI 文案，只替换搜索模型定义及重复 narrowing。

## 5. 测试与验收

### 5.1 AppSearch

- 三种字段类型均能渲染并提交正确模型；
- 错误字段 key 或错误值类型不会在模板渲染阶段白屏；
- 非法值不会静默写入或触发 query；
- `SearchFieldType`、字段 key、模型值和 emits 类型通过 `vue-tsc`；
- 现有页面查询参数和 reset 行为回归通过；
- 页面源码不再出现针对 `SearchFormModel` 的重复字段 narrowing 断言。

### 5.2 路由与数据库

- 数据库迁移首次执行完成所有旧值到新值的转换；
- 重复执行无额外数据变化、access version 不重复推进；
- 目标值冲突时事务回滚；
- 菜单 ID 和角色授权关系保持不变；
- 动态路由测试只使用统一后的 `path/componentPath`；
- 删除 `componentPathMap` 后，所有页面仍能按接口值注册；
- 旧 `/access/*`、`/cloud/object-storage` 运行时路径不再注册或 redirect；
- 未知 `componentPath` 仍明确失败。

### 5.3 验证命令

```powershell
cd D:\admin\server
go test ./internal/module/permission/menu ./internal/database -run 'Menu|RBAC|Migration' -count=1

cd D:\admin\web
pnpm vitest run tests/components/AppSearch tests/router tests/layout tests/views/permission tests/views/account tests/views/cloud/storage-object --pool=threads --maxWorkers=1
pnpm format:check
pnpm lint
pnpm check:architecture
pnpm typecheck
pnpm build
```

完整前端 Vitest 仍使用：

```powershell
pnpm vitest run --pool=threads --maxWorkers=1
```

## 6. 交付边界

本 spec 完成后，下一步单独编写 implementation plan，按以下顺序执行：

1. 先补失败测试和迁移测试；
2. 更新数据库 SQL、后端迁移目标和测试夹具；
3. 删除前端 componentPath 映射并统一路由路径；
4. 实现 AppSearch 泛型模型和入口校验；
5. 迁移 8 个页面/组件的搜索模型；
6. 运行定向验证和完整门禁。
