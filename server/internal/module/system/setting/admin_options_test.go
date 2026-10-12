package setting

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
	if len(zh.ValueTypes) != 6 || len(en.ValueTypes) != 6 {
		t.Fatalf("catalog sizes: %#v / %#v", zh, en)
	}
	if zh.ValueTypes[4].Label != "媒体-文件" || zh.ValueTypes[5].Label != "媒体-图片/视频" {
		t.Fatalf("media type labels: %#v", zh.ValueTypes)
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
