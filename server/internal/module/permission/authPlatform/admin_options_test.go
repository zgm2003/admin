package authplatform

import (
	"admin/server/internal/shared/i18n"
	"context"
	"testing"
)

func TestAdminOptionsOwnLocalizedValues(t *testing.T) {
	zh := adminOptions(i18n.WithLocale(context.Background(), i18n.ZhCN))
	en := adminOptions(i18n.WithLocale(context.Background(), i18n.EnUS))
	if len(zh.LoginTypes) != 3 || len(en.LoginTypes) != 3 {
		t.Fatalf("catalog sizes: %#v / %#v", zh, en)
	}
	if zh.LoginTypes[0].Value != "email" || en.LoginTypes[0].Value != zh.LoginTypes[0].Value {
		t.Fatal("option value changed")
	}
	if zh.LoginTypes[0].Label == en.LoginTypes[0].Label || zh.LoginTypes[0].Label == "" {
		t.Fatal("option label must follow request locale")
	}
}
