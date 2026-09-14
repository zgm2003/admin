# 邮件与短信模板正文能力实施计划

> 执行要求：使用 superpowers:executing-plans 按任务顺序实施。每个行为变化先写失败测试，再写最小实现。未经维护者明确授权，不执行数据库迁移、不提交 Git。

目标：在系统中维护可直接上传腾讯云的短信正文与完整邮件 HTML，支持动态变量、示例值、预览和复制，同时保持验证码发送链路不变。

已确认决策：
- 短信正文使用腾讯云位置占位符 {1}、{2}；variableKeys 数组顺序决定占位符映射。
- 邮件正文存储完整 HTML 文档；变量使用 {{code}}、{{ttl_minutes}}。
- 邮件编辑提供富文本、HTML 源码、邮件预览三种视图；源码是最终事实，富文本只编辑 body 内容。
- 预览使用 sandbox iframe，不使用 v-html。
- 当前四个验证码场景要求 code、ttl_minutes；数据结构允许以后追加 invitation_link。
- 模板禁用时腾讯模板 ID 可以为空；启用时必须是有效 ID。
- 富文本编辑器只在打开邮件模板编辑弹窗时异步加载，不制造入口大 chunk。
- 登录邮件使用用户提供的完整 HTML 样例作为默认模板，其余场景复用同一布局并替换文案。

全局约束：
- 不使用 Docker。
- 不自动 commit、fetch、pull、push、rebase 或回滚其他开发者改动。
- 执行前检查 git status --short，只修改本计划列出的文件。
- PostgreSQL 是事实来源；迁移前只读检查真实结构、约束及现有变量 JSON 形状。
- API/Worker 不执行 DDL；迁移由维护者手动执行。
- API DTO 从 unknown 严格解析，禁止 any、as any、@ts-ignore。
- scene 不可编辑；名称、主题、正文、变量、示例值和模板 ID 可编辑。
- 本次不改变腾讯云发送链路；本地 content 用于编辑、预览和复制上传。
- HTML 预览不得执行脚本；不把未净化内容直接插入管理页面 DOM。

---

## Task 1：数据库事实核对、迁移和 Model

文件：
- Create: docs/database/2026-09-14-message-template-content.sql
- Modify: server/internal/module/message/sms/template/model.go
- Modify: server/internal/module/message/mail/template/model.go
- Test: 两个模板模块的 repository/service schema 测试

步骤：
- [ ] 使用项目现有配置只读查询真实 PostgreSQL 的两张模板表：列、约束、活动数据、变量 JSON 形状；不得输出 DSN、密钥或模板敏感内容。
- [ ] 写失败测试，确认 Repository/Service 能读写 Content，Mail 能表示空的腾讯模板 ID。
- [ ] Model 增加 Content string；Mail 的 TencentTemplateID 使用 *int，SMS 继续用空字符串表示未上传。
- [ ] Mail Model 增加 VariableKeys json.RawMessage，避免让对象语义继续叫 variables。
- [ ] 编写事务型幂等 forward migration：两张表增加 content；Mail 增加 variable_keys 并从旧 variables 对象提取 key；code、ttl_minutes 排在最前，其余稳定排序；Mail ID 改为可空；回填四个 SMS 腾讯格式正文和四个完整内联 CSS HTML 邮件正文；content、variable_keys 回填后设为 NOT NULL；删除旧 SMS 固定参数 CHECK；删除 Mail 旧 variables 列；增加 JSON 数组结构约束；异常主动抛错。
- [ ] 不执行迁移，只检查 SQL 事务边界、幂等性和失败回滚。
- [ ] 运行：cd D:\admin\server; go test ./internal/module/message/sms/template ./internal/module/message/mail/template

---

## Task 2：后端动态变量、正文和场景校验

文件：
- Modify: server/internal/module/message/sms/template/{protocol,service,repository,handler}.go
- Modify: server/internal/module/message/mail/template/{protocol,service,repository,handler}.go
- 必要时修改 Mail runtime/readiness/records
- Test: 两个模板模块 service/handler 测试及 Mail runtime/readiness 测试

步骤：
- [ ] 先写失败测试：空/超长正文、非法/重复变量、缺少/多余示例、缺少必需变量、scene 改动、启用时无模板 ID。
- [ ] 统一 API 字段为 content、variableKeys、exampleVariables。
- [ ] SMS 只允许正文中的 {1} 到变量数量，每个位置至少一次，允许重复；拒绝 {0}、缺号、超范围和 {code}。
- [ ] Mail 要求每个变量以 {{key}} 出现在 HTML；拒绝 script、iframe、object、embed、form、事件属性和 javascript URL。
- [ ] 通用变量名符合 ^[a-z][a-z0-9_]{1,63}$，不重复；示例 key 精确匹配且值非空；验证码场景额外要求 code/ttl_minutes，ttl 为 1–60。
- [ ] 保存时 trim；Repository 只做数据库访问；Mail readiness/runtime 只把正整数 ID 作为启用条件。
- [ ] 运行模板和 Mail 定向 Go 测试。

---

## Task 3：后端 API DTO、Repository 和运行时兼容

文件：
- Modify: 两个模板模块的 protocol、repository、handler
- 必要时修改 server/internal/module/message/mail/runtime.go、readiness.go、records.go
- Test: API handler、repository、Mail runtime/readiness 测试

