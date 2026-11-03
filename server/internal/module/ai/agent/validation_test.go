package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestUpdateValidationRejectsUnboundedAndAmbiguousRelations(t *testing.T) {
	base := UpdateInput{Name: "A", Capabilities: []string{"text"}, SystemPrompt: "", MultiplierPPM: 1000000, GenerationParams: json.RawMessage(`{}`)}
	for _, raw := range []json.RawMessage{json.RawMessage(`{} {}`), json.RawMessage(strings.Repeat(`{"a":`, 18) + `{}` + strings.Repeat(`}`, 18)), json.RawMessage(`{"x":"` + strings.Repeat("x", 65536) + `"}`)} {
		base.GenerationParams = raw
		if e := base.Validate(); e == nil {
			t.Fatal("unbounded params accepted")
		}
	}
	base.GenerationParams = json.RawMessage(`{}`)
	base.Bindings = []BindingInput{{ProviderModelID: 1, Status: 1, TextProtocol: 1, Parameters: json.RawMessage(`{}`)}, {ProviderModelID: 1, Status: 1, TextProtocol: 1, Parameters: json.RawMessage(`{}`)}}
	if e := base.Validate(); e == nil {
		t.Fatal("duplicate bindings accepted")
	}
	base.Bindings = nil
	base.Tools = []ToolBindingInput{{ToolID: 1, ExposedName: "run shell", Status: 1, ParameterConstraints: json.RawMessage(`{"maxCalls":0}`)}}
	if e := base.Validate(); e == nil {
		t.Fatal("invalid tool constraints accepted")
	}
}
func TestMutationRequiresRuntimeCoordinator(t *testing.T) {
	s := NewService(nil, nil)
	if e := s.SwitchBinding(context.Background(), 1, 1); e == nil {
		t.Fatal("nil runtime accepted")
	}
}
