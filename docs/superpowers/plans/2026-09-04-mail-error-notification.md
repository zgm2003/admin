# Mail Error Notification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让邮件收件规则拒绝和 RBAC 权限拒绝都能得到明确的一次性前端通知，同时保持 401 自动刷新流程不产生中间态噪声。

**Architecture:** 后端在邮件模块内使用独立的领域错误码和 i18n 消息键，HTTP 语义仍为 403；前端请求层把所有终态 403 和终态 401 交给统一通知逻辑，仅在 token 自动刷新进行中的中间 401 路径保持静默。调用方继续收到标准 `ApiError`，不由请求层吞错或改写成功响应。

**Tech Stack:** Go、Gin、`apperror.Error`、共享 i18n catalog、Axios、TypeScript、Element Plus `ElNotification`、Go test、Vitest。

**Spec:** `docs/superpowers/specs/2026-09-04-mail-error-notification-design.md`

## Global Constraints

- HTTP/API envelope 只允许 `code`、`data`、`message`；业务错误必须通过 `apperror.Error` 输出。
- `10003` 只表示 RBAC 权限拒绝；邮件业务拒绝使用 `18000`，不能复用通用权限码。
- HTTP 403 是终态错误，必须触发一次 `ElNotification`；只有自动刷新中的中间 401 不通知。
- 请求层负责通用通知，页面不得对同一个 `ApiError` 重复弹通知。
- 继续传递原始 `context.Context`，不修改邮件发送、限流、Redis、数据库和路由权限边界。
- 只修改本计划列出的文件和对应测试，不运行会产生无关行尾改动的全量格式化。

---

### Task 1: 建立邮件拒绝错误契约

**Files:**
- Modify: `server/internal/module/message/mail/errors.go`
- Modify: `server/internal/module/message/mail/service_test.go`
- Modify: `server/internal/shared/i18n/catalog.go`
- Modify: `server/internal/shared/i18n/catalog_test.go`

**Interfaces:**
- Consumes: `mail.send` 中 `denied(ErrRecipientDenied)` 调用、`apperror.Error` 字段和共享 i18n catalog。
- Produces: `mail.CodeRecipientDenied = 18000`、`i18n.KeyMailRecipientDenied`，以及 HTTP 403 的邮件领域 `*apperror.Error`。

- [ ] **Step 1: 确认失败测试覆盖错误字段**

保留或补充 `TestSendReturnsRecipientDeniedAsMailBusinessError`，使用 `ruleEvaluatorStub{decision: RuleDecision{Allowed: false}}` 调用 `Service.Send`，断言：

```go
var appErr *apperror.Error
if !errors.As(err, &appErr) {

	t.Fatalf("error = %v, want application error", err)
}
if appErr.HTTPStatus != http.StatusForbidden ||
	appErr.Code != CodeRecipientDenied ||
	appErr.MessageKey != i18n.KeyMailRecipientDenied {

	t.Fatalf("application error = status %d, code %d, key %q", appErr.HTTPStatus, appErr.Code, appErr.MessageKey)
}
```

- [ ] **Step 2: 运行后端失败测试**

运行：`cd D:\admin\server; go test ./internal/module/message/mail -run TestSendReturnsRecipientDeniedAsMailBusinessError -count=1`

预期：在实现错误码、消息键前失败，失败原因应是符号尚未定义或断言不满足，而不是数据库或网络错误。

- [ ] **Step 3: 实现邮件领域错误**

在 `errors.go` 按现有用户、角色模块的错误模式增加 `CodeRecipientDenied = 18000`，并让 `denied` 返回包含以下字段的 `*apperror.Error`：

```go
HTTPStatus: http.StatusForbidden
Code:       CodeRecipientDenied
MessageKey: i18n.KeyMailRecipientDenied
Cause:      err
```

不得调用 `apperror.Forbidden`，不得修改 `ErrRecipientDenied` 的触发位置。

- [ ] **Step 4: 补齐中英文 i18n**

在 `catalog.go` 的 key 常量、`ZhCN` catalog 和 `EnUS` catalog 中同时加入：

```text
mail.recipientDenied
```

两种 locale 都必须定义同一个 key，且不引入插值参数。沿用 `ValidateCatalogs` 确保目录键集合一致。

在 `catalog_test.go` 增加无插值参数的双 locale 翻译测试：

```go
func TestCatalogsTranslateMailErrors(t *testing.T) {
	for _, locale := range []i18n.Locale{i18n.ZhCN, i18n.EnUS} {
		message, err := i18n.Translate(locale, i18n.KeyMailRecipientDenied, nil)
		if err != nil || message == "" {
			t.Errorf("Translate(%q, %q) = %q,%v", locale, i18n.KeyMailRecipientDenied, message, err)
		}
	}
}
```

- [ ] **Step 5: 运行后端契约测试**

运行：

```powershell
cd D:\admin\server
go test ./internal/module/message/mail -run TestSendReturnsRecipientDeniedAsMailBusinessError -count=1
go test ./internal/shared/i18n -run 'TestCatalogsHaveTheSameKeysAndParameters|TestCatalogsTranslateMailErrors' -count=1
```

预期：邮件拒绝错误字段、中文/英文 catalog 校验全部通过；允许发送、限流和其他邮件错误测试未被改动。

### Task 2: 调整前端终态错误通知

