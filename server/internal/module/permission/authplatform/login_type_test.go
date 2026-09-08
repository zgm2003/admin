package authplatform

import (
	"reflect"
	"testing"
)

func TestNormalizeLoginTypesUsesCanonicalOrderAndRejectsDuplicates(t *testing.T) {
	got, err := normalizeLoginTypes([]LoginType{LoginTypePassword, LoginTypeEmail, LoginTypePhone})
	if err != nil || !reflect.DeepEqual(got, []LoginType{LoginTypeEmail, LoginTypePhone, LoginTypePassword}) {
		t.Fatalf("normalizeLoginTypes() = %#v, %v", got, err)
	}
	if _, err := normalizeLoginTypes([]LoginType{LoginTypeEmail, LoginTypeEmail}); err == nil {
		t.Fatal("duplicate login type was accepted")
	}
	if _, err := normalizeLoginTypes([]LoginType{LoginTypeEmail, "sms"}); err == nil {
		t.Fatal("invalid login type was accepted")
	}
	if _, err := normalizeLoginTypes(nil); err == nil {
		t.Fatal("empty login type set was accepted")
	}
}

func TestParseLoginTypesRejectsCorruptJSON(t *testing.T) {
	if _, err := parseLoginTypes([]byte(`["email",`)); err == nil {
		t.Fatal("corrupt login types JSON was accepted")
	}
	if _, err := parseLoginTypes(nil); err == nil {
		t.Fatal("missing login types was accepted")
	}
}
