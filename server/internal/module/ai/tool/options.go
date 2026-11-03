package tool

import (
	"admin/server/internal/shared/option"
	"context"
)

func Options(ctx context.Context) any {
	return struct {
		Statuses         []option.Option[int16]  `json:"statuses"`
		Types            []option.Option[int16]  `json:"types"`
		Builtins         []option.Option[string] `json:"builtins"`
		IdempotencyModes []option.Option[int16]  `json:"idempotencyModes"`
		Defaults         map[string]int          `json:"defaults"`
		Constraints      map[string]int          `json:"constraints"`
	}{Statuses: []option.Option[int16]{option.New(ctx, int16(0), "停用", "Disabled"), option.New(ctx, int16(1), "启用", "Enabled")}, Types: []option.Option[int16]{option.New(ctx, int16(1), "HTTP", "HTTP"), option.New(ctx, int16(2), "内置", "Built-in")}, Builtins: []option.Option[string]{option.New(ctx, "currentTime", "当前时间", "Current time"), option.New(ctx, "calculator", "计算器", "Calculator")}, IdempotencyModes: []option.Option[int16]{option.New(ctx, int16(0), "无副作用", "No side effects"), option.New(ctx, int16(1), "上游幂等键", "Upstream idempotency"), option.New(ctx, int16(2), "查询确认", "Query confirmation"), option.New(ctx, int16(3), "人工确认", "Manual confirmation")}, Defaults: map[string]int{"status": 0, "timeoutMs": 10000, "outputLimitBytes": 65536}, Constraints: map[string]int{"maxPageSize": 100, "minTimeoutMs": 100, "maxTimeoutMs": 30000, "maxOutputLimitBytes": 65536, "maxSchemaBytes": 65536, "maxSchemaDepth": 16}}
}
