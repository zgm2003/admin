# 前端 API DTO 直接消费设计

## 1. 状态

状态：用户已批准设计，等待实施计划。

## 2. 背景

当前项目的 Go 后端已经负责业务 DTO、字段编排、分页结构、关联关系和错误协议。前端
`request.ts` 也已经在 Axios 响应拦截器中完成了 envelope 解包，但各业务 API 仍普遍使用
`request<unknown>()`，随后通过 `*.contract.ts` 的 `parse*` 函数再次检查、排序、补规则并
重建对象。

当前链路实际变成：

```text
Go DTO -> request.ts 解包 -> unknown -> 业务 contract parser -> 重建 DTO -> View
```

这复制了后端业务协议，增加了大量重复代码，也让前端 API 调用不再是清晰的线性数据流。

## 3. 目标

统一为：

```text
View -> api/<module>.ts -> request<T>() -> 校验 envelope -> return data -> Go API
```

具体目标：

1. `request.ts` 负责唯一的 HTTP envelope 校验、错误转换和 `data` 解包。
2. 业务 API 文件使用明确的 TypeScript DTO 类型，直接返回 `request<T>()` 的结果。
3. 页面直接消费后端已经编排好的 DTO，不重新排序、补默认值、重建对象或推断缺失字段。
4. 删除业务级 `parse*` 运行时协议层，避免前后端各维护一套业务事实。
5. 保留 TypeScript 编译期类型安全，禁止使用 `any`、宽泛断言或静默兼容。

## 4. 非目标

- 不改变 Go API 的 URL、HTTP 方法、请求字段、响应 DTO 或业务行为。
- 不改变公共响应 envelope：响应仍然只能包含 `code`、`data`、`message`。
- 不删除 `request.ts` 对 envelope 的运行时检查。
- 不在前端新增默认值、假数据、字段别名或旧协议兼容。
- 不把所有 DTO 生成系统、OpenAPI 工具链或通用 API SDK 引入本次改造。
- 不进行与 API DTO 消费无关的页面重构、视觉调整或后端重构。

## 5. 公共 request 契约

`web/src/utils/request.ts` 是唯一的 envelope 边界。

后端成功响应：

```json
{
  "code": 0,
  "data": {},
  "message": ""
}
```

`request<T>()` 必须：

1. 确认响应是对象而不是数组或 `null`；
2. 确认对象只包含 `code`、`data`、`message` 三个字段；
3. 确认 `code` 是整数、`message` 是字符串；
4. `code === 0` 时返回 `data`，类型为 `T`；
5. `code !== 0` 时抛出 `ApiError`，不把失败数据返回给业务 API；
6. HTTP 错误响应也必须解析同一 envelope；响应不符合 envelope 时抛出 `ProtocolError`；
7. 保留现有 401 刷新 Token、单次重试和最终登出行为。

因此业务 API 不需要再写：

```ts
const envelope = await request(...)
return envelope.data
```

也不需要写：

```ts
const raw = await request<unknown>(...)
return parseUserPage(raw)
```

## 6. 业务 API 文件规则

每个 `web/src/api/<module>.ts` 只包含：

- 请求参数类型；
- 后端响应 DTO 类型；
- 请求函数；
- 必要的输入字段整理，例如把函数参数放入 URL 或请求体。

成功请求直接返回明确类型：

```ts
export async function getUsers(query: UserListQuery): Promise<UserPage> {
  return request<UserPage>({
    method: 'GET',
    url: '/api/v1/users',
    params: query,
  })
}
```

禁止：

- `request<unknown>()` 后再调用业务 `parse*`；
- 对后端 DTO 重新排序、去重、补字段或改变嵌套结构；
- 用可选链、`??` 或类型断言掩盖后端必填字段缺失；
- 在页面中直接拼接 API envelope；
- 为了“兼容”读取 `msg`、旧字段名或旧响应结构。

## 7. contract 文件迁移

现有 `*.contract.ts` 按职责拆分处理：

1. 纯 TypeScript 类型、请求输入和响应 DTO 类型迁移到对应的 `api/<module>.ts`，或在确有
   多个消费者时移动到明确命名的类型文件；
