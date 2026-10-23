package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestUpgradeUserPreservesIdentityAndGeneration(t *testing.T) {
	raw := `{"schemaVersion":1,"state":"ready","userId":42,"generation":"unchanged","isEnabled":true,"deleted":false,"mutationToken":null}`
	next, err := upgradeUser("auth:user-state:42", raw)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(next), &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["generation"]) != `"unchanged"` || string(fields["status"]) != "true" || string(fields["schemaVersion"]) != "2" {
		t.Fatal(next)
	}
	if _, found := fields["isEnabled"]; found {
		t.Fatal("legacy field remains")
	}
	again, err := upgradeUser("auth:user-state:42", next)
	if err != nil || again != next {
		t.Fatal("not idempotent", err)
	}
	for _, bad := range []string{"", `{}`, strings.Replace(raw, `"ready"`, `"invalidating"`, 1), strings.Replace(raw, `"isEnabled":true`, `"isEnabled":true,"status":false`, 1), strings.Replace(raw, `"isEnabled":true`, `"isEnabled":null`, 1), strings.Replace(raw, `"userId":42`, `"userId":42,"userId":42`, 1), raw + "{}"} {
		if _, err := upgradeUser("auth:user-state:42", bad); err == nil {
			t.Fatal("malformed payload accepted", bad)
		}
	}
	if _, err := upgradeUser("auth:user-state:43", raw); err == nil {
		t.Fatal("identity mismatch accepted")
	}
}

func TestBackupTTLPreservesPersistentAndMissingSentinels(t *testing.T) {
	for _, test := range []struct {
		duration time.Duration
		want     int64
	}{{-1, -1}, {-2, -2}, {1500 * time.Millisecond, 1500}, {0, 0}} {
		if got := backupTTLMilliseconds(test.duration); got != test.want {
			t.Fatalf("TTL %v = %d, want %d", test.duration, got, test.want)
		}
	}
}
