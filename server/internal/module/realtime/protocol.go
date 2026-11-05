package realtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
)

type TargetType string

const (
	TargetUser     TargetType = "user"
	TargetPlatform TargetType = "platform"

	DurabilityDurable = "durable"

	EventRealtimeConnected        = "realtime.connected.v1"
	EventRealtimePong             = "realtime.pong.v1"
	EventRealtimeResumed          = "realtime.resumed.v1"
	EventRealtimeResyncRequired   = "realtime.resyncRequired.v1"
	EventRealtimeError            = "realtime.error.v1"
	EventNotificationCreated      = "notification.created.v1"
	EventNotificationStateChanged = "notification.stateChanged.v1"

	ClientPing   = "realtime.ping.v1"
	ClientResume = "realtime.resume.v1"
)

type EventInput struct {
	EventID           string
	DedupKey          string
	PlatformID        int64
	EventType         string
	TargetType        TargetType
	TargetUserID      *int64
	AudienceMaxUserID *int64
	Payload           json.RawMessage
	OccurredAt        time.Time
}

type Envelope struct {
	EventID    string          `json:"eventId"`
	Type       string          `json:"type"`
	RequestID  *string         `json:"requestId,omitempty"`
	Sequence   int64           `json:"sequence"`
	OccurredAt time.Time       `json:"occurredAt"`
	Durability string          `json:"durability"`
	Data       json.RawMessage `json:"data"`
}

type TicketSubject struct {
	SchemaVersion   int       `json:"schemaVersion"`
	PlatformID      int64     `json:"platformId"`
	PlatformCode    string    `json:"platformCode"`
	UserID          int64     `json:"userId"`
	SessionID       int64     `json:"sessionId"`
	SessionVersion  int64     `json:"sessionVersion"`
	AccessExpiresAt time.Time `json:"accessExpiresAt"`
	IssuedAt        time.Time `json:"issuedAt"`
}

type SessionValidator interface {
	ValidateRealtimeSession(context.Context, string, int64, int64, int64) error
}

type ClientFrame struct {
	Type          string
	RequestID     string
	AfterSequence *int64
}

type PubSubPayload struct {
	SchemaVersion     int        `json:"schemaVersion"`
	PlatformID        int64      `json:"platformId"`
	TargetType        TargetType `json:"targetType"`
	TargetUserID      *int64     `json:"targetUserId,omitempty"`
	AudienceMaxUserID *int64     `json:"audienceMaxUserId,omitempty"`
	Envelope          Envelope   `json:"envelope"`
}

func DecodeEnvelope(raw []byte) (Envelope, error) {
	var envelope Envelope
	if err := strictDecode(raw, &envelope); err != nil {
		return Envelope{}, err
	}
	if err := validateEnvelope(envelope); err != nil {
		return Envelope{}, err
	}
	return envelope, nil
}

func EncodeEnvelope(envelope Envelope) ([]byte, error) {
	if err := validateEnvelope(envelope); err != nil {
		return nil, err
	}
	return json.Marshal(envelope)
}

func DecodeClientFrame(raw []byte) (ClientFrame, error) {
	var wire struct {
		Type      string          `json:"type"`
		RequestID string          `json:"requestId"`
		Data      json.RawMessage `json:"data,omitempty"`
	}
	if err := strictDecode(raw, &wire); err != nil {
		return ClientFrame{}, err
	}
	if wire.RequestID == "" || len(wire.RequestID) > 128 {
		return ClientFrame{}, errors.New("requestId must contain 1..128 bytes")
	}
	switch wire.Type {
	case ClientPing:
		if len(wire.Data) != 0 {
			return ClientFrame{}, errors.New("ping data is not allowed")
		}
		return ClientFrame{Type: wire.Type, RequestID: wire.RequestID}, nil
	case ClientResume:
		if len(wire.Data) == 0 {
			return ClientFrame{}, errors.New("resume data is required")
		}
		var data struct {
			AfterSequence int64 `json:"afterSequence"`
		}
		if err := strictDecode(wire.Data, &data); err != nil {
			return ClientFrame{}, fmt.Errorf("decode resume data: %w", err)
		}
		if data.AfterSequence < 0 {
			return ClientFrame{}, errors.New("afterSequence must be non-negative")
		}
		return ClientFrame{Type: wire.Type, RequestID: wire.RequestID, AfterSequence: &data.AfterSequence}, nil
	default:
		return ClientFrame{}, fmt.Errorf("unsupported client frame type %q", wire.Type)
	}
}

