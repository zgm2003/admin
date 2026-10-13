package configcachestatus

import (
	"admin/server/internal/shared/i18n"
	"context"
	"testing"
)

func TestPaymentNamespacePresentation(t *testing.T) {
	zh, err := namespaceLabel(i18n.WithLocale(context.Background(), i18n.ZhCN), "payment.config")
	if err != nil || zh != "支付配置" {
		t.Fatalf("%q %v", zh, err)
	}
	en, err := namespaceLabel(i18n.WithLocale(context.Background(), i18n.EnUS), "payment.config")
	if err != nil || en != "Payment configuration" {
		t.Fatalf("%q %v", en, err)
	}
}
