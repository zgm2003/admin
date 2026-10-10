package account

import (
	"context"
	"testing"

	"admin/server/internal/module/permission/role"
	"admin/server/internal/shared/yesno"
)

func TestAccountActionPolicyMatrix(t *testing.T) {
	ctx := context.Background()
	ordinary := ListItem{ID: 2, IsEnabled: yesno.Yes, Roles: []RoleSummary{{ID: 10, Code: role.CodeRegisteredUser}}}
	actions, labels := accountActions(ctx, 1, actorFacts{ActorActive: true}, ordinary)
	if !actions.Update || !actions.Status || !actions.Delete || !actions.Authorize || labels.Update == "" {
		t.Fatalf("ordinary: %+v %+v", actions, labels)
	}
	ordinary.ID = 1
	actions, _ = accountActions(ctx, 1, actorFacts{ActorActive: true}, ordinary)
	if !actions.Update || actions.Status || actions.Delete || actions.Authorize {
		t.Fatalf("self: %+v", actions)
	}
	ordinary.ID = 2
	ordinary.Roles = []RoleSummary{{ID: 11, Code: role.CodeSuperAdmin}}
	actions, _ = accountActions(ctx, 1, actorFacts{ActorActive: true}, ordinary)
	if actions.Update || actions.Status || actions.Delete || actions.Authorize {
		t.Fatalf("protected: %+v", actions)
	}
	actions, _ = accountActions(ctx, 1, actorFacts{ActorActive: true, ActorSuper: true, EffectiveSuperAdmins: 1}, ordinary)
	if !actions.Update || actions.Status || actions.Delete || !actions.Authorize {
		t.Fatalf("last admin: %+v", actions)
	}
	options := assignmentOptions(1, actorFacts{ActorActive: true, ActorSuper: true, EffectiveSuperAdmins: 1}, Summary{ID: 2, IsEnabled: yesno.Yes}, ordinary.Roles, []int64{11})
	if len(options) != 1 || options[0].Selectable || !options[0].Locked {
		t.Fatalf("locked last admin: %+v", options)
	}
}
