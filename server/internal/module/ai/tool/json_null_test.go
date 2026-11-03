package tool

import (
	"encoding/json"
	"testing"
)

func TestBuiltinAcceptsJSONNullHTTPDefinition(t *testing.T) {
	var v VersionInput
	if e := json.Unmarshal([]byte(`{"type":2,"builtinKey":"calculator","description":"c","inputSchema":{"type":"object"},"outputSchema":{"type":"object"},"httpDefinition":null,"timeoutMs":1000,"outputLimitBytes":65536,"hasSideEffect":0,"idempotencyMode":0}`), &v); e != nil {
		t.Fatal(e)
	}
	if e := v.Validate(); e != nil {
		t.Fatal(e)
	}
	if row := versionOf(v, 1); row.HTTPDefinition != nil {
		t.Fatalf("builtin stored HTTP null literal: %s", row.HTTPDefinition)
	}
}
