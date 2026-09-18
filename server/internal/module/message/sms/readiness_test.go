package sms

import (
	"errors"
	"strings"
	"testing"
)

func TestDecodeReadinessRejectsDuplicateFields(t *testing.T) {
	raw := `{"schemaVersion":1,"generation":1,"ready":true,"ready":false,"ttlMinutes":0}`
	if _, _, err := decodeReadiness(raw, 1); !errors.Is(err, ErrReadinessCorrupt) {
		t.Fatal("readiness accepted a duplicate ready field")
	}
}

func TestDecodeReadinessRejectsMalformedShapes(t *testing.T) {
	for _, raw := range []string{
		`{}`,
		`{"schemaVersion":1,"generation":1,"ready":true,"ttlMinutes":0}`,
		`{"schemaVersion":1,"generation":1,"ready":true,"ttlMinutes":61}`,
		`{"schemaVersion":1,"generation":1,"ready":false,"ttlMinutes":5}`,
		`{"schemaVersion":1,"generation":1,"ready":true,"ttlMinutes":5,"extra":true}`,
		`{"schemaVersion":1,"generation":1,"ready":true,"ttlMinutes":5}{}`,
		`{"schemaVersion":1,"generation":2,"ready":true,"ttlMinutes":5}`,
	} {
		if _, _, err := decodeReadiness(raw, 1); !errors.Is(err, ErrReadinessCorrupt) {
			t.Fatalf("readiness accepted malformed payload: %s", raw)
		}
	}
}

func TestReadinessCodecPreservesGenerationAndRejectsUnknownFields(t *testing.T) {
	raw, err := encodeReadiness(7, VerifyCodeReadiness{Ready: true, TTLMinutes: 5})
	if err != nil {
		t.Fatal(err)
	}
	value, found, err := decodeReadiness(raw, 7)
	if err != nil || !found || !value.Ready || value.TTLMinutes != 5 {
		t.Fatalf("decoded readiness = %+v, found=%v, err=%v", value, found, err)
	}
	corrupt := strings.Replace(raw, `{`, `{"extra":true,`, 1)
	if _, _, err := decodeReadiness(corrupt, 7); !errors.Is(err, ErrReadinessCorrupt) {
		t.Fatalf("unknown field error = %v, want ErrReadinessCorrupt", err)
	}
}

func TestValidateReadinessValueRejectsImpossibleComputeResult(t *testing.T) {
	for _, value := range []VerifyCodeReadiness{
		{Ready: true, TTLMinutes: 0},
		{Ready: true, TTLMinutes: 61},
		{Ready: false, TTLMinutes: 5},
	} {
		if err := validateReadinessValue(value); err == nil {
			t.Fatalf("accepted readiness %+v", value)
		}
	}
	if err := validateReadinessValue(VerifyCodeReadiness{Ready: true, TTLMinutes: 5}); err != nil {
		t.Fatal(err)
	}
	if err := validateReadinessValue(VerifyCodeReadiness{}); err != nil {
		t.Fatal(err)
	}
}
