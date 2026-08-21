package accessstate

import (
	"reflect"
	"testing"
)

func TestStateKey(t *testing.T) {
	if got := StateKey(7); got != "authz:access-state:7" {
		t.Fatalf("StateKey(7) = %q", got)
	}
}

func TestNormalizeVersionsSortsDeduplicatesAndRejectsConflicts(t *testing.T) {
	versions, err := normalizeVersions([]Version{{UserID: 9, Version: 4}, {UserID: 7, Version: 3}, {UserID: 9, Version: 4}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(versions, []Version{{UserID: 7, Version: 3}, {UserID: 9, Version: 4}}) {
		t.Fatalf("normalized versions = %+v", versions)
	}
	if _, err := normalizeVersions([]Version{{UserID: 7, Version: 3}, {UserID: 7, Version: 4}}); err == nil {
		t.Fatal("conflicting duplicate versions were accepted")
	}
	for _, invalid := range [][]Version{{{UserID: 0, Version: 1}}, {{UserID: 1, Version: 0}}} {
		if _, err := normalizeVersions(invalid); err == nil {
			t.Fatalf("invalid versions were accepted: %+v", invalid)
		}
	}
}
