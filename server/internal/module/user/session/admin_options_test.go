package session

import (
	"admin/server/internal/shared/i18n"
	"context"
	"testing"
)

func TestAdminOptionsOwnLocalizedValues(t *testing.T) {
	zh := adminOptions(i18n.WithLocale(context.Background(), i18n.ZhCN))
	en := adminOptions(i18n.WithLocale(context.Background(), i18n.EnUS))
	if len(zh.Statuses) != 3 || len(en.Statuses) != 3 {
		t.Fatalf("catalog sizes: %#v / %#v", zh, en)
	}
	if zh.Statuses[0].Value != "active" || en.Statuses[0].Value != zh.Statuses[0].Value {
		t.Fatal("option value changed")
	}
	if zh.Statuses[0].Label == en.Statuses[0].Label || zh.Statuses[0].Label == "" {
		t.Fatal("option label must follow request locale")
	}
}
