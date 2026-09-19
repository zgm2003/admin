package realtime

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestProtocolDecodesValidEnvelope(t *testing.T) {
	raw := `{"eventId":"2ec9ca86-e265-4551-a15a-05c333326db0","type":"notification.created.v1","requestId":"request-1","sequence":12,"occurredAt":"2026-09-18T12:00:00Z","durability":"durable","data":{"notificationId":9}}`
	envelope, err := DecodeEnvelope([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if envelope.EventID != "2ec9ca86-e265-4551-a15a-05c333326db0" || envelope.Type != EventNotificationCreated || envelope.Sequence != 12 || envelope.RequestID == nil || *envelope.RequestID != "request-1" {
		t.Fatalf("envelope=%+v", envelope)
	}
	if string(envelope.Data) != `{"notificationId":9}` {
		t.Fatalf("data=%s", envelope.Data)
	}
}

func TestEnvelopeRejectsMalformedOrUnsupportedPayload(t *testing.T) {
	valid := `{"eventId":"2ec9ca86-e265-4551-a15a-05c333326db0","type":"notification.created.v1","sequence":12,"occurredAt":"2026-09-18T12:00:00Z","durability":"durable","data":{}}`
	tests := []struct{ name, raw string }{
		{"unknown field", strings.Replace(valid, `"data":{}`, `"extra":1,"data":{}`, 1)},
		{"duplicate field", strings.Replace(valid, `"sequence":12`, `"sequence":12,"sequence":13`, 1)},
		{"trailing JSON", valid + ` {}`},
		{"unknown type", strings.Replace(valid, "notification.created.v1", "notification.unknown.v1", 1)},
		{"invalid UUID", strings.Replace(valid, "2ec9ca86-e265-4551-a15a-05c333326db0", "bad", 1)},
		{"zero sequence", strings.Replace(valid, `"sequence":12`, `"sequence":0`, 1)},
		{"invalid durability", strings.Replace(valid, "durable", "temporary", 1)},
		{"long request id", strings.Replace(valid, `"data":{}`, `"requestId":"`+strings.Repeat("x", 129)+`","data":{}`, 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DecodeEnvelope([]byte(test.raw)); err == nil {
				t.Fatal("payload accepted")
			}
		})
	}
}

func TestClientFrameAcceptsOnlyStrictPingAndResume(t *testing.T) {
	ping, err := DecodeClientFrame([]byte(`{"type":"realtime.ping.v1","requestId":"r-1"}`))
	if err != nil || ping.Type != ClientPing || ping.RequestID != "r-1" {
		t.Fatalf("ping=%+v err=%v", ping, err)
	}
	resume, err := DecodeClientFrame([]byte(`{"type":"realtime.resume.v1","requestId":"r-2","data":{"afterSequence":123}}`))
	if err != nil || resume.Type != ClientResume || resume.AfterSequence == nil || *resume.AfterSequence != 123 {
		t.Fatalf("resume=%+v err=%v", resume, err)
	}

	invalid := []string{
		`{"type":"realtime.ping.v1","requestId":"r","data":{}}`,
		`{"type":"realtime.resume.v1","requestId":"r","data":{"afterSequence":-1}}`,
		`{"type":"realtime.resume.v1","requestId":"r","data":{"afterSequence":1,"extra":true}}`,
		`{"type":"notification.created.v1","requestId":"r"}`,
		`{"type":"realtime.ping.v1","type":"realtime.resume.v1","requestId":"r"}`,
		`{"type":"realtime.ping.v1","requestId":"r"} null`,
	}
	for _, raw := range invalid {
		if _, err := DecodeClientFrame([]byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

func TestPubSubPayloadRequiresMatchingStaticTarget(t *testing.T) {
	envelope := Envelope{EventID: "2ec9ca86-e265-4551-a15a-05c333326db0", Type: EventNotificationStateChanged, Sequence: 5, OccurredAt: time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC), Durability: DurabilityDurable, Data: json.RawMessage(`{"kind":"read","notificationId":1,"readThroughNotificationId":null}`)}
	userID := int64(10)
	raw, err := EncodePubSubPayload(PubSubPayload{SchemaVersion: 1, PlatformID: 1, TargetType: TargetUser, TargetUserID: &userID, Envelope: envelope})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodePubSubPayload(raw)
	if err != nil || decoded.TargetUserID == nil || *decoded.TargetUserID != 10 {
		t.Fatalf("decoded=%+v err=%v", decoded, err)
	}

	bad := strings.Replace(string(raw), `"targetUserId":10`, `"targetUserId":10,"audienceMaxUserId":20`, 1)
	if _, err := DecodePubSubPayload([]byte(bad)); err == nil {
		t.Fatal("accepted mixed target")
	}
}
