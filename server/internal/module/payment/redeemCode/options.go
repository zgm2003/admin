package redeemcode

import (
	"admin/server/internal/shared/option"
	"context"
)

func (s *Service) Options(ctx context.Context) (Options, error) {
	if err := ctx.Err(); err != nil {
		return Options{}, mapError(err)
	}
	return Options{
		Limits:   Limits{MaxBatchQuantity: 1000, MaxVoidQuantity: 1000, MaxExportQuantity: 10000, MaxCodeLength: 128, CodeLength: 20, Remark: 255, MinimumAmount: "0.01", MaximumAmount: "92233720368.54", AmountScale: 2},
		Defaults: Defaults{Quantity: 1},
		Statuses: []StatusOption{statusOption(ctx, StatusUnused, "未使用", "Unused"), statusOption(ctx, StatusUsed, "已使用", "Used"), statusOption(ctx, StatusVoided, "已作废", "Voided"), statusOption(ctx, StatusExpired, "已过期", "Expired")},
	}, nil
}
func statusOption(ctx context.Context, value Status, zh, en string) StatusOption {
	v := option.New(ctx, value, zh, en)
	return StatusOption{Value: v.Value, Label: v.Label}
}
