package menu

import (
	"admin/server/internal/shared/i18n"
	"context"
	"testing"
)

func TestMenuOptionsUseWriteRules(t *testing.T) {
	zh := formOptions(i18n.WithLocale(context.Background(), i18n.ZhCN))
	en := formOptions(i18n.WithLocale(context.Background(), i18n.EnUS))
	if len(zh.MenuTypes) != 3 || zh.MenuTypes[0].Value != TypeDirectory || zh.MenuTypes[0].Label == en.MenuTypes[0].Label {
		t.Fatal("menu types are not localized")
	}
	if zh.Constraints.CodePattern != menuCodePattern.String() || zh.Constraints.PathPattern != menuPathPattern.String() || zh.Constraints.NameMaxLength != menuTextMaxRunes || zh.Constraints.PathMaxLength != menuPathMaxRunes {
		t.Fatal("menu constraints diverge from write validation")
	}
}
