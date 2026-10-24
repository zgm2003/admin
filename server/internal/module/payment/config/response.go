package config

import (
	"admin/server/internal/module/payment"
	"admin/server/internal/payment/alipay"
	"admin/server/internal/shared/option"
	"admin/server/internal/shared/yesno"
	"context"
	"fmt"
	"time"
)

type Actions struct {
	Update bool `json:"update"`
	Status bool `json:"status"`
	Delete bool `json:"delete"`
	Test   bool `json:"test"`
}
type Presentation struct {
	Provider    string                             `json:"provider"`
	Environment string                             `json:"environment"`
	PayMethods  []option.Option[payment.PayMethod] `json:"payMethods"`
	Status      string                             `json:"status"`
}
type SafeValue struct {
	ID                   int64                     `json:"id"`
	Name                 string                    `json:"name"`
	Provider             payment.Provider          `json:"provider"`
	Environment          payment.Environment       `json:"environment"`
	AppID                string                    `json:"appId"`
	SellerID             string                    `json:"sellerId"`
	CurrentVersion       int64                     `json:"currentVersion"`
	SortOrder            int32                     `json:"sortOrder"`
	Status               yesno.Value               `json:"status"`
	NotifyURL            string                    `json:"notifyUrl"`
	PayMethods           []payment.PayMethod       `json:"payMethods"`
	CredentialConfigured bool                      `json:"credentialConfigured"`
	Certificates         alipay.CertificateSummary `json:"certificates"`
	Actions              Actions                   `json:"actions"`
	Presentation         Presentation              `json:"presentation"`
	CreatedAt            time.Time                 `json:"createdAt"`
	UpdatedAt            time.Time                 `json:"updatedAt"`
}

func safeValue(ctx context.Context, c Current) (SafeValue, error) {
	m, v := c.Config, c.Credential
	certs, err := alipay.InspectCertificates(v.AppCertPEM, v.AlipayCertPEM, v.RootCertPEM)
	if err != nil {
		return SafeValue{}, err
	}
	if _, err := notifyTemplate(v.NotifyURL, m.ID, v.Version); err != nil {
		return SafeValue{}, err
	}
	provider := option.New(ctx, m.Provider, "支付宝", "Alipay").Label
	if m.Provider != 1 {
		provider = fmt.Sprint(m.Provider)
	}
	environment := option.New(ctx, m.Environment, "沙箱", "Sandbox").Label
	if m.Environment == 2 {
		environment = option.New(ctx, m.Environment, "生产", "Production").Label
	}
	methods := make([]option.Option[payment.PayMethod], 0, len(v.PayMethods))
	for _, method := range v.PayMethods {
		switch method {
		case 1:
			methods = append(methods, option.New(ctx, method, "PC 网页", "PC Web"))
		case 2:
			methods = append(methods, option.New(ctx, method, "H5 网页", "H5 Web"))
		}
	}
	status := option.New(ctx, m.Status, "停用", "Disabled").Label
	if m.Status == 1 {
		status = option.New(ctx, m.Status, "启用", "Enabled").Label
	}
	return SafeValue{ID: m.ID, Name: m.Name, Provider: m.Provider, Environment: m.Environment, AppID: m.AppID, SellerID: m.SellerID, CurrentVersion: m.CurrentVersion, SortOrder: m.SortOrder, Status: m.Status, NotifyURL: v.NotifyURL, PayMethods: append([]payment.PayMethod(nil), v.PayMethods...), CredentialConfigured: v.PrivateKeyCiphertext != "", Certificates: certs, Actions: Actions{Update: true, Status: true, Delete: true, Test: true}, Presentation: Presentation{provider, environment, methods, status}, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}, nil
}
