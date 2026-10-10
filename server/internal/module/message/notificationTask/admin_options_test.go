package notificationtask

import (
	"admin/server/internal/module/message/notification"
	"admin/server/internal/shared/i18n"
	"context"
	"testing"
)

func TestAdminOptionsOwnLocalizedValues(t *testing.T) {
	zh := adminOptions(i18n.WithLocale(context.Background(), i18n.ZhCN))
	en := adminOptions(i18n.WithLocale(context.Background(), i18n.EnUS))
	if len(zh.Statuses) != 7 || len(en.Statuses) != 7 {
		t.Fatalf("catalog sizes: %#v / %#v", zh, en)
	}
	if zh.Statuses[0].Value != 1 || en.Statuses[0].Value != zh.Statuses[0].Value {
		t.Fatal("option value changed")
	}
	if zh.Statuses[0].Label == en.Statuses[0].Label || zh.Statuses[0].Label == "" {
		t.Fatal("option label must follow request locale")
	}
	if zh.Defaults.Variant != notification.VariantInfo || zh.Defaults.Priority != notification.PriorityNormal || zh.Defaults.LinkType != notification.LinkNone || zh.Defaults.AudienceType != AudiencePlatform {
		t.Fatalf("incorrect create defaults: %#v", zh.Defaults)
	}
	if zh.Constraints.TitleMaxLength != notification.TitleMaxLength {
		t.Fatal("title constraint must use the write validator constant")
	}
}
