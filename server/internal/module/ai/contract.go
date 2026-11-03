// Package ai contains the Admin AI domain contracts shared by its vertical slices.
package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type Operation string

const (
	OperationText            Operation = "text.generate"
	OperationImage           Operation = "image.generate"
	OperationImageEdit       Operation = "image.edit"
	OperationAudio           Operation = "audio.speech"
	OperationAudioTranscribe Operation = "audio.transcribe"
	OperationVideo           Operation = "video.generate"
)

func (v Operation) Valid() bool {
	switch v {
	case OperationText, OperationImage, OperationImageEdit, OperationAudio, OperationAudioTranscribe, OperationVideo:
		return true
	default:
		return false
	}
}

// TextProtocol is the persisted text protocol choice on an Agent binding.
// Media operations use dedicated endpoints instead of this enum.
type TextProtocol int16

const (
	TextProtocolResponses       TextProtocol = 1
	TextProtocolChatCompletions TextProtocol = 2
)

func (v TextProtocol) Valid() bool {
	return v == TextProtocolResponses || v == TextProtocolChatCompletions
}

type RunStatus int16

const (
	RunStatusQueued          RunStatus = 0
	RunStatusRunning         RunStatus = 1
	RunStatusWaitingUpstream RunStatus = 2
	RunStatusSucceeded       RunStatus = 3
	RunStatusFailed          RunStatus = 4
	RunStatusCanceled        RunStatus = 5
)

func (v RunStatus) Valid() bool {
	return v >= RunStatusQueued && v <= RunStatusCanceled
}

type PriceStatus int16

const (
	PriceStatusPreview   PriceStatus = 0
	PriceStatusPublished PriceStatus = 1
)

func (v PriceStatus) Valid() bool {
	return v == PriceStatusPreview || v == PriceStatusPublished
}

// DecodeJSONObject validates that raw is exactly one non-null JSON object.
// It keeps the raw bytes so callers can hash the normalized payload at the boundary.
func DecodeJSONObject(raw json.RawMessage) (json.RawMessage, error) {
	return decodeJSONShape(raw, '{', '}')
}

// DecodeJSONArray validates that raw is exactly one non-null JSON array.
func DecodeJSONArray(raw json.RawMessage) (json.RawMessage, error) {
	return decodeJSONShape(raw, '[', ']')
}

func decodeJSONShape(raw json.RawMessage, first, last byte) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) < 2 || trimmed[0] != first || trimmed[len(trimmed)-1] != last {
		return nil, errors.New("AI JSON value has the wrong shape")
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode AI JSON value: %w", err)
	}
	if value == nil {
		return nil, errors.New("AI JSON value cannot be null")
	}
	switch first {
	case '{':
		if _, ok := value.(map[string]any); !ok {
			return nil, errors.New("AI JSON value must be an object")
		}
	case '[':
		if _, ok := value.([]any); !ok {
			return nil, errors.New("AI JSON value must be an array")
		}
	default:
		return nil, errors.New("AI JSON shape is unsupported")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("AI JSON value must contain one document")
		}
		return nil, fmt.Errorf("decode trailing AI JSON value: %w", err)
	}
	return append(json.RawMessage(nil), trimmed...), nil
}