2. 删除业务级 `parse*`、`closed`、`positive`、`timestamp`、排序和关系一致性校验函数；
3. 删除只为这些 parser 服务的测试；
4. 仍被页面、路由或测试直接作为类型导入的类型，迁移后更新 import，不保留空壳兼容文件；
5. `access`、菜单树、角色权限矩阵等页面所需的结构类型仍然保留，但只作为类型，不再对
   后端返回值做第二次业务编排；
6. 公共本地存储解析、用户输入校验和表单校验不属于 API DTO parser，本次不误删。

本次范围覆盖现有业务 API：

```text
access
auth
auth-platform
menu
operation-log
role
session
user
```

`health` 和 `taskDemo` 同样改为明确的 `request<T>()` 返回类型，但不需要为简单结果新增
独立 contract 文件。

## 8. 前后端职责边界

后端负责：

- DTO 字段和嵌套结构；
- 字段是否必填；
- 关联数据、排序、分页和状态的一致性；
- 业务错误和 HTTP 状态；
- `code/data/message` envelope。

前端负责：

- TypeScript 编译期类型声明；
- 请求参数的 UI 输入转换；
- loading、error、空态和交互状态；
- 将后端 DTO 映射为组件真正需要的展示状态，但不得伪造或修补后端业务事实。

如果后端返回缺字段或类型错误，前端不会静默修复。公共 envelope 违规由 `request.ts`
抛出 `ProtocolError`；业务 DTO 的具体契约问题由类型检查、后端测试和接口联调发现，不能
通过默认值隐藏。

## 9. 迁移顺序

按以下顺序逐模块迁移，保持每一步可构建：

1. 调整 `request.ts` 的泛型返回说明和定向测试，确认成功响应返回 `data`；
2. 迁移 `user.ts`，作为完整 CRUD 样板；
3. 迁移 `role.ts`、`session.ts`、`auth-platform.ts` 和 `operation-log.ts`；
4. 迁移 `menu.ts`、`access.ts` 和 `auth.ts`；
5. 更新页面、Store、路由和测试中的类型 import；
6. 删除无引用的业务 contract 文件和 parser 测试；
7. 执行旧 parser、`request<unknown>` 和旧 envelope 字段扫描。

迁移期间不保留两套 API 调用方式。每个模块完成后，API 文件必须直接返回明确 DTO。

## 10. 测试与验收

### 10.1 request 测试

必须覆盖：

- 成功 envelope 返回 `data`，调用方不需要再次访问 `.data`；
- 缺少 `code`、`data` 或 `message` 时抛出 `ProtocolError`；
- 存在额外 envelope 字段时抛出 `ProtocolError`；
- 非零 `code` 抛出 `ApiError`；
- HTTP 错误 envelope 能转换为 `ApiError`；
- 401 刷新并重试逻辑保持原行为。

### 10.2 API 与页面测试

- 每个 API 函数的返回值类型是后端 DTO，而不是 `unknown`；
- 用户、角色、菜单、会话、认证平台和操作日志页面直接使用返回字段；
- 不再调用业务 `parse*` 函数；
- 删除 parser 后，列表、详情、写入结果和空响应行为不变；
- 后端返回错误时页面仍显示现有明确错误状态。

### 10.3 静态扫描

实施完成后执行：

```powershell
rg -n "request<unknown>" web/src/api web/src/views
rg -n "parse[A-Z].*\(" web/src/api web/src/views
rg -n "\.contract" web/src/api web/src/views web/src/router
rg -n "\bmsg\b|response\.data\.data|\.data\.data" web/src
```

预期：业务 API 不再使用 `request<unknown>` 或业务 `parse*`；只保留确有用途的类型文件和
公共 envelope 测试。

### 10.4 完整验证

```powershell
cd D:\admin\web
pnpm vitest run --pool=threads --maxWorkers=1
pnpm build
```

后端无行为变化，但跨端契约完成后仍执行：

```powershell
cd D:\admin\server
go test ./...
go build ./...
```

## 11. 完成标准

- `request.ts` 是唯一公共 envelope 解包入口；
- 所有业务 API 直接返回明确 DTO；
- 前端不再复制后端业务 DTO 的运行时校验和编排；
- 页面直接消费后端字段，现有功能和错误行为不变；
- 无业务 `request<unknown>`、无业务 `parse*` 调用、无旧 `msg` 兼容；
- TypeScript 严格检查和前端构建通过；
- 不自动提交、推送或重写既有 Git 历史。
