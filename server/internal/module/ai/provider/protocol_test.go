package provider

import "testing"

func TestProtocolValues(t *testing.T) {
	for _, protocol := range []Protocol{ProtocolOpenAI, ProtocolAnthropic, ProtocolGemini, ProtocolOpenAIResponses} {
		if !protocol.Valid() {
			t.Fatalf("protocol %d should be valid", protocol)
		}
	}
	if Protocol(0).Valid() || Protocol(5).Valid() {
		t.Fatal("unexpected protocol value accepted")
	}
}
