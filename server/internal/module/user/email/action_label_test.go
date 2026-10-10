package email

import (
	"admin/server/internal/shared/i18n"
	"context"
	"encoding/json"
	"testing"
)

func TestIdentityChangeLogReturnsLocalizedActionLabel(t *testing.T) {
	result := changeLogListResponse(i18n.WithLocale(context.Background(), i18n.EnUS), ChangeLogPage{List: []ChangeLogItem{{Action: ActionBind}}})
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		List []struct{ ActionLabel string }
	}
	if err = json.Unmarshal(data, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.List) != 1 || response.List[0].ActionLabel != "Bind" {
		t.Fatalf("response=%s", data)
	}
}
