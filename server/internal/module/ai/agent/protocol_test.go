package agent

import (
	"encoding/json"
	"testing"
)

func TestCreateInputRejectsStatusAndActiveBindingOnUpdateShape(t *testing.T) {
	input := UpdateInput{Name: "A", Description: "", Capabilities: []string{"text"}, SystemPrompt: "p", MultiplierPPM: 1000000, GenerationParams: json.RawMessage(`{}`), Bindings: []BindingInput{{ProviderModelID: 1}}}
	if err := input.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeUpdate(map[string]any{"name": "A", "status": 1, "activeBindingId": "2"}); err == nil {
		t.Fatal("update accepted status/current binding")
	}
}

func TestCreateInputRequiresValidCapabilitiesAndPrompt(t *testing.T) {
	cases := []CreateInput{{Name: "A", Capabilities: nil, SystemPrompt: "p", MultiplierPPM: 1, GenerationParams: json.RawMessage(`{}`)}, {Name: "A", Capabilities: []string{"shell"}, SystemPrompt: "p", MultiplierPPM: 1, GenerationParams: json.RawMessage(`{}`)}, {Name: "A", Capabilities: []string{"text"}, SystemPrompt: string(make([]byte, 65537)), MultiplierPPM: 1, GenerationParams: json.RawMessage(`{}`)}}
	for _, input := range cases {
		if err := input.Validate(); err == nil {
			t.Fatalf("invalid input accepted: %+v", input)
		}
	}
}
