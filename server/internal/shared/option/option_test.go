package option

import (
	"context"
	"testing"

	"admin/server/internal/shared/i18n"
)

func TestNewLocalizedUsesTheRequestedCatalogLocale(t *testing.T) {
	for _, test := range []struct {
		locale i18n.Locale
		want   string
	}{
		{locale: i18n.ZhCN, want: "待发送"},
		{locale: i18n.EnUS, want: "Pending"},
	} {
		ctx := i18n.WithLocale(context.Background(), test.locale)
		got, err := NewLocalized(ctx, 1, i18n.MessageKey("mail.options.status.pending"), nil)
		if err != nil {
			t.Fatalf("NewLocalized(%q) error = %v", test.locale, err)
		}
		if got.Value != 1 || got.Label != test.want {
			t.Fatalf("NewLocalized(%q) = %+v, want label %q", test.locale, got, test.want)
		}
	}
}

func TestNewLocalizedRejectsMissingCatalogKeys(t *testing.T) {
	if _, err := NewLocalized(context.Background(), "pending", i18n.MessageKey("mail.options.missing"), nil); err == nil {
		t.Fatal("NewLocalized accepted a missing catalog key")
	}
}
