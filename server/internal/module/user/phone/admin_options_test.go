package phone

import (
	"admin/server/internal/shared/i18n"
	"context"
	"testing"
)

func TestAdminOptionsOwnLocalizedValues(t *testing.T) {
	zh := adminOptions(i18n.WithLocale(context.Background(), i18n.ZhCN))
	en := adminOptions(i18n.WithLocale(context.Background(), i18n.EnUS))
	if len(zh.Actions) != 2 || len(en.Actions) != 2 {
		t.Fatalf("catalog sizes: %#v / %#v", zh, en)
	}
	if zh.Actions[0].Value != 1 || en.Actions[0].Value != zh.Actions[0].Value {
		t.Fatal("option value changed")
	}
	if zh.Actions[0].Label == en.Actions[0].Label || zh.Actions[0].Label == "" {
		t.Fatal("option label must follow request locale")
	}
}
