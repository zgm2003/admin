package cachegeneration

import (
	"admin/server/internal/shared/i18n"
	"context"
	"encoding/json"
	"testing"
)

func TestCacheDisplayOwnedByBackend(t *testing.T) {
	for _, locale := range []i18n.Locale{i18n.ZhCN, i18n.EnUS} {
		ctx := i18n.WithLocale(context.Background(), locale)
		value, err := listItemResponse(ctx, Item{Namespace: "system.setting", ScopeKey: "global", Status: StatusMissing})
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var row struct {
			NamespaceLabel string
			ScopeLabel     string
			StatusLabel    string
			StatusHint     string
			StatusTone     string
		}
		if err = json.Unmarshal(data, &row); err != nil {
			t.Fatal(err)
		}
		if row.NamespaceLabel == "" || row.ScopeLabel == "" || row.StatusLabel == "" || row.StatusHint == "" || row.StatusTone != "warning" {
			t.Fatalf("missing display: %s", data)
		}
	}
	unknown, err := listItemResponse(context.Background(), Item{Namespace: "new.domain", ScopeKey: "local", Status: "future"})
	if err != nil {
		t.Fatal(err)
	}
	if unknown.NamespaceLabel != "new.domain" || unknown.StatusLabel != "future" || unknown.StatusTone != "info" {
		t.Fatalf("unknown values changed: %#v", unknown)
	}
	zh, err := formOptions(i18n.WithLocale(context.Background(), i18n.ZhCN))
	if err != nil {
		t.Fatal(err)
	}
	en, err := formOptions(i18n.WithLocale(context.Background(), i18n.EnUS))
	if err != nil {
		t.Fatal(err)
	}
	if len(zh.PublishStates) != 3 || zh.PublishStates[0].Value != PublishStateReady || zh.PublishStates[0].Label == en.PublishStates[0].Label {
		t.Fatal("publish options not localized")
	}
}
