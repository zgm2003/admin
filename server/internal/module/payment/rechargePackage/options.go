package rechargepackage

import (
	"context"

	"admin/server/internal/shared/money"
	"admin/server/internal/shared/option"
	"admin/server/internal/shared/yesno"
)

type Limits struct {
	Name          int    `json:"name"`
	Badge         int    `json:"badge"`
	AmountScale   int    `json:"amountScale"`
	MinimumAmount string `json:"minimumAmount"`
	MaximumAmount string `json:"maximumAmount"`
	SortMinimum   int32  `json:"sortMinimum"`
	SortMaximum   int32  `json:"sortMaximum"`
}

type Defaults struct {
	SortOrder int32       `json:"sortOrder"`
	IsEnabled yesno.Value `json:"isEnabled"`
}

type Options struct {
	Statuses []option.Option[yesno.Value] `json:"statuses"`
	Defaults Defaults                     `json:"defaults"`
	Limits   Limits                       `json:"limits"`
}

func (s *Service) Options(ctx context.Context) (Options, error) {
	if err := ctx.Err(); err != nil {
		return Options{}, dependency(err)
	}
	maxCents := int64(^uint64(0) >> 1)
	if maxCents > 9223372036854 {
		maxCents = 9223372036854
	}
	maximum, err := money.FormatRMBCents(maxCents)
	if err != nil {
		return Options{}, dependency(err)
	}
	return Options{
		Statuses: []option.Option[yesno.Value]{
			option.New(ctx, yesno.No, "停用", "Disabled"),
			option.New(ctx, yesno.Yes, "启用", "Enabled"),
		},
		Defaults: Defaults{SortOrder: 0, IsEnabled: yesno.No},
		Limits:   Limits{Name: MaxNameLength, Badge: MaxBadgeLength, AmountScale: 2, MinimumAmount: "0.01", MaximumAmount: maximum, SortMinimum: 0, SortMaximum: 2147483647},
	}, nil
}
