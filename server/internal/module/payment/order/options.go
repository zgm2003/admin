package order

import (
	"admin/server/internal/module/payment"
	"admin/server/internal/shared/option"
)

type Checkout struct {
	Order SafeValue `json:"order"`
	URL   string    `json:"checkout"`
}

type PackageOption struct {
	ID     int64   `json:"id"`
	Name   string  `json:"name"`
	Amount string  `json:"amount"`
	Badge  *string `json:"badge"`
}

type RechargeLimits struct {
	MinimumAmount string `json:"minimumAmount"`
	MaximumAmount string `json:"maximumAmount"`
	AmountScale   int    `json:"amountScale"`
}

type Options struct {
	PayMethods []option.Option[payment.PayMethod] `json:"payMethods"`
	Packages   []PackageOption                    `json:"packages"`
	Available  bool                               `json:"available"`
	Limits     RechargeLimits                     `json:"limits"`
}
