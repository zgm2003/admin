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

func TestSendMessageRequestAcceptsOptionalEditMessage(t *testing.T) {
	requestID := "3c9f1714-51f9-4a1e-9f8f-ffde2b7e1a4e"
	messageID := int64(18)
	input, err := (sendMessageRequest{
		RequestID: &requestID, Content: stringPointer("edited"), EditMessageID: &messageID,
	}).input()
	if err != nil {
		t.Fatal(err)
	}
	if input.EditMessageID == nil || *input.EditMessageID != messageID {
		t.Fatalf("editMessageId = %v", input.EditMessageID)
	}

	messageID = 0
	if _, err = (sendMessageRequest{
		RequestID: &requestID, Content: stringPointer("edited"), EditMessageID: &messageID,
	}).input(); err == nil {
		t.Fatal("invalid editMessageId was accepted")
	}
}

func stringPointer(value string) *string { return &value }
