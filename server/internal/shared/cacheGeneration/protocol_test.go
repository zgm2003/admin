package cachegeneration

import (
	"strings"
	"testing"
)

func TestScopeValidatesStableCoordinates(t *testing.T) {
	for _, scope := range []Scope{
		{Namespace: "system.setting", ScopeKey: "global"},
		{Namespace: "system.setting", ScopeKey: "test-20260916"},
	} {
		if err := scope.Validate(); err != nil {
			t.Fatalf("scope %+v rejected: %v", scope, err)
		}
	}

	cases := []struct {
		name  string
		scope Scope
	}{
		{"empty namespace", Scope{Namespace: "", ScopeKey: "global"}},
		{"empty scope key", Scope{Namespace: "system.setting", ScopeKey: ""}},
		{"uppercase namespace", Scope{Namespace: "System.Setting", ScopeKey: "global"}},
		{"underscore in namespace", Scope{Namespace: "system_setting", ScopeKey: "global"}},
		{"space in namespace", Scope{Namespace: "system setting", ScopeKey: "global"}},
		{"space in scope key", Scope{Namespace: "system.setting", ScopeKey: "space key"}},
		{"path separator", Scope{Namespace: "system.setting", ScopeKey: "a/b"}},
		{"control character", Scope{Namespace: "system.setting", ScopeKey: "a\nb"}},
		{"overlong namespace", Scope{Namespace: strings.Repeat("a", 129), ScopeKey: "global"}},
		{"overlong scope key", Scope{Namespace: "system.setting", ScopeKey: strings.Repeat("a", 129)}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := test.scope.Validate(); err == nil {
				t.Fatalf("scope %+v was accepted", test.scope)
			}
		})
	}

	if _, err := NewScope("system.setting", "global"); err != nil {
		t.Fatalf("NewScope rejected valid coordinates: %v", err)
	}
	for _, pair := range [][2]string{{"", "global"}, {"system.setting", ""}} {
		if _, err := NewScope(pair[0], pair[1]); err == nil {
			t.Fatalf("NewScope accepted %q/%q", pair[0], pair[1])
		}
	}
}

func TestSnapshotKeyIsDeterministicAndValidated(t *testing.T) {
	scope, err := NewScope("system.setting", "global")
	if err != nil {
		t.Fatal(err)
	}
	if got := StateKey(scope); got != "config-cache:state:v1:system.setting:global" {
		t.Fatalf("state key = %q", got)
	}
	snapshot, err := SnapshotKey(scope, 12, "setting:auth.captcha.ttl_minutes")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot != "config-cache:snapshot:v1:system.setting:global:12:setting:auth.captcha.ttl_minutes" {
		t.Fatalf("snapshot key = %q", snapshot)
	}
	fill, err := FillKey(scope, 12, "brand")
	if err != nil {
		t.Fatal(err)
	}
	if fill != "config-cache:fill:v1:system.setting:global:12:brand" {
		t.Fatalf("fill key = %q", fill)
	}

	cases := []struct {
		name       string
		generation int64
		variant    string
	}{
		{"zero generation", 0, "brand"},
		{"negative generation", -1, "brand"},
		{"empty variant", 12, ""},
		{"space in variant", 12, "setting:a b"},
		{"path separator in variant", 12, "setting:a/b"},
		{"control character in variant", 12, "setting:a\nb"},
		{"overlong variant", 12, strings.Repeat("a", 193)},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := SnapshotKey(scope, test.generation, test.variant); err == nil {
				t.Fatalf("SnapshotKey accepted %d/%q", test.generation, test.variant)
			}
		})
	}
}

func TestDecodeStateAcceptsFrozenStatePayloads(t *testing.T) {
	ready, err := decodeState(`{"schemaVersion":1,"state":"ready","generation":12}`)
	if err != nil {
		t.Fatalf("decode ready state: %v", err)
	}
	if ready.State != StateReady || ready.Generation != 12 || ready.BaseGeneration != 0 || ready.MutationToken != "" {
		t.Fatalf("ready state = %+v", ready)
	}

	invalidating, err := decodeState(`{"schemaVersion":1,"state":"invalidating","baseGeneration":12,"mutationToken":"base64url-token"}`)
	if err != nil {
		t.Fatalf("decode invalidating state: %v", err)
	}
	if invalidating.State != StateInvalidating || invalidating.Generation != 0 || invalidating.BaseGeneration != 12 || invalidating.MutationToken != "base64url-token" {
		t.Fatalf("invalidating state = %+v", invalidating)
	}
}

func TestDecodeStateRejectsUnknownDuplicateAndTrailingFields(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"unknown field", `{"schemaVersion":1,"state":"ready","generation":12,"extra":true}`},
		{"duplicate field", `{"schemaVersion":1,"state":"ready","generation":12,"generation":13}`},
		{"trailing content", `{"schemaVersion":1,"state":"ready","generation":12} {}`},
		{"missing generation", `{"schemaVersion":1,"state":"ready"}`},
		{"zero generation", `{"schemaVersion":1,"state":"ready","generation":0}`},
		{"negative generation", `{"schemaVersion":1,"state":"ready","generation":-1}`},
		{"ready with mutation token", `{"schemaVersion":1,"state":"ready","generation":12,"mutationToken":"token"}`},
		{"empty mutation token", `{"schemaVersion":1,"state":"invalidating","baseGeneration":12,"mutationToken":""}`},
		{"missing mutation token", `{"schemaVersion":1,"state":"invalidating","baseGeneration":12}`},
		{"invalidating with generation", `{"schemaVersion":1,"state":"invalidating","generation":12,"baseGeneration":12,"mutationToken":"token"}`},
		{"zero base generation", `{"schemaVersion":1,"state":"invalidating","baseGeneration":0,"mutationToken":"token"}`},
		{"unknown state", `{"schemaVersion":1,"state":"stale","generation":12}`},
		{"wrong schema version", `{"schemaVersion":2,"state":"ready","generation":12}`},
		{"not an object", `[]`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeState(test.value); err == nil {
				t.Fatalf("decodeState accepted %s", test.value)
			}
		})
	}
}
