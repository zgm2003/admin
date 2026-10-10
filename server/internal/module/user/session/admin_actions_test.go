package session

import (
	"encoding/json"
	"testing"
)

func TestAdminSessionRevokeActionIsReturnedByBackend(t *testing.T) {
	for _, test := range []struct {
		status SessionStatus
		id     int64
		want   bool
	}{
		{SessionStatus("active"), 1, false}, {SessionStatus("active"), 2, true},
		{SessionStatus("expired"), 2, false}, {SessionStatus("revoked"), 2, false},
	} {
		value := newSessionAdminListResponse([]AdminSession{{ID: test.id, Status: test.status}}, 1, AdminSessionQuery{Page: 1, PageSize: 20}, 1)
		data, err := json.Marshal(value.List[0])
		if err != nil {
			t.Fatal(err)
		}
		var row map[string]json.RawMessage
		if err := json.Unmarshal(data, &row); err != nil {
			t.Fatal(err)
		}
		var actions map[string]bool
		if err := json.Unmarshal(row["actions"], &actions); err != nil {
			t.Fatalf("session must return actions: %v", err)
		}
		if actions["revoke"] != test.want {
			t.Fatalf("status=%s id=%d revoke=%v", test.status, test.id, actions["revoke"])
		}
	}
}
