# 前端 API DTO 直接消费实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让前端 API 直接接收 Go 后端 DTO，统一由 `request.ts` 校验 envelope 并返回 `data`，删除业务级重复 parser。

**Architecture:** `request.ts` 是唯一的 HTTP envelope 边界，成功响应返回 `data`，失败响应抛出统一错误。每个 `api/<module>.ts` 定义明确 TypeScript DTO 并直接返回 `request<T>()`；页面只消费 DTO，不重新编排业务数据。

**Tech Stack:** Vue 3、TypeScript strict、Axios、Vitest、Go HTTP API。

---

### Task 1: 固化 request 解包契约

**Files:**
- Modify: `web/src/utils/request.ts`
- Test: `web/tests/utils/request.test.ts`

- [ ] **Step 1: 补充成功解包测试**

在 `web/tests/utils/request.test.ts` 的 request client 测试中增加成功响应断言：模拟
`{ code: 0, data: { value: 'ok' }, message: '' }`，调用 `request<{ value: string }>()`
后结果必须是 `{ value: 'ok' }`，调用方不能再读取 `.data`。

- [ ] **Step 2: 补充 envelope 失败测试**

覆盖缺少 `code`、`data`、`message`、出现额外字段、`code` 类型错误和非零 `code`；预期分别
抛出 `ProtocolError` 或 `ApiError`。

- [ ] **Step 3: 调整实现和泛型说明**

确认 Axios 成功拦截器保持：

```ts
response.data = unwrapSuccessEnvelope<unknown>(response.data)
```

`request<T>()` 返回 `Promise<T>`，不新增二次 `.data` 访问。保留 401 刷新、单次重试和登出行为。

- [ ] **Step 4: 运行定向测试**

运行：`cd D:\admin\web; pnpm vitest run tests/utils/request.test.ts --pool=threads --maxWorkers=1`

预期：request 测试全部通过。

### Task 2: 迁移认证类型和认证 API

**Files:**
- Modify: `web/src/api/auth.ts`
- Modify: `web/src/utils/request.ts`
- Modify: `web/src/store/auth.ts`
- Modify: `web/src/permission.ts`
- Modify: `web/src/views/auth/login/index.vue`
- Modify: `web/src/views/auth/register/index.vue`
- Modify: `web/tests/api/auth.test.ts`
- Modify: `web/tests/router/index.test.ts`
- Delete: `web/src/api/auth.contract.ts`
- Delete: `web/tests/api/auth.contract.test.ts`

- [ ] **Step 1: 将认证输入和响应类型移入 `auth.ts`**

保留 `AccessCredential`、`AuthPolicy`、`CurrentUser`、`LoginInput`、`RegisterInput`、
`RegisteredUser` 等纯类型，删除 `parseCredential`、`parseAuthPolicy`、`parseCurrentUser`、
`parseRegisteredUser`。

- [ ] **Step 2: 直接返回认证 DTO**

将 `register`、`login`、`getCurrentUser`、`getAuthPolicy` 改为明确的
`request<具体类型>()` 并直接返回结果；`logout` 使用后端约定的空对象响应类型，不再自行
检查对象并抛 parser 错误。

- [ ] **Step 3: 处理刷新请求**

`performRefresh` 保留 envelope 校验，但解包后直接按 `AccessCredential` 类型消费，不再调用
`parseCredential`。刷新 Token 的现有错误和重试流程不得改变。

- [ ] **Step 4: 更新类型 import 和测试**

把 Store、权限初始化、登录和注册页面及测试中的 `auth.contract` 类型导入改为 `api/auth`。
删除只覆盖 parser 的测试，保留登录、刷新、当前用户和错误行为测试。

- [ ] **Step 5: 运行认证定向测试**

运行：`cd D:\admin\web; pnpm vitest run tests/api/auth.test.ts tests/router/index.test.ts --pool=threads --maxWorkers=1`

