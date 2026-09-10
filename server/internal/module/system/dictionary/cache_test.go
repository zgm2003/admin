package dictionary

import "testing"

func TestDecodeOptionsSnapshotRejectsUnknownFields(t *testing.T) {
	raw := `{"schemaVersion":1,"generation":"2","language":"en-US","codes":["user.gender"],"options":{"user.gender":[]},"ignored":true}`
	if _, err := decodeOptionsSnapshot(raw, []string{"user.gender"}, "en-US", "2"); err == nil {
		t.Fatal("cache snapshot with unknown fields was accepted")
	}
}

func TestDecodeOptionsSnapshotRejectsTrailingJSON(t *testing.T) {
	raw := `{"schemaVersion":1,"generation":"2","language":"en-US","codes":["user.gender"],"options":{"user.gender":[]}} {}`
	if _, err := decodeOptionsSnapshot(raw, []string{"user.gender"}, "en-US", "2"); err == nil {
		t.Fatal("cache snapshot with trailing JSON was accepted")
	}
}

func TestDecodeOptionsSnapshotRejectsDuplicateFields(t *testing.T) {
	raw := `{"schemaVersion":1,"schemaVersion":1,"generation":"2","language":"en-US","codes":["user.gender"],"options":{"user.gender":[]}}`
	if _, err := decodeOptionsSnapshot(raw, []string{"user.gender"}, "en-US", "2"); err == nil {
		t.Fatal("cache snapshot with duplicate fields was accepted")
	}
}

func TestDecodeOptionsSnapshotRequiresExactCodesAndUniqueValues(t *testing.T) {
	tests := []string{
		`{"schemaVersion":1,"generation":"2","language":"en-US","codes":["user.gender"],"options":{}}`,
		`{"schemaVersion":1,"generation":"2","language":"en-US","codes":["user.gender"],"options":{"user.gender":[],"extra":[]}}`,
		`{"schemaVersion":1,"generation":"2","language":"en-US","codes":["user.gender"],"options":{"user.gender":[{"label":"Male","value":"male"},{"label":"Other","value":"male"}]}}`,
	}
	for _, raw := range tests {
		if _, err := decodeOptionsSnapshot(raw, []string{"user.gender"}, "en-US", "2"); err == nil {
			t.Fatalf("invalid cache snapshot was accepted: %s", raw)
		}
	}
}

func TestDecodeOptionsSnapshotAcceptsCodesIndependentOfRequestOrder(t *testing.T) {
	raw := `{"schemaVersion":1,"generation":"2","language":"zh-CN","codes":["user.gender","user.status"],"options":{"user.status":[{"label":"启用","value":"enabled"}],"user.gender":[]}}`
	result, err := decodeOptionsSnapshot(raw, []string{"user.status", "user.gender"}, "zh-CN", "2")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 || result["user.status"][0].Value != "enabled" {
		t.Fatalf("result=%+v", result)
	}
}
