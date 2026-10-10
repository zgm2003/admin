package recipientRule

import (
	"context"
	"testing"

	"admin/server/internal/shared/yesno"
)

func TestPlaintextRuleContractUsesNumericEnumsAndStoresPattern(t *testing.T) {
	if ScopePhone != 0 || ScopePrefix != 1 || ActionDeny != 0 || ActionAllow != 1 {
		t.Fatalf("numeric contract scope=(%v,%v) action=(%v,%v)", ScopePhone, ScopePrefix, ActionDeny, ActionAllow)
	}
	repository := &fakeRepository{}
	service := newMutationService(repository)
	result, err := service.Create(context.Background(), CreateInput{Scope: ScopePhone, Pattern: "15671628271", Action: ActionDeny, Name: "黑名单", IsEnabled: yesno.Yes})
	if err != nil {
		t.Fatal(err)
	}
	if repository.created.Pattern != "+8615671628271" || result.Pattern != "+8615671628271" {
		t.Fatalf("plaintext pattern was not persisted/returned: stored=%+v result=%+v", repository.created, result)
	}
}
