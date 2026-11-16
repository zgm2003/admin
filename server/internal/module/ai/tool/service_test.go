package tool

import (
	"context"
	"strings"
	"testing"
)

func TestToolDefinitionsUseProviderSafeNames(t *testing.T) {
	service := NewService(nil, nil, nil, nil)
	for code, definition := range service.defs {
		if definition.APIName == "" {
			t.Fatalf("tool %s has no API name", code)
		}
		if strings.Contains(definition.APIName, ".") {
			t.Fatalf("tool API name %q contains a dot", definition.APIName)
		}
		if service.apiNames[definition.APIName] != code {
			t.Fatalf("API name %q does not resolve to %q", definition.APIName, code)
		}
		parameters, ok := definition.Parameters["type"].(string)
		if !ok || parameters != "object" {
			t.Fatalf("tool %s parameters = %#v", code, definition.Parameters)
		}
	}
}

func TestStatusRequest(t *testing.T) {
	enabled := int16(1)
	status, err := (statusRequest{Status: &enabled}).input()
	if err != nil || status != StatusEnabled {
		t.Fatalf("status=%v err=%v", status, err)
	}
	invalid := int16(2)
	if _, err := (statusRequest{Status: &invalid}).input(); err == nil {
		t.Fatal("expected invalid status error")
	}
}

func TestExecuteRejectsUnknownTool(t *testing.T) {
	service := NewService(nil, nil, nil, nil)
	if _, err := service.Execute(context.Background(), "unknown", "{}", Invocation{}); err == nil {
		t.Fatal("expected unknown tool error")
	}
}
