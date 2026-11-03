package ai

import (
	"encoding/json"
	"testing"
)

func TestOperationAndProtocolValidation(t *testing.T) {
	for _, operation := range []Operation{OperationText, OperationImage, OperationImageEdit, OperationAudio, OperationAudioTranscribe, OperationVideo} {
		if !operation.Valid() {
			t.Fatalf("expected operation %s to be valid", operation)
		}
	}
	if Operation("unknown").Valid() {
		t.Fatal("unknown operation accepted")
	}
	for _, protocol := range []TextProtocol{TextProtocolResponses, TextProtocolChatCompletions} {
		if !protocol.Valid() {
			t.Fatalf("expected protocol %d to be valid", protocol)
		}
	}
	if TextProtocol(99).Valid() || TextProtocol(0).Valid() {
		t.Fatal("unknown protocol accepted")
	}
}

func TestRunStatusAndPriceStatusValidation(t *testing.T) {
	for _, status := range []RunStatus{RunStatusQueued, RunStatusRunning, RunStatusWaitingUpstream, RunStatusSucceeded, RunStatusFailed, RunStatusCanceled} {
		if !status.Valid() {
			t.Fatalf("expected run status %d to be valid", status)
		}
	}
	if RunStatus(99).Valid() {
		t.Fatal("unknown run status accepted")
	}
	for _, status := range []PriceStatus{PriceStatusPreview, PriceStatusPublished} {
		if !status.Valid() {
			t.Fatalf("expected price status %d to be valid", status)
		}
	}
	if PriceStatus(99).Valid() {
		t.Fatal("unknown price status accepted")
	}
}

func TestDecodeObjectAndArrayRejectsWrongJSONShape(t *testing.T) {
	if _, err := DecodeJSONObject(json.RawMessage(`{"model":"gpt"}`)); err != nil {
		t.Fatalf("object rejected: %v", err)
	}
	if _, err := DecodeJSONArray(json.RawMessage(`[{"type":"text"}]`)); err != nil {
		t.Fatalf("array rejected: %v", err)
	}
	for _, raw := range []json.RawMessage{json.RawMessage(`null`), json.RawMessage(`[]`), json.RawMessage(`{"a":1}x`)} {
		if _, err := DecodeJSONObject(raw); err == nil {
			t.Fatalf("invalid object accepted: %s", raw)
		}
	}
	if _, err := DecodeJSONArray(json.RawMessage(`{"type":"text"}`)); err == nil {
		t.Fatal("object accepted as array")
	}
}
