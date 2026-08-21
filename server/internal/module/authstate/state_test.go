package authstate

import (
	"testing"
)

func TestAuthenticationStateKeys(t *testing.T) {
	if got := UserStateKey(7); got != "auth:user-state:7" {
		t.Fatalf("UserStateKey(7) = %q", got)
	}
	if got := SessionsStateKey("admin", 7); got != "auth:sessions-state:admin:7" {
		t.Fatalf("SessionsStateKey(admin, 7) = %q", got)
	}
}

func TestNormalizeMutationFactsRejectsConflictsAndSortsKeys(t *testing.T) {
	facts, err := normalizeMutationFacts(MutationFacts{
		Users:    []UserFact{{UserID: 9, Generation: "user-nine", IsEnabled: true}, {UserID: 2, Generation: "user-two", IsEnabled: true}},
		Sessions: []SessionsFact{{Platform: "app", UserID: 9, Generation: "app-nine"}, {Platform: "admin", UserID: 2, Generation: "admin-two"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if facts.Users[0].UserID != 2 || facts.Users[1].UserID != 9 {
		t.Fatalf("normalized users = %+v", facts.Users)
	}
	if facts.Sessions[0].UserID != 2 || facts.Sessions[0].Platform != "admin" || facts.Sessions[1].UserID != 9 {
		t.Fatalf("normalized sessions = %+v", facts.Sessions)
	}

	_, err = normalizeMutationFacts(MutationFacts{Users: []UserFact{
		{UserID: 7, Generation: "first", IsEnabled: true},
		{UserID: 7, Generation: "second", IsEnabled: true},
	}})
	if err == nil {
		t.Fatal("conflicting duplicate user facts were accepted")
	}
}

func TestNewGenerationIsCanonicalRandomToken(t *testing.T) {
	first, err := NewGeneration()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewGeneration()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 43 || len(second) != 43 || first == second {
		t.Fatalf("generations = %q, %q", first, second)
	}
}
