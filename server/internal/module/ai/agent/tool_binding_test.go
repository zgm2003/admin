package agent

import (
	"encoding/json"
	"testing"
)

func TestToolBindingsRejectReservedNamesAndDuplicateIDs(t *testing.T) {
	v := UpdateInput{Name: "A", Capabilities: []string{"text"}, SystemPrompt: "", MultiplierPPM: 1, GenerationParams: json.RawMessage(`{}`), Tools: []ToolBindingInput{{ToolID: 1, ExposedName: "__admin_retired_1", Status: 1, ParameterConstraints: json.RawMessage(`{"maxCalls":1}`)}}}
	if e := v.Validate(); e == nil {
		t.Fatal("reserved tool name accepted")
	}
	v.Tools[0].ExposedName = "a"
	v.Tools = append(v.Tools, ToolBindingInput{ToolID: 1, ExposedName: "b", Status: 1, ParameterConstraints: json.RawMessage(`{"maxCalls":1}`)})
	if e := v.Validate(); e == nil {
		t.Fatal("duplicate tool identity accepted")
	}
}