func EncodePubSubPayload(payload PubSubPayload) ([]byte, error) {
	if err := validatePubSubPayload(payload); err != nil {
		return nil, err
	}
	return json.Marshal(payload)
}

func DecodePubSubPayload(raw []byte) (PubSubPayload, error) {
	var payload PubSubPayload
	if err := strictDecode(raw, &payload); err != nil {
		return PubSubPayload{}, err
	}
	if err := validatePubSubPayload(payload); err != nil {
		return PubSubPayload{}, err
	}
	return payload, nil
}

func validateEnvelope(envelope Envelope) error {
	if _, err := uuid.Parse(envelope.EventID); err != nil {
		return errors.New("eventId must be a UUID")
	}
	if !validServerEventType(envelope.Type) {
		return fmt.Errorf("unsupported server event type %q", envelope.Type)
	}
	if envelope.RequestID != nil && (len(*envelope.RequestID) == 0 || len(*envelope.RequestID) > 128) {
		return errors.New("requestId must contain 1..128 bytes")
	}
	if envelope.Sequence <= 0 {
		return errors.New("sequence must be positive")
	}
	if envelope.OccurredAt.IsZero() {
		return errors.New("occurredAt is required")
	}
	if envelope.Durability != DurabilityDurable {
		return errors.New("durability must be durable")
	}
	if err := validateJSONObject(envelope.Data, "data"); err != nil {
		return err
	}
	return nil
}

func validatePubSubPayload(payload PubSubPayload) error {
	if payload.SchemaVersion != 1 || payload.PlatformID <= 0 {
		return errors.New("invalid Pub/Sub metadata")
	}
	switch payload.TargetType {
	case TargetUser:
		if payload.TargetUserID == nil || *payload.TargetUserID <= 0 || payload.AudienceMaxUserID != nil {
			return errors.New("invalid user target")
		}
	case TargetPlatform:
		if payload.TargetUserID != nil || payload.AudienceMaxUserID == nil || *payload.AudienceMaxUserID < 0 {
			return errors.New("invalid platform target")
		}
	default:
		return errors.New("invalid targetType")
	}
	return validateEnvelope(payload.Envelope)
}

func ValidateEventInput(input EventInput) error {
	if _, err := uuid.Parse(input.EventID); err != nil {
		return errors.New("eventId must be a UUID")
	}
	if input.PlatformID <= 0 || input.DedupKey == "" || len(input.DedupKey) > 256 || !validServerEventType(input.EventType) || input.OccurredAt.IsZero() {
		return errors.New("invalid event metadata")
	}
	if err := validateJSONObject(input.Payload, "payload"); err != nil {
		return err
	}
	switch input.TargetType {
	case TargetUser:
		if input.TargetUserID == nil || *input.TargetUserID <= 0 || input.AudienceMaxUserID != nil {
			return errors.New("invalid user target")
		}
	case TargetPlatform:
		if input.TargetUserID != nil || input.AudienceMaxUserID == nil || *input.AudienceMaxUserID < 0 {
			return errors.New("invalid platform target")
		}
	default:
		return errors.New("invalid target type")
	}
	return nil
}

func validServerEventType(value string) bool {
	switch value {
	case EventRealtimeConnected, EventRealtimePong, EventRealtimeResumed, EventRealtimeResyncRequired,
		EventRealtimeError, EventNotificationCreated, EventNotificationStateChanged:
		return true
	default:
		return false
	}
}

func validateJSONObject(raw json.RawMessage, name string) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return fmt.Errorf("%s must be a JSON object", name)
	}
	var value map[string]json.RawMessage
	if err := strictDecode(trimmed, &value); err != nil {
		return fmt.Errorf("invalid %s: %w", name, err)
	}
	return nil
}

func strictDecode(raw []byte, target any) error {
	if len(raw) > 64*1024 {
		return errors.New("JSON payload exceeds 64 KiB")
	}
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := expectJSONEOF(decoder); err != nil {
		return err
	}
	return nil
}

func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	return expectJSONEOF(decoder)
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("JSON object key must be a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate JSON field %q", key)
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid JSON delimiter")
	}
	_, err = decoder.Token()
	return err
}

func expectJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return err
	}
	return nil
}

func normalizedError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 512 {
		return value[:512]
	}
	return value
}
