package tool

import (
	"context"
	"encoding/json"
	"testing"
)

func TestMutationRequiresRuntimeCoordinator(t *testing.T) {
	s := NewService(nil)
	if e := s.Delete(context.Background(), 1); e == nil {
		t.Fatal("nil runtime accepted")
	}
}
func TestSchemaRejectsUnknownKeywordsAndDeepValues(t *testing.T) {
	for _, raw := range []json.RawMessage{json.RawMessage(`{"type":"object","$ref":"https://example.com/schema"}`), json.RawMessage(`{"type":"object","properties":{"value":{"type":"anything"}}}`), json.RawMessage(`{} {}`)} {
		if e := schemaObject(raw); e == nil {
			t.Fatalf("bad schema accepted %s", raw)
		}
	}
}
