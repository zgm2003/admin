package ratelimitpolicy

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPolicyResponseDoesNotDuplicatePlatformScope(t *testing.T) {
	encoded, err := json.Marshal(newPolicyResponse(Model{
		PlatformID: 1, Key: "business_email_minute", Mode: "business", Dimension: "platform_email",
		Limit: 1, WindowSeconds: 60, UpdatedAt: time.Now().UTC(),
	}))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if _, found := fields["platformId"]; found {
		t.Fatalf("policy response duplicated platform scope: %s", encoded)
	}
	for _, key := range []string{"key", "mode", "dimension", "limit", "windowSeconds", "updatedAt"} {
		if _, found := fields[key]; !found {
			t.Fatalf("policy response missing %q: %s", key, encoded)
		}
	}
}

func TestPublicResponsesDoNotExposeCacheOrOptimisticVersions(t *testing.T) {
	now := time.Now().UTC()
	platform, err := json.Marshal(PlatformResponse{
		PlatformID: 1, PlatformCode: "admin", PlatformName: "Admin",
		Policies: []PolicyResponse{newPolicyResponse(Model{
			PlatformID: 1, Key: "business_email_minute", Mode: "business", Dimension: "platform_email",
			Limit: 1, WindowSeconds: 60, UpdatedAt: now,
		})},
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := json.Marshal(UpdateResponse{PlatformID: 1, Policy: newPolicyResponse(Model{
		PlatformID: 1, Key: "business_email_minute", Mode: "business", Dimension: "platform_email",
		Limit: 2, WindowSeconds: 120, UpdatedAt: now,
	})})
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{platform, updated} {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		if _, found := fields["version"]; found {
			t.Fatalf("response exposed version: %s", raw)
		}
		if _, found := fields["revision"]; found {
			t.Fatalf("response exposed revision: %s", raw)
		}
	}
}
