package transaction

import (
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/option"
	"context"
)

type Options struct {
	Directions  []option.Option[Direction]  `json:"directions"`
	SourceTypes []option.Option[SourceType] `json:"sourceTypes"`
}

func (s *Service) Options(ctx context.Context) (Options, error) {
	if err := ctx.Err(); err != nil {
		return Options{}, apperror.DependencyUnavailable(err)
	}
	return Options{
		Directions:  []option.Option[Direction]{option.New(ctx, DirectionCredit, "收入", "Credit"), option.New(ctx, DirectionDebit, "消费", "Debit")},
		SourceTypes: []option.Option[SourceType]{option.New(ctx, SourcePayment, "支付充值", "Payment"), option.New(ctx, SourceRedeemCode, "兑换码", "Redeem code"), option.New(ctx, SourceAI, "AI 消费", "AI usage")},
	}, nil
}
