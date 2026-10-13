package config

import (
	"admin/server/internal/module/payment"
	"admin/server/internal/payment/alipay"
	"admin/server/internal/shared/option"
	"admin/server/internal/shared/yesno"
	"context"
)

type Limits struct {
	Code             int   `json:"code"`
	Name             int   `json:"name"`
	Identity         int   `json:"identity"`
	NotifyURL        int   `json:"notifyUrl"`
	CertificateBytes int   `json:"certificateBytes"`
	PrivateKeyBytes  int   `json:"privateKeyBytes"`
	SortMinimum      int32 `json:"sortMinimum"`
	SortMaximum      int32 `json:"sortMaximum"`
}
type Defaults struct {
	Provider    payment.Provider    `json:"provider"`
	Environment payment.Environment `json:"environment"`
	PayMethods  []payment.PayMethod `json:"payMethods"`
	IsEnabled   yesno.Value         `json:"isEnabled"`
	SortOrder   int32               `json:"sortOrder"`
}
type Options struct {
	Providers          []option.Option[payment.Provider]    `json:"providers"`
	Environments       []option.Option[payment.Environment] `json:"environments"`
	PayMethods         []option.Option[payment.PayMethod]   `json:"payMethods"`
	Statuses           []option.Option[yesno.Value]         `json:"statuses"`
	Defaults           Defaults                             `json:"defaults"`
	Limits             Limits                               `json:"limits"`
	NotifyPathTemplate string                               `json:"notifyPathTemplate"`
}

func (s *Service) Options(ctx context.Context) (Options, error) {
	if err := ctx.Err(); err != nil {
		return Options{}, dependency(err)
	}
	return Options{Providers: []option.Option[payment.Provider]{option.New(ctx, payment.ProviderAlipay, "支付宝", "Alipay")}, Environments: []option.Option[payment.Environment]{option.New(ctx, payment.EnvironmentSandbox, "沙箱", "Sandbox"), option.New(ctx, payment.EnvironmentProduction, "生产", "Production")}, PayMethods: []option.Option[payment.PayMethod]{option.New(ctx, payment.PayMethodWeb, "PC 网页", "PC Web"), option.New(ctx, payment.PayMethodH5, "H5 网页", "H5 Web")}, Statuses: []option.Option[yesno.Value]{option.New(ctx, yesno.No, "停用", "Disabled"), option.New(ctx, yesno.Yes, "启用", "Enabled")}, Defaults: Defaults{Provider: 1, Environment: 1, PayMethods: []payment.PayMethod{1, 2}}, Limits: Limits{Code: 64, Name: 128, Identity: 64, NotifyURL: 2048, CertificateBytes: alipay.MaxCertificateBytes, PrivateKeyBytes: alipay.MaxPrivateKeyBytes, SortMinimum: -2147483648, SortMaximum: 2147483647}, NotifyPathTemplate: NotifyPathTemplate}, nil
}