**Files:**
- Modify: `web/src/utils/request.ts`
- Modify: `web/tests/utils/request.test.ts`

**Interfaces:**
- Consumes: `ApiError` 的 `httpStatus`、Axios 401 刷新分支、`notifyRequestError`。
- Produces: 所有终态 403 和终态 401 的一次性通知；刷新成功的中间 401 仍无通知。

- [ ] **Step 1: 拆分现有 401/403 通知测试**

删除或改写把 401 和 403 一起断言为“不通知”的测试。新增/保留以下行为断言：

```ts
it('notifies RBAC forbidden failures', async () => {
  const client = createRequestClient('http://localhost:16301')

  await expect(
    client.get('/api/admin/v1/protected', {
      adapter: failureAdapter(403, {
        code: 10003,
        data: null,
        message: '没有访问权限',
      }),
    }),
  ).rejects.toMatchObject({ code: 10003, httpStatus: 403 })
  expect(ElNotification.error).toHaveBeenCalledOnce()
})

it('notifies mail business forbidden failures', async () => {
  const client = createRequestClient('http://localhost:16301')

  await expect(
    client.post('/api/admin/v1/mail/test', undefined, {
      adapter: failureAdapter(403, {
        code: 18000,
        data: null,
        message: '收件邮箱被收件规则拒绝',
      }),
    }),
  ).rejects.toMatchObject({ code: 18000, httpStatus: 403 })
  expect(ElNotification.error).toHaveBeenCalledOnce()
})
```

测试实现必须使用现有 `failureAdapter`，同时断言 `code`、`httpStatus` 和通知次数。

- [ ] **Step 2: 先运行前端失败测试**

运行：`cd D:\admin\web; pnpm vitest run tests/utils/request.test.ts --pool=threads --maxWorkers=1`

预期：403 测试在现有 `notifyRequestError` 静默条件下失败，现有刷新成功的中间 401 测试仍能通过。

- [ ] **Step 3: 移除 401/403 全局静默**

调整 `notifyRequestError`：不再按 HTTP 401 或 403 直接返回；403/10003、403/18000 和终态 401 都按普通终态错误显示通知并继续 reject。不得根据错误文案、URL 或模块名称猜测错误类型。

- [ ] **Step 4: 保持并验证 401 状态机**

不要在首次受保护请求收到 401、且尚未调用 `notifyRequestError` 的刷新分支中新增通知。确保以下已有路径各只发生一次终态通知：

- 登录/刷新接口直接返回 401；
- 原请求刷新失败并由 `performRefresh` 处理；
- 重试后的原请求仍返回 401。

如果 `performRefresh` 已经通知终态错误，外层 catch 不得再次通知。认证清理和 `onUnauthorized` 回调保持原有调用顺序；本次不新增重试后 401 的全局跳转回调。

并在并发终态 401 测试中补充：

```ts
expect(ElNotification.error).toHaveBeenCalledOnce()
expect(onUnauthorized).toHaveBeenCalledOnce()
```

- [ ] **Step 5: 运行前端定向测试**

运行：`cd D:\admin\web; pnpm vitest run tests/utils/request.test.ts --pool=threads --maxWorkers=1`

预期：403 权限拒绝和邮件拒绝各通知一次；刷新成功的中间 401 不通知；终态 401 不重复通知；协议错误、网络错误、业务错误原有断言继续通过。

### Task 3: 跨层回归与审查

**Files:**
- Test: `server/internal/module/message/mail/service_test.go`
- Test: `server/internal/shared/i18n/catalog_test.go`
- Test: `web/tests/utils/request.test.ts`

**Interfaces:**
- Consumes: Task 1 的邮件错误契约和 Task 2 的请求通知策略。
- Produces: 可验证的 API 错误语义、通知次数和认证刷新行为。

- [ ] **Step 1: 检查后端权限错误未被改写**

确认 `server/internal/module/permission/access/middleware.go` 仍返回 HTTP 403、`apperror.CodeForbidden`（10003）和 `i18n.KeyPermissionDenied`；本次改动不得修改该中间件。

- [ ] **Step 2: 检查邮件拒绝链路**

确认 `Service.send` 在规则拒绝时只调用 `denied(ErrRecipientDenied)`，handler 仍使用统一 `response.Fail` 输出 `code/data/message` envelope，未新增第二次通知或旁路返回。

- [ ] **Step 3: 运行邮件和共享错误回归**

运行：

```powershell
cd D:\admin\server
go test ./internal/module/message/mail -count=1
go test ./internal/shared/apperror ./internal/shared/i18n -count=1
```

预期：邮件模块和共享错误/i18n 测试通过；失败时只修复本计划范围内的契约或测试，不顺手修改限流、数据库或页面结构。

- [ ] **Step 4: 运行前端邮件与请求回归**

运行：

```powershell
cd D:\admin\web
pnpm vitest run tests/utils/request.test.ts tests/views/message/mail/index.test.ts --pool=threads --maxWorkers=1
```

预期：请求通知、邮件收件规则状态切换和页面现有测试全部通过，状态切换不再出现“成功但无反馈”的情况。

- [ ] **Step 5: 完成代码审查记录**

审查 diff 只包含错误码/i18n、请求通知策略和对应测试；确认没有修改 API envelope、权限码、Redis key、数据库迁移或邮件限流规则。执行 AI 在提交前按项目 Git 规范检查 `git status --short`、暂存 diff 和测试结果。
