package loginlog

import (
	"admin/server/internal/shared/i18n"
	"context"
	"testing"
)

func TestAdminOptionsOwnLocalizedValues(t *testing.T) {
	zh, err := adminOptions(i18n.WithLocale(context.Background(), i18n.ZhCN))
	if err != nil {
		t.Fatal(err)
	}
	en, err := adminOptions(i18n.WithLocale(context.Background(), i18n.EnUS))
	if err != nil {
		t.Fatal(err)
	}
	if len(zh.EventTypes) != 3 || len(en.EventTypes) != 3 {
		t.Fatalf("catalog sizes: %#v / %#v", zh, en)
	}
	if zh.EventTypes[0].Value != 1 || en.EventTypes[0].Value != zh.EventTypes[0].Value {
		t.Fatal("option value changed")
	}
	if zh.EventTypes[0].Label == en.EventTypes[0].Label || zh.EventTypes[0].Label == "" {
		t.Fatal("option label must follow request locale")
	}
}
