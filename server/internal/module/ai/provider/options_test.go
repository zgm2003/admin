package provider

import (
	"context"
	"testing"
)

func TestOptionsIncludeOpenAIResponses(t *testing.T) {
	var service Service
	options, err := service.Options(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(options.Protocols) != 4 {
		t.Fatalf("protocol options = %d, want 4", len(options.Protocols))
	}
	found := false
	for _, item := range options.Protocols {
		if item.Value == ProtocolOpenAIResponses {
			found = true
			if item.Label == "" {
				t.Fatal("OpenAI Responses option has no label")
			}
		}
	}
	if !found {
		t.Fatal("OpenAI Responses option is missing")
	}
}
