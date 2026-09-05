package mail

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRateLimitRetryWaitHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := waitRateLimitRetry(ctx, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waitRateLimitRetry() error = %v, want context.Canceled", err)
	}
}

func TestRateLimitRetryWaitHonorsDelay(t *testing.T) {
	started := time.Now()
	if err := waitRateLimitRetry(context.Background(), 10*time.Millisecond); err != nil {
		t.Fatalf("waitRateLimitRetry() error = %v", err)
	}
	if elapsed := time.Since(started); elapsed < 8*time.Millisecond {
		t.Fatalf("waitRateLimitRetry() returned after %s, want at least 8ms", elapsed)
	}
}

func TestRateLimitPolicyStoreRejectsMissingDependencies(t *testing.T) {
	store := NewRateLimitPolicyStore(nil, nil)
	if _, err := store.Load(context.Background()); err == nil {
		t.Fatal("Load accepted a missing Redis dependency")
	}
	if _, err := store.Update(context.Background(), RateLimitPolicyInput{
		Key: "business_email_minute", Limit: 1, WindowSeconds: 60,
	}); err == nil {
		t.Fatal("Update accepted missing store dependencies")
	}
}

func TestRateLimitPolicyCatalogIsFixed(t *testing.T) {
	got := FixedRateLimitPolicies()
	if len(got) != 7 {
		t.Fatalf("policy count = %d, want 7", len(got))
	}
	if got[0].Key != "business_email_minute" || got[0].Limit != 1 || got[0].WindowSeconds != 60 {
		t.Fatalf("business email policy = %+v", got[0])
	}
	if got[6].Key != "admin_test_email_10m" || got[6].Limit != 3 || got[6].WindowSeconds != 600 {
		t.Fatalf("admin email policy = %+v", got[6])
	}
}

func TestRateLimitPolicyInputRejectsUnknownKeyAndOutOfRangeValues(t *testing.T) {
	for _, input := range []RateLimitPolicyInput{
		{Key: "custom", Limit: 1, WindowSeconds: 60},
		{Key: "business_email_minute", Limit: 0, WindowSeconds: 60},
		{Key: "business_email_minute", Limit: 100001, WindowSeconds: 60},
		{Key: "business_email_minute", Limit: 1, WindowSeconds: 0},
		{Key: "business_email_minute", Limit: 1, WindowSeconds: 86401},
	} {
		if err := ValidateRateLimitPolicyInput(input); err == nil {
			t.Fatalf("accepted %+v", input)
		}
	}
}

func TestRateLimitSnapshotReadyRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	catalog := RateLimitCatalog{Version: 4, Policies: fixedPoliciesWithTimestamp(now)}

	snapshot, err := snapshotFromCatalog(catalog)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := encodeRateLimitSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeRateLimitSnapshot(raw)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.State != rateLimitPolicyStateReady || decoded.Version != 4 {
		t.Fatalf("decoded snapshot = %+v", decoded)
	}
	roundTrip := catalogFromSnapshot(decoded)
	if roundTrip.Version != 4 || len(roundTrip.Policies) != 7 {
		t.Fatalf("round trip catalog = %+v", roundTrip)
	}
	for _, policy := range roundTrip.Policies {
		spec, ok := fixedRateLimitSpecByKey(policy.Key)
		if !ok || policy.Mode != spec.Mode || policy.Dimension != spec.Dimension {
			t.Fatalf("round trip policy %q lost static metadata: %+v", policy.Key, policy)
		}
	}
}