### Task 3: 迁移用户 API 作为完整 CRUD 样板

**Files:**
- Modify: `web/src/api/user.ts`
- Modify: `web/src/views/account/users/index.vue`
- Modify: `web/tests/api/user.test.ts`
- Modify: `web/tests/views/account/users/index.test.ts`
- Delete: `web/src/api/user.contract.ts`
- Delete: `web/tests/api/user.contract.test.ts`

- [ ] **Step 1: 将用户类型移入 `user.ts`**

保留 `UserListQuery`、`UserListItem`、`UserPage`、`UserRoleSummary`、`UserRolesResponse`、
`UpdateUserInput`、`UpdateUserRolesInput` 和各写入结果 DTO，全部作为纯 TypeScript 类型。

- [ ] **Step 2: 直接返回后端 DTO**

把 `getUsers`、`getUserRoleOptions`、`updateUser`、`updateUserStatus`、`deleteUser`、
`getUserRoles`、`updateUserRoles` 改成 `request<具体 DTO>()` 直接返回，删除所有 `parse*` 调用。

- [ ] **Step 3: 更新页面类型 import**

用户页面从 `api/user.ts` 导入类型，保持现有 loading、错误、分页、角色和操作逻辑；不得在页面
增加排序、默认值或字段修补。

- [ ] **Step 4: 更新 API 和页面测试**

API 测试只验证 URL、请求方法、请求体和返回的后端 DTO；删除 parser 的字段重建测试，保留
CRUD 调用和错误透传测试。

- [ ] **Step 5: 运行用户定向测试**

运行：`cd D:\admin\web; pnpm vitest run tests/api/user.test.ts tests/views/account/users/index.test.ts --pool=threads --maxWorkers=1`

### Task 4: 迁移角色、会话、认证平台和操作日志 API

**Files:**
- Modify: `web/src/api/role.ts`
- Modify: `web/src/api/session.ts`
- Modify: `web/src/api/auth-platform.ts`
- Modify: `web/src/api/operation-log.ts`
- Modify: `../../../web/src/views/permission/roles/index.vue`
- Modify: `../../../web/src/views/permission/roles/role-permission-matrix.ts`
- Modify: `web/src/views/account/sessions/index.vue`
- Modify: `../../../web/src/views/permission/auth-platforms/index.vue`
- Modify: `web/src/views/system/operation-logs/index.vue`
- Modify: matching files under `web/tests/api` and `web/tests/views`
- Delete: `web/src/api/role.contract.ts`, `web/src/api/session.contract.ts`, `web/src/api/auth-platform.contract.ts`, `web/src/api/operation-log.contract.ts`
- Delete: parser-only tests for those four contract files

- [ ] **Step 1: 迁移纯类型**

将四个 contract 文件中的请求输入、列表项、分页、详情和写入结果类型移动到对应 API 文件；
保留页面和矩阵实际使用的类型名，避免改变后端字段。

- [ ] **Step 2: 删除 parser 调用**

所有列表、统计、详情、状态、默认角色、权限矩阵、会话撤销和日志查询函数直接使用
`request<具体 DTO>()` 返回值。

- [ ] **Step 3: 更新消费者和测试 import**

更新页面、矩阵组件、Store 和测试中的类型来源；测试改为验证 API 返回值透传，不再测试前端
排序、去重或字段闭合逻辑。

- [ ] **Step 4: 运行定向测试**

运行：`cd D:\admin\web; pnpm vitest run tests/api/role.test.ts tests/api/session.test.ts tests/api/auth-platform.test.ts tests/views/access/roles/index.test.ts tests/views/account/sessions/index.test.ts tests/views/access/auth-platforms/index.test.ts tests/views/system/operation-logs/index.test.ts --pool=threads --maxWorkers=1`

### Task 5: 迁移菜单和访问快照 API

