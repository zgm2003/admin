package setting

import (
	"admin/server/internal/shared/i18n"
	"context"
	"testing"
)

func TestAdminOptionsOwnLocalizedValues(t *testing.T) {
	zh := adminOptions(i18n.WithLocale(context.Background(), i18n.ZhCN))
	en := adminOptions(i18n.WithLocale(context.Background(), i18n.EnUS))
	if len(zh.ValueTypes) != 5 || len(en.ValueTypes) != 5 {
		t.Fatalf("catalog sizes: %#v / %#v", zh, en)
	}
	if zh.ValueTypes[0].Value != 1 || en.ValueTypes[0].Value != zh.ValueTypes[0].Value {
		t.Fatal("option value changed")
	}
	if zh.ValueTypes[0].Label == en.ValueTypes[0].Label || zh.ValueTypes[0].Label == "" {
		t.Fatal("option label must follow request locale")
	}
	for _, item := range zh.ValueTypes {
		if item.AllowEmpty != validSettingValue("", item.Value) {
			t.Fatal("empty form rule must match write validator")
		}
	}
}
