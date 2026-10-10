package role_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"admin/server/internal/module/permission/role"
	"admin/server/internal/shared/yesno"
)

func TestRoleListReturnsBusinessActions(t *testing.T) {
	tx, ctx := openRoleTransaction(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	stored := role.Role{Code: "actions_" + suffix, Name: "Actions " + suffix, IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&stored).Error; err != nil {
		t.Fatal(err)
	}
	result, err := newRoleTestService(t, role.NewRepository(tx)).List(ctx, role.ListQuery{Page: 1, PageSize: 20, Keyword: stored.Code})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(result.List)
	var facts []struct {
		Actions      struct{ Update, Status, SetDefault, Delete, Authorize bool }
		ActionLabels struct{ Update string }
	}
	if err = json.Unmarshal(data, &facts); err != nil {
		t.Fatal(err)
	}
	if len(facts) != 1 || !facts[0].Actions.Update || !facts[0].Actions.Status || !facts[0].Actions.SetDefault || !facts[0].Actions.Delete || !facts[0].Actions.Authorize || facts[0].ActionLabels.Update == "" {
		t.Fatalf("business actions missing: %s", data)
	}
}
