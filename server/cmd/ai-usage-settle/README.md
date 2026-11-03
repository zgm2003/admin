# ai-usage-settle

对既有、用量已知的 Charge 检查冻结账单事实，或显式重试原结算服务。未知用量使用 `ai-usage-reconcile` 提交证据，本命令不补用量、不修改价格、不新增或清除账单。

在 `server` 目录运行；使用环境或本地 `.env` 中现有 `POSTGRES_DSN`，不要把 DSN 放入命令行：

```powershell
go run ./cmd/ai-usage-settle --charge-id TARGET
go run ./cmd/ai-usage-settle --charge-id TARGET --apply
```

`TARGET` 是既有 Charge 的正整数 ID。仅支持 `--charge-id` 和显式 `--apply`，不接受用户、金额、倍率或价格参数。

默认 dry-run 调用 `usage.Service.Preview`，在现有 `WithCharge` 的短事务和 Charge → Wallet → Run 行锁下读取一致事实，核对请求、冻结费率、全部 items 与既有 Ledger。它不写数据、不记录失败、不扣款，也不调用 `MarkFailure`。这与 `ai-usage-reconcile` 的 PostgreSQL `READ ONLY` dry-run 不同：本命令取得短暂行锁，可能等待并发结算。

`--apply` 先预览，再调用现有 `usage.Service.Settle`，适用于待结算或 retryPending（包括自动重试耗尽）的已知账单；已结算重放继续校验 Ledger，不重复扣款。所有权、价格、金额、Ledger 唯一性、余额变化与 reservation 释放均由原结算服务处理；失败记录也沿用原服务。没有供应商请求、迁移或后台服务启动。

输出只含公开 Charge/Run ID、状态、币种、既有金额、Ledger 是否存在和 dry-run 标识。若执行返回错误或报告打印失败，先检查持久 Charge 与 Ledger 再重试；不能根据终端输出认定没有扣款。预览不会证明后续结算必然成功，apply 会在新的事务内重新核验全部事实。
