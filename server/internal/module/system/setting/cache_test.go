package setting

import (
	"errors"
	"strings"
	"testing"
	"time"

	"admin/server/internal/shared/yesno"
)

func TestCacheRecordPayloadRoundTripsWithStrictCoordinates(t *testing.T) {
	now := time.Now().UTC()
	row := Record{
		ID: 7, Key: "auth.captcha.ttl_minutes", Value: "2", ValueType: ValueTypeNumber, Description: "",
		IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes, CreatedAt: now, UpdatedAt: now,
	}
	payload, err := encodeRecordPayload(12, row)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeRecordPayload(payload)
	if err != nil {
		t.Fatalf("decodeRecordPayload() error = %v", err)
	}
	if decoded.Generation != 12 || decoded.Key != row.Key || decoded.Value != row.Value || decoded.ValueType != ValueTypeNumber || decoded.IsEnabled != yesno.Yes {
		t.Fatalf("decoded payload = %+v", decoded)
	}
	if decoded.record().ID != 7 || !decoded.record().CreatedAt.Equal(now) {
		t.Fatalf("decoded record = %+v", decoded.record())
	}
}

func TestCacheRecordPayloadRejectsUnknownDuplicateAndInvalidFields(t *testing.T) {
	valid := `{"schemaVersion":1,"generation":12,"id":7,"key":"auth.captcha.ttl_minutes","value":"2","valueType":2,"description":"","isEnabled":1,"isBuiltin":1,"createdAt":"2026-09-16T00:00:00Z","updatedAt":"2026-09-16T00:00:00Z"}`
	if _, err := decodeRecordPayload(valid); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}

	cases := []struct {
		name    string
		payload string
	}{
		{"unknown field", strings.Replace(valid, `"id":7`, `"id":7,"extra":true`, 1)},
		{"duplicate field", strings.Replace(valid, `"value":"2"`, `"value":"2","value":"3"`, 1)},
		{"trailing content", valid + ` {}`},
		{"wrong schema version", strings.Replace(valid, `"schemaVersion":1`, `"schemaVersion":2`, 1)},
		{"zero generation", strings.Replace(valid, `"generation":12`, `"generation":0`, 1)},
		{"empty key", strings.Replace(valid, `"key":"auth.captcha.ttl_minutes"`, `"key":""`, 1)},
		{"invalid value type", strings.Replace(valid, `"valueType":2`, `"valueType":9`, 1)},
		{"invalid enabled flag", strings.Replace(valid, `"isEnabled":1`, `"isEnabled":5`, 1)},
		{"missing field", strings.Replace(valid, `"description":"",`, ``, 1)},
		{"not an object", `[]`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeRecordPayload(test.payload); !errors.Is(err, ErrSnapshotCorrupt) {
				t.Fatalf("decodeRecordPayload(%s) error = %v want ErrSnapshotCorrupt", test.payload, err)
			}
		})
	}
}

func TestCacheBrandPayloadRejectsUnknownAndInvalidFields(t *testing.T) {
	valid := `{"schemaVersion":1,"generation":12,"titleZhCN":"智澜","titleEnUS":"ZHILAN","defaultAvatar":""}`
	if _, err := decodeBrandPayload(valid); err != nil {
		t.Fatalf("valid brand payload rejected: %v", err)
	}

	cases := []struct {
		name    string
		payload string
	}{
		{"zero generation", strings.Replace(valid, `"generation":12`, `"generation":0`, 1)},
		{"unknown field", strings.Replace(valid, `"defaultAvatar":""`, `"defaultAvatar":"","extra":1`, 1)},
		{"trailing content", valid + ` {}`},
		{"not an object", `not-json`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeBrandPayload(test.payload); !errors.Is(err, ErrSnapshotCorrupt) {
				t.Fatalf("decodeBrandPayload(%s) error = %v want ErrSnapshotCorrupt", test.payload, err)
			}
		})
	}
}
