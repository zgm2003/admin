package sms

import "testing"

func TestDecodeReadinessRejectsDuplicateFields(t *testing.T) {
	raw := `{"schemaVersion":1,"ready":true,"ready":false,"ttlMinutes":0}`
	if _, err := decodeReadiness(raw); err == nil {
		t.Fatal("readiness accepted a duplicate ready field")
	}
}

func TestDecodeReadinessRejectsMalformedShapes(t *testing.T) {
	for _, raw := range []string{
		`{}`,
		`{"schemaVersion":1,"ready":true,"ttlMinutes":0}`,
		`{"schemaVersion":1,"ready":true,"ttlMinutes":61}`,
		`{"schemaVersion":1,"ready":false,"ttlMinutes":5}`,
		`{"schemaVersion":1,"ready":true,"ttlMinutes":5,"extra":true}`,
		`{"schemaVersion":1,"ready":true,"ttlMinutes":5}{}`,
	} {
		if _, err := decodeReadiness(raw); err == nil {
			t.Fatalf("readiness accepted malformed payload: %s", raw)
		}
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
