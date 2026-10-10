package authplatform

import (
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
	"context"
	"encoding/json"
	"testing"
)

func TestAdminListOwnsPresentationAndActions(t *testing.T) {
	body, err := newListResponse(context.Background(), []ListItem{{Platform: Platform{ID: 1, LoginTypes: []byte(`["email","password"]`), MaxSessions: 0, IsBuiltin: yesno.Yes}}}, 1, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(body)
	var decoded struct {
		List []struct {
			Presentation struct {
				MaxSessionsLabel string                          `json:"maxSessionsLabel"`
				LoginTypes       []struct{ Value, Label string } `json:"loginTypes"`
			} `json:"presentation"`
			Actions struct{ Update, Status, Delete bool } `json:"actions"`
		}
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	row := decoded.List[0]
	if row.Presentation.MaxSessionsLabel != "不限" || len(row.Presentation.LoginTypes) != 2 {
		t.Fatalf("missing server presentation: %s", raw)
	}
	if !row.Actions.Update || !row.Actions.Status || row.Actions.Delete {
		t.Fatalf("builtin action contract: %s", raw)
	}
}

func TestPlatformPresentationLocaleAndRawFallback(t *testing.T) {
	ctx := i18n.WithLocale(context.Background(), i18n.EnUS)
	for _, test := range []struct {
		sessions int16
		label    string
	}{{0, "Unlimited"}, {1, "Single session"}, {4, "Up to 4"}, {-1, "-1"}} {
		value := newPlatformPresentation(ctx, Platform{MaxSessions: test.sessions}, []LoginType{LoginTypeEmail, "future"})
		if value.MaxSessionsLabel != test.label || value.LoginTypes[0].Label != "Email code" || value.LoginTypes[1].Label != "future" {
			t.Fatalf("presentation: %#v", value)
		}
	}
	if adminOptions(ctx).NameMaxBytes != MaximumNameBytes {
		t.Fatal("name byte constraint drift")
	}
}
