package run

import "testing"

func TestSendMessageRequestAcceptsOptionalAgent(t *testing.T) {
	requestID := "3c9f1714-51f9-4a1e-9f8f-ffde2b7e1a4e"
	agentID := int64(7)
	request := sendMessageRequest{RequestID: &requestID, Content: stringPointer("hello"), AgentID: &agentID}
	input, err := request.input()
	if err != nil {
		t.Fatal(err)
	}
	if input.AgentID == nil || *input.AgentID != agentID {
		t.Fatalf("agentId = %v", input.AgentID)
	}

	request.AgentID = nil
	input, err = request.input()
	if err != nil {
		t.Fatal(err)
	}
	if input.AgentID != nil {
		t.Fatalf("agentId = %v, want nil", input.AgentID)
	}
}

func TestSendMessageRequestRejectsInvalidAgent(t *testing.T) {
	requestID := "3c9f1714-51f9-4a1e-9f8f-ffde2b7e1a4e"
	agentID := int64(0)
	if _, err := (sendMessageRequest{
		RequestID: &requestID, Content: stringPointer("hello"), AgentID: &agentID,
	}).input(); err == nil {
		t.Fatal("invalid agentId was accepted")
	}
}

func stringPointer(value string) *string { return &value }
