package tool

import (
	"encoding/json"
	"testing"
)

func TestVersionValidationRejectsUnsafeHTTPAndArbitraryBuiltin(t *testing.T) {
	base := VersionInput{Type: TypeHTTP, Description: "http", InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object"}`), TimeoutMS: 1000, OutputLimitBytes: 65536}
	for _, def := range []json.RawMessage{json.RawMessage(`{"url":"http://example.com"}`), json.RawMessage(`{"url":"https://127.0.0.1/x"}`)} {
		base.HTTPDefinition = def
		if err := base.Validate(); err == nil {
			t.Fatalf("unsafe HTTP accepted: %s", def)
		}
	}
	base.Type = TypeBuiltin
	base.HTTPDefinition = nil
	base.BuiltinKey = "exec"
	if err := base.Validate(); err == nil {
		t.Fatal("arbitrary builtin accepted")
	}
}

func TestVersionValidationEnforcesJSONSchemasAndOutputLimit(t *testing.T) {
	input := VersionInput{Type: TypeBuiltin, BuiltinKey: "calculator", Description: "c", InputSchema: json.RawMessage(`[]`), OutputSchema: json.RawMessage(`{"type":"object"}`), TimeoutMS: 100, OutputLimitBytes: 65536}
	if err := input.Validate(); err == nil {
		t.Fatal("array schema accepted")
	}
	input.InputSchema = json.RawMessage(`{"type":"object"}`)
	input.OutputLimitBytes = 65537
	if err := input.Validate(); err == nil {
		t.Fatal("oversized output accepted")
	}
}
