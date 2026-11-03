# Provider / Price Slice 报告

## 修改
- `server/internal/module/ai/provider/`：Provider/ProviderVersion/ProviderModel DTO、分层仓储与服务；普通更新 DTO 不包含状态/价格，凭据版本不可变，空 API Key 保留旧密文。HTTPS Base URL、请求头、媒体域名 allowlist、超时和模型操作严格校验；凭据使用 AI 专用 HKDF + AES-GCM 随机 nonce，DTO/日志不暴露密文。
- `provider/model/`：Provider Model CRUD；价格预览/同步、人民币初始复制、价格身份（operation/usage_key/selector/unit/unit_quantity）、已有人民币价保留、精确小数溢出留空待人工填写、预览确认发布、发布版本/费率不可变、按 Run acceptedAt 查询生效版本。同步/发布/改价在 Provider Model 行锁中执行。
- `officialModel/`：标准模型 CRUD，source URL 固定 HTTPS，能力候选由服务端校验；前端标准模型页提供 CRUD、状态切换和已生效供应商价格只读聚合。
- 前端：`web/src/api/ai/provider.ts`、`officialModel.ts` 提供明确 DTO 与字段投影；Provider 页覆盖凭据版本配置、模型目录预览/显式能力确认、模型状态和价格版本；Standard Model 页覆盖 CRUD、状态及只读价格聚合。独立双语 flat locale 位于 `web/src/i18n/messages/aiProvider.ts` 和 `aiOfficialModel.ts`，由主 locale 装配。
- OpenAI 适配器：固定官方 `pricing.md`、`image-generation.md`、`speech-to-text.md`、`text-to-speech.md` 四份 Markdown；独立 `/models` 身份目录客户端；兼容 Base URL 不套官方价格，模型与价格严格按身份筛选。
- `provider/model/testdata/openai-pricing.md`：2026-10-29 无密钥 HTTP 200 采集的四份官方 Markdown 合并 fixture（191,848 bytes，SHA-256 `f93236020bd0b4c4048cd0dd50a8f72da5d50d72e791612c535b242b35454b1c`），保留每段 `SOURCE` URL 注释及 SHA-256 摘要进入价格快照。
- 价格维度：图像 grouped token（Image/Text modality）及 legacy 每张图 `quality × size`；GPT audio/realtime 与 `gpt-4o-mini-tts` 的音频 token、`tts-1`/`tts-1-hd` 字符；转写模型的 token 与每分钟音频。Sora/Videos API 官方文档明确记载 2026-09-24 关闭，options 不再提供 `video.generate`。

## 验证
- `cd D:\admin\server; go test -short ./internal/module/ai/provider/... ./internal/module/ai/officialModel`：通过。
- 同一命令覆盖图像 quality/size 每张、grouped image token、TTS 字符/token、transcription 分钟/token fixture 解析。
- `go vet ./internal/module/ai/provider/... ./internal/module/ai/officialModel`：通过。
- `go build ./internal/module/ai/provider/... ./internal/module/ai/officialModel`：通过。
- `git diff --check -- server/internal/module/ai/provider server/internal/module/ai/officialModel`：通过。
- `cd D:\admin\web; pnpm vitest run --pool=threads --maxWorkers=1 tests/api/ai/provider.test.ts`：通过（5 tests）。
- `cd D:\admin\web; pnpm exec vue-tsc --noEmit --pretty false`：通过。
- `cd D:\admin\web; pnpm check:architecture`：通过（0 baseline findings）。
- `cd D:\admin\web; pnpm exec prettier --check <AI 前端文件>`：通过。
- `cd D:\admin\server; go test -short ./internal/module/ai/provider -run TestProviderRoutes -count=1`：通过。
- PostgreSQL 集成用例已写入 `provider/model/repository_integration_test.go`，采用 `testschema.Open` 独立 schema 和 Silent logger；未运行（并行环境此前连接超时）。串行命令：`cd D:\admin\server; go test ./internal/module/ai/provider/model -run Integration -count=1`。

## 风险
- cmd 装配、主 locale spread、菜单与动态路由由主代理接入；本分支已补 Provider 模型目录同步专用候选权限路由 `ai:provider:sync`。
- 价格同步拒绝未知/畸形图像 quality、size、音频单位，不将视频能力映射成可用价格；官方文档格式变化会拒绝同步，不静默写入。
- 未运行 PostgreSQL 集成测试（由主代理串行运行）。
- Provider/OfficialModel 页面集成挂载测试未纳入本次提交：当前 ElementPlus jsdom 组合会在隐藏对话框中的 `el-select-v2` 触发递归更新；页面已通过 `vue-tsc` 与架构检查，API 契约测试保留并通过。
