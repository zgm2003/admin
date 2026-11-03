package conversation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAgentRuntimeProjectionFiltersUnsupportedCapabilities(t *testing.T) {
	v, e := PublicAgentRuntime(AgentRuntime{Capabilities: []string{"text", "image", "video"}, Operations: []string{"text.generate", "image.generate", "video.generate"}, Rates: []AgentRate{{ID: "1", Operation: "text.generate"}}, Parameters: json.RawMessage(`{}`), GenerationParams: json.RawMessage(`{}`), Limits: json.RawMessage(`{}`)})
	if e != nil {
		t.Fatal(e)
	}
	if len(v.Operations) != 1 || v.Operations[0] != "text.generate" {
		t.Fatalf("%+v", v)
	}
	b, _ := json.Marshal(v)
	if strings.Contains(string(b), "prompt") {
		t.Fatal(string(b))
	}
}

func TestCreateConversationRejectsAgentWithoutTextRuntime(t *testing.T) {
	s := NewService(nil)
	s.SetAgentResolver(func(context.Context, int64, time.Time) (AgentRuntime, error) {
		return AgentRuntime{Capabilities: []string{"image"}, Operations: []string{"image.generate"}, Rates: []AgentRate{{ID: "1", Operation: "image.generate"}}}, nil
	})
	if _, e := s.Create(context.Background(), 1, 1, 1, "title"); e == nil {
		t.Fatal("media agent accepted for text conversation")
	}
}