**Files:**
- Modify: `web/src/api/menu.ts`
- Modify: `web/src/api/access.ts`
- Modify: `web/src/router/access-routes.ts`
- Modify: `web/src/layout/components/AccessMenuNode.vue`
- Modify: `web/src/layout/components/RouteTabs.vue`
- Modify: `web/src/layout/breadcrumbs.ts`
- Modify: `../../../web/src/store/permission.ts`
- Modify: `../../../web/src/views/permission/menus/index.vue`
- Modify: `../../../web/src/views/permission/menus/filter-menu-tree.ts`
- Modify: matching menu/access tests
- Delete: `web/src/api/menu.contract.ts`, `web/src/api/access.contract.ts`
- Delete: parser-only menu/access contract tests

- [ ] **Step 1: 迁移菜单和访问快照类型**

将 `ManagedMenuNode`、菜单输入和结果、`AccessMenuNode`、`AccessSnapshot`、角色权限树节点等
纯类型移动到对应 API 文件或明确的 `api/<module>.types.ts`；不能保留业务 parser。

- [ ] **Step 2: 直接消费后端树 DTO**

`getMenus` 和 `getAccess` 直接返回后端树结构。页面保留搜索时对展示树的筛选，这是 UI 状态
转换，不得修改 API 返回的业务事实或补缺失字段。

- [ ] **Step 3: 更新路由、布局和菜单页面类型 import**

确保动态路由、侧栏、面包屑、RouteTabs、角色权限矩阵和菜单管理页使用新的类型来源，行为
保持现有菜单协议设计。

- [ ] **Step 4: 运行定向测试**

运行：`cd D:\admin\web; pnpm vitest run tests/api/access.test.ts tests/api/menu.test.ts tests/router/access-routes.test.ts tests/views/access/menus/index.test.ts tests/views/access/menus/filter-menu-tree.test.ts tests/layout/components/AccessMenuNode.test.ts --pool=threads --maxWorkers=1`

### Task 6: 收敛简单 API 并清理残留

**Files:**
- Modify: `web/src/api/health.ts`
- Modify: `web/src/api/taskDemo.ts`
- Modify: all remaining `web/src` and `web/tests` imports found by scans
- Delete: any contract file with no remaining type-only purpose

- [ ] **Step 1: 为简单接口补明确返回类型**

健康检查、任务提交等接口直接使用明确的 `request<T>()` 类型；不新增 parser 或兼容字段。

- [ ] **Step 2: 执行残留扫描**

运行：

```powershell
rg -n "request<unknown>" web/src/api web/src/views
rg -n "parse[A-Z].*\(" web/src/api web/src/views
rg -n "\.contract" web/src/api web/src/views web/src/router
rg -n "\bmsg\b|response\.data\.data|\.data\.data" web/src
```

预期：运行时代码不再有业务 `request<unknown>`、业务 `parse*` 或 `msg` 兼容；类型文件只在
确有多个消费者时保留，并且不导出运行时 parser。

- [ ] **Step 3: 运行 TypeScript 检查**

运行：`cd D:\admin\web; pnpm build`

预期：Vue、TypeScript 和 Vite 构建通过。

### Task 7: 全量验证与交付检查

**Files:**
- Test: `web/tests/**/*.test.ts`
- Verify: `server/**`（无行为修改，但执行跨端回归）

- [ ] **Step 1: 运行前端全量测试**

运行：`cd D:\admin\web; pnpm vitest run --pool=threads --maxWorkers=1`

预期：全部前端测试通过；若已有完整测试超时，记录具体未完成项，不宣称全量通过。

- [ ] **Step 2: 运行后端回归**

运行：`cd D:\admin\server; go test ./...; go build ./...`

预期：后端测试和构建通过，确认本次前端协议收敛没有影响跨端契约。

- [ ] **Step 3: 检查工作区和 diff**

运行：`cd D:\admin; git status --short; git diff --check`

确认只包含本计划涉及的前端代码、测试和文档，不删除其他开发者改动，不自动提交、推送或
重写既有历史。
