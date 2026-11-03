# ai-usage-reconcile

仅核对已经终结、仍保留未知消费 reservation 的 Run。默认 dry-run：PostgreSQL 只读、可重复读事务，不修改请求、账单、任务或钱包。无供应商请求、无模型重发、无迁移、无 Redis 依赖；不会启动 API/Worker。

在 `server` 目录运行，使用现有 `POSTGRES_DSN`（可从本地 `.env` 读取，禁止将 DSN 放入命令参数或审计文件）：

```powershell
go run ./cmd/ai-usage-reconcile --run-id TARGET --evidence ./evidence.json
go run ./cmd/ai-usage-reconcile --run-id TARGET --evidence ./evidence.json --operator TARGET --apply
```

`TARGET` Run ID 替换为正整数，`--operator` 替换为可追溯的维护者标识。`--apply` 必须显式提供，且必须提供操作者。命令不接受扣费用户、余额或任意金额。

## 证据格式

证据为本地、最大 1 MiB 的 JSON 普通文件。拒绝额外字段、重复 JSON 键、重复 request ID 和不完整用量。下列 `TARGET` 必须替换为从现存 Run、请求事实和供应商报告核对得到的值；所有 ID 是十进制字符串：

```json
{
  "schemaVersion": 1,
  "runId": "TARGET",
  "operation": "text.generate",
  "providerModelId": "TARGET",
  "priceVersionId": "TARGET",
  "snapshotDigest": "TARGET",
  "requests": [
    {
      "requestId": "TARGET",
      "requestKey": "TARGET",
      "requestFingerprint": "TARGET",
      "upstreamRequestId": "TARGET",
      "sourceReference": "TARGET",
      "observedAt": "2026-10-29T00:00:00Z",
      "status": 2,
      "usage": {
        "known": true,
        "inputTokens": 0,
        "cachedInputTokens": 0,
        "outputTokens": 0
      }
    }
  ]
}
```

示例零计数只是格式，不构成用量证据；必须替换成供应商确认的实际计数。`status` 为 2（供应商完成）或 3（确认失败但用量已知）。`observedAt` 是证据观察时刻，不能早于请求或晚于当前时间五分钟。`sourceReference` 是保留的供应商导出/报告定位；不得填 Token、密码、带凭据 URL 或原始认证头。

- `snapshotDigest` 必须等于现有 Charge 的摘要，且等于 `SHA256(trim(config_snapshot) || 0x00 || trim(policy_snapshot))`；使用数据库当前读取出的冻结文本计算，不能对 JSON 自行重新排版。
- 每条证据逐一核对请求 ID、Run、requestKey、requestFingerprint 和已有 upstreamRequestId。已有上游 ID 不允许替换；未知 ID 需要证据提供实际供应商追踪 ID。
- 文字计数必须显式包含 `known/inputTokens/cachedInputTokens/outputTokens`；缓存输入包含于 inputTokens，不能重复算。
- 图片使用 `known/imageCount`（必须明确实际生成数）；音频使用现有 `MediaUsage` 的 `basis/durationSeconds/inputTokens/outputTokens/audioInputTokens/textInputTokens` 中冻结费率要求的实际维度，缺少即拒绝核对，不以提示词或请求数量估算。
- 命令验证本地证据与持久身份、不可变价格的对应关系，不会自行联系供应商来证明文件内容。维护者必须保存可追溯原始证据。

## Apply 与恢复

Apply 在一个短事务内锁定 Charge → Run → Request，仅补充未知请求，将证据原文件 SHA-256、操作者、供应商定位、观察/应用时间写入请求 `request_summary.reconciliation`。原文件保留在维护者的审计存储，不把原始认证信息写进数据库。

证据只覆盖部分请求、或还有未知工具副作用时，请求证据可以被记录，但 Charge 保持 reconciliationRequired，Run 与 reservation 继续阻止新消费。所有消费事实完整后，使用生成终结流程相同的 `textUsageLines/mediaUsageLines` 和 `usage.Calculate`，按原冻结费率/倍率准备一次 Charge items；只把未知 Charge 4 改为待结算 1，保持 Run 已有执行终态和 reservation=1，并写入同一 `ai.usage.settle` 任务。钱包、交易唯一约束和 reservation 释放仍由现有 usage settlement 服务处理。

相同文件重放只核对既有审计，不重复写 items/排任务。已知请求不能被另一份证据改写；已有 items、金额或价格不匹配时拒绝，不删除账单、不重开 Run、不重新生成。

输出是 JSON dry-run/apply 报告，只含 Run/Charge ID、证据摘要、是否完整、拟/实际用量金额和排队结论。若打印报告失败或事务提交结果未知，应先读取持久 reconciliation 审计再重试，不能认定未写入。
