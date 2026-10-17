package rechargepackage

import (
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSafeValueUsesDecimalAmountAndLocalizedActions(t *testing.T) {
	badge := "Popular"
	row := Model{ID: 2, Name: "Starter", AmountCents: 1234, Badge: &badge, IsEnabled: yesno.Yes, CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(2, 0).UTC()}
	for _, test := range []struct {
		locale i18n.Locale
		label  string
	}{{i18n.ZhCN, "启用"}, {i18n.EnUS, "Enabled"}} {
		ctx := i18n.WithLocale(context.Background(), test.locale)
		value, err := safeValue(ctx, row)
		if err != nil || value.Amount != "12.34" || value.IsEnabled != yesno.Yes || value.Presentation.Status != test.label || !value.Actions.Update || !value.Actions.Status || !value.Actions.Delete {
			t.Fatalf("value=%+v err=%v", value, err)
		}
		raw, err := json.Marshal(value)
		if err != nil || !strings.Contains(string(raw), `"amount":"12.34"`) || strings.Contains(string(raw), `"amountCents"`) || strings.Contains(string(raw), `"code"`) {
			t.Fatalf("transport amount is not a decimal string: %s %v", raw, err)
		}
	}
}

func TestSafeValueRejectsCorruptRowsAndPreservesNullBadge(t *testing.T) {
	row := Model{ID: 1, Name: "Starter", AmountCents: 1, IsEnabled: yesno.No}
	value, err := safeValue(context.Background(), row)
	if err != nil || value.Badge != nil {
		t.Fatalf("null badge: %+v %v", value, err)
	}
	row.AmountCents = 0
	if _, err := safeValue(context.Background(), row); err == nil {
		t.Fatal("corrupt nonpositive amount rendered")
	}
}
