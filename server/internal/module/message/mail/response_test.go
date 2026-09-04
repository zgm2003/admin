package mail

import (
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
