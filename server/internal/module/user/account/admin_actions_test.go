package account_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"admin/server/internal/module/user/account"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
)

func TestAccountListReturnsActorOwnedActions(t *testing.T) {
	tx, ctx, roles := openUserTransaction(t)
	ordinary, err := roles.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	actor := createListedUser(t, tx, ctx, "actor"+suffix, "actor"+suffix+"@example.com", yesno.Yes, time.Now(), ordinary.ID)
	target := createListedUser(t, tx, ctx, "target"+suffix, "target"+suffix+"@example.com", yesno.Yes, time.Now(), ordinary.ID)
	service := newUserTestService(t, account.NewRepository(tx))
	query := account.ListQuery{Page: 1, PageSize: 20, Keyword: target.Username}
	raw, _ := json.Marshal(map[string]int64{"ActorUserID": actor.ID})
	if err = json.Unmarshal(raw, &query); err != nil {
		t.Fatal(err)
	}
	result, err := service.List(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(result.List)
	var facts []struct {
		Actions      struct{ Update, Status, Delete, Authorize bool }
		ActionLabels struct{ Update string }
	}
	if err = json.Unmarshal(data, &facts); err != nil {
		t.Fatal(err)
	}
	if len(facts) != 1 || !facts[0].Actions.Update || !facts[0].Actions.Status || !facts[0].Actions.Delete || !facts[0].Actions.Authorize || facts[0].ActionLabels.Update == "" {
		t.Fatalf("actions missing: %s", data)
	}
	superRole, err := roles.FindByCode(ctx, "super_admin")
	if err != nil {
		t.Fatal(err)
	}
	superActor := createListedUser(t, tx, ctx, "superactor"+suffix, "superactor"+suffix+"@example.com", yesno.Yes, time.Now(), superRole.ID)
	superTarget := createListedUser(t, tx, ctx, "supertarget"+suffix, "supertarget"+suffix+"@example.com", yesno.Yes, time.Now(), superRole.ID)
	result, err = service.List(ctx, account.ListQuery{ActorUserID: superActor.ID, Page: 1, PageSize: 20, Keyword: superTarget.Username})
	if err != nil || len(result.List) != 1 || !result.List[0].Actions.Update || !result.List[0].Actions.Authorize || !result.List[0].Actions.Status {
		t.Fatalf("super actor: %+v %v", result, err)
	}
	result, err = service.List(ctx, account.ListQuery{ActorUserID: actor.ID, Page: 1, PageSize: 20, Keyword: superTarget.Username})
	if err != nil || len(result.List) != 1 || result.List[0].Actions.Update || result.List[0].Actions.Authorize || result.List[0].Actions.Status {
		t.Fatalf("ordinary actor: %+v %v", result, err)
	}
	assignment, err := service.Roles(ctx, actor.ID, superTarget.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, option := range assignment.Roles {
		if option.Selectable || (option.ID == superRole.ID && !option.Locked) {
			t.Fatalf("protected assignment: %+v", assignment)
		}
	}
	assignment, err = service.Roles(ctx, actor.ID, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, option := range assignment.Roles {
		if option.ID == superRole.ID && (option.Selectable || option.Locked) {
			t.Fatalf("ordinary assignment: %+v", assignment)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = service.List(canceled, account.ListQuery{ActorUserID: actor.ID, Page: 1, PageSize: 20}); appErrorCodeForUser(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("canceled list: %v", err)
	}

}