步骤：
- [ ] 为两类列表响应建立显式 Safe DTO，不直接序列化带 RawMessage 的 Model。
- [ ] PUT 请求和响应统一使用 variableKeys，不再暴露旧 variables。
- [ ] Mail ID 响应为 number 或 null；禁用模板允许 null，启用模板必须正整数。
- [ ] 验证上下文从 handler 贯穿 service/repository，不引入后台 context。
- [ ] 先运行失败测试，再验证前后端 JSON 字段完全一致。

---

## Task 4：前端 API DTO 和变量工具

文件：
- Modify: web/src/api/message/sms.ts
- Modify: web/src/api/message/mail.ts
- Create: web/src/views/message/components/templateVariables.ts
- Test: API 测试和变量工具测试

步骤：
- [ ] 先写 parser 失败测试：缺 content、变量不是字符串数组、重复变量、Mail ID 类型错误、未知字段。
- [ ] Template/Input 统一使用 content、variableKeys: string[]、exampleVariables: Record<string,string>；Mail ID 为 number | null。
- [ ] 实现不可变 synchronizeExampleVariables(keys, current)：保留已有值、新变量为空、删除多余 key。
- [ ] 实现 smsPlaceholderMappings(keys)，生成 {1} -> key 映射。
- [ ] 运行 API 和工具定向测试。

---

## Task 5：短信模板 UI

文件：
- Modify: web/src/views/message/sms/template/index.vue
- Modify: web/src/i18n/messages/zh-CN.ts
- Modify: web/src/i18n/messages/en-US.ts
- Test: web/tests/views/message/sms/index.test.ts

步骤：
- [ ] 先写失败 UI 测试：scene disabled、其他字段可编辑、InputTag 初始值、增加/删除/拖动变量、示例同步、位置映射、示例预览、复制和精确保存。
- [ ] 使用 el-input-tag 绑定 form.variableKeys，开启 draggable，保留数组顺序。
- [ ] 动态渲染示例值输入框，不手写固定 code/TTL 字段。
- [ ] 清楚显示 {1} -> code、{2} -> ttl_minutes；正文 textarea 使用腾讯格式。
- [ ] 预览只替换示例值，不修改原文；复制按钮复制原始腾讯正文。
- [ ] Clipboard 不可用或失败时明确报错；按钮使用 el-button 的 :icon 属性。

---

## Task 6：邮件完整 HTML 富文本、源码、预览和复制

依赖：
- @wangeditor-next/editor 6.4.2
- @wangeditor-next/editor-for-vue 6.4.2

文件：
- Create: web/src/views/message/mail/template/MailHtmlEditor.vue
- Create: web/src/views/message/mail/template/mailHtmlDocument.ts
- Modify: web/src/views/message/mail/template/index.vue
- Modify: web/src/i18n/messages/zh-CN.ts
- Modify: web/src/i18n/messages/en-US.ts
- Test: Mail 页面和 HTML 工具测试

步骤：
- [ ] 先写失败测试：完整 HTML 往返、保留 doctype/head/html 属性、只替换 body、变量预览替换、危险 HTML 拒绝、复制完整 HTML。
- [ ] MailHtmlEditor 父子契约：props 为 modelValue 和 exampleVariables；只 emit update:modelValue。
- [ ] 富文本视图只编辑 body innerHTML；源码视图编辑完整 HTML，是保存/复制事实；源码解析失败禁止保存且不静默丢弃。
- [ ] 预览使用 iframe sandbox=""、srcdoc、referrerpolicy="no-referrer"，禁止脚本、表单和同源权限；不使用 v-html。
- [ ] 邮件变量使用 el-input-tag，示例值动态生成；复制按钮复制完整 HTML。
- [ ] 通过 defineAsyncComponent(() => import(...)) 懒加载编辑器和 CSS，检查构建产物为独立 editor chunk。
- [ ] 弹窗宽度适配完整 HTML 编辑，不改全局主题 CSS。
- [ ] 登录场景使用用户提供的完整 HTML，其他场景保持相同邮件布局。
- [ ] 运行 Mail 页面和 HTML 工具定向测试。

---

## Task 7：跨层验证和交付

- [ ] 后端运行：cd D:\admin\server; go fmt ./...; go vet ./...; go test ./...; go build ./...
- [ ] 前端运行：cd D:\admin\web; pnpm lint; pnpm check:architecture; pnpm typecheck; pnpm vitest run --pool=threads --maxWorkers=1; pnpm build
- [ ] 检查编辑器独立异步 chunk，入口无新的大 chunk 警告。
- [ ] 最终检查：cd D:\admin; git status --short; git diff --check
- [ ] 只更新 docs/agent/STATUS.md，记录实现、实际验证、迁移未执行、真实腾讯上传/发送未验证。
- [ ] 不提交 Git。

## 验收标准

1. 短信可编辑、预览并复制腾讯 {n} 正文，位置映射清晰。
2. 邮件可编辑完整 HTML、富文本 body、预览并复制完整 HTML。
3. 两类模板均用 el-input-tag 管理动态变量和示例值。
4. 除 scene 外字段可编辑；未上传模板可无 ID，但不能启用。
5. 当前验证码场景继续要求 code、ttl_minutes，未来变量可追加。
6. API/数据库统一为 variableKeys/variable_keys，无重复旧字段。
7. 富文本编辑器懒加载，不破坏 chunk 优化。
8. 不执行迁移、不调用真实腾讯云、不自动提交。