func TestRateLimitSnapshotRejectsMalformedPayloads(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	catalog := RateLimitCatalog{Version: 1, Policies: fixedPoliciesWithTimestamp(now)}
	ready, err := encodeRateLimitSnapshot(mustSnapshot(t, catalog))
	if err != nil {
		t.Fatal(err)
	}

	invalidating, err := encodeRateLimitSnapshot(RateLimitSnapshot{
		SchemaVersion: rateLimitPolicySchemaVersion,
		State:         rateLimitPolicyStateInvalidating,
		Version:       1,
		MutationToken: stringPointer("token"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeRateLimitSnapshot(invalidating); err != nil {
		t.Fatalf("valid invalidating snapshot rejected: %v", err)
	}

	reject := []string{
		`{"schemaVersion":1,"state":"ready","version":1,"policies":{}}`,
		`{"schemaVersion":1,"state":"ready","version":0,"policies":{}}`,
		`{"schemaVersion":1,"state":"ready","version":1,"policies":{"business_email_minute":{"limit":1,"windowSeconds":60,"updatedAt":"2026-09-04T12:00:00Z"}}}`,
		`{"schemaVersion":1,"state":"ready","version":1,"policies":{"business_email_minute":{"limit":0,"windowSeconds":60,"updatedAt":"2026-09-04T12:00:00Z"}}}`,
		`{"schemaVersion":1,"state":"ready","version":1,"policies":{"business_email_minute":{"limit":1,"windowSeconds":0,"updatedAt":"2026-09-04T12:00:00Z"}}}`,
		`{"schemaVersion":1,"state":"ready","version":1,"policies":{"business_email_minute":{"limit":1,"windowSeconds":60,"updatedAt":"0001-01-01T00:00:00Z"}}}`,
		`{"schemaVersion":2,"state":"ready","version":1,"policies":{}}`,
		`{"schemaVersion":1,"state":"ready","version":1,"policies":{},"extra":true}`,
		`{"schemaVersion":1,"state":"invalidating","mutationToken":"","version":1}`,
		`{"schemaVersion":1,"state":"invalidating","mutationToken":"t","version":0}`,
		`{"schemaVersion":1,"state":"invalidating","mutationToken":"t","version":1,"policies":{}}`,
		`{"schemaVersion":1,"state":"invalidating","mutationToken":"t","version":1,"policies":null}`,
		`{"schemaVersion":1,"state":"invalidating","mutationToken":"t","version":1,"policies":{"business_email_minute":{"limit":1,"windowSeconds":60,"updatedAt":"2026-09-04T12:00:00Z"}}}`,
		`{"schemaVersion":1,"state":"ready","version":1,"policies":{"business_email_minute":{"limit":1,"windowSeconds":60,"updatedAt":"2026-09-04T12:00:00Z"},"business_email_minute":{"limit":2,"windowSeconds":60,"updatedAt":"2026-09-04T12:00:00Z"}}}`,
	}
	for _, raw := range reject {
		if _, err := decodeRateLimitSnapshot(raw); err == nil {
			t.Fatalf("accepted malformed snapshot: %s", raw)
		}
	}

	_ = ready
	if !strings.Contains(ready, `"policies"`) {
		t.Fatalf("ready snapshot must include policies: %s", ready)
	}
}

func TestRateLimitSnapshotInvalidatingOmitsPolicies(t *testing.T) {
	token := "t"
	snapshot := RateLimitSnapshot{
		SchemaVersion: rateLimitPolicySchemaVersion,
		State:         rateLimitPolicyStateInvalidating,
		Version:       1,
		MutationToken: &token,
	}
	raw, err := encodeRateLimitSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "policies") {
		t.Fatalf("invalidating snapshot must omit policies: %s", raw)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["policies"]; ok {
		t.Fatalf("invalidating snapshot must not serialize a null policies field")
	}
	if _, err := decodeRateLimitSnapshot(raw); err != nil {
		t.Fatalf("valid invalidating snapshot rejected: %v", err)
	}
}

func TestRateLimitPolicyRollbackRejectsNonReadyPublicationResult(t *testing.T) {
	for _, result := range []string{"published", "newer"} {
		if err := validateRateLimitPolicyPublicationResult(result); err != nil {
			t.Fatalf("publication result %q rejected: %v", result, err)
		}
	}
	for _, result := range []string{"missing", "token-mismatch", "unexpected"} {
		if err := validateRateLimitPolicyPublicationResult(result); err == nil {
			t.Fatalf("publication result %q accepted", result)
		}
	}
}

func fixedPoliciesWithTimestamp(at time.Time) []RateLimitPolicy {
	policies := FixedRateLimitPolicies()
	for i := range policies {
		policies[i].UpdatedAt = at
	}
	return policies
}

func mustSnapshot(t *testing.T, catalog RateLimitCatalog) RateLimitSnapshot {
	t.Helper()
	snapshot, err := snapshotFromCatalog(catalog)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func stringPointer(value string) *string { return &value }
