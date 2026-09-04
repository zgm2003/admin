package mail

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestLogResponseFormatsTimesAsUTC(t *testing.T) {
	local := time.Date(2026, 9, 3, 14, 33, 44, 248500000, time.FixedZone("CST", 8*60*60))
	value := logResponseFromModel(Log{
		ID: 1, PlatformID: 2, Scene: SceneLogin, ToEmail: "user@example.com", Subject: "code",
		Status: StatusSent, SentAt: &local, CreatedAt: local, UpdatedAt: local,
	})

	if value.SentAt == nil || *value.SentAt != "2026-09-03T06:33:44.2485Z" {
		t.Fatalf("sentAt = %v, want UTC RFC3339Nano", value.SentAt)
	}
	if value.CreatedAt != "2026-09-03T06:33:44.2485Z" || value.UpdatedAt != "2026-09-03T06:33:44.2485Z" {
		t.Fatalf("timestamps = %q / %q, want UTC RFC3339Nano", value.CreatedAt, value.UpdatedAt)
	}
}

func TestLogResponseKeepsUnsentTimeNull(t *testing.T) {
	value := logResponseFromModel(Log{})
	if value.SentAt != nil {
		t.Fatalf("sentAt = %v, want nil", value.SentAt)
	}
}

func TestLogDetailResponseFormatsExpirationAsUTC(t *testing.T) {
	local := time.Date(2026, 9, 4, 14, 33, 44, 248500000, time.FixedZone("CST", 8*60*60))
	value := logDetailResponseFromModel(LogDetail{
		Log: Log{}, VerificationCode: "123456", VerificationExpiresAt: &local,
	})
	if value.VerificationExpiresAt == nil || *value.VerificationExpiresAt != "2026-09-04T06:33:44.2485Z" {
		t.Fatalf("verificationExpiresAt = %v, want UTC RFC3339Nano", value.VerificationExpiresAt)
	}
}

func TestLogResponsesSerializeUTCStringsAndNulls(t *testing.T) {
	local := time.Date(2026, 9, 3, 14, 33, 44, 0, time.FixedZone("CST", 8*60*60))
	encoded, err := json.Marshal(logListResponseFromModels([]Log{{
		ID: 1, SentAt: nil, CreatedAt: local, UpdatedAt: local,
	}}, 1, 1, 20))
	if err != nil {
		t.Fatalf("marshal log response: %v", err)
	}
	value := string(encoded)
	for _, fragment := range []string{
		`"sentAt":null`,
		`"createdAt":"2026-09-03T06:33:44Z"`,
		`"updatedAt":"2026-09-03T06:33:44Z"`,
		`"total":1`,
		`"page":1`,
		`"pageSize":20`,
	} {
		if !strings.Contains(value, fragment) {
			t.Fatalf("response JSON = %s, want fragment %s", value, fragment)
		}
	}
}
