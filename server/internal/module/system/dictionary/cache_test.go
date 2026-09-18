package dictionary

import (
	"errors"
	"strings"
	"testing"
)

func TestDecodeOptionsSnapshotRejectsUnknownFields(t *testing.T) {
	raw := `{"schemaVersion":1,"generation":2,"language":"en-US","codes":["user.gender"],"options":{"user.gender":[]},"ignored":true}`
	if _, err := decodeOptionsSnapshot(raw, []string{"user.gender"}, "en-US", 2); !errors.Is(err, ErrOptionsSnapshotCorrupt) {
		t.Fatal("cache snapshot with unknown fields was accepted")
	}
}

func TestDecodeOptionsSnapshotRejectsTrailingJSON(t *testing.T) {
	raw := `{"schemaVersion":1,"generation":2,"language":"en-US","codes":["user.gender"],"options":{"user.gender":[]}} {}`
	if _, err := decodeOptionsSnapshot(raw, []string{"user.gender"}, "en-US", 2); !errors.Is(err, ErrOptionsSnapshotCorrupt) {
		t.Fatal("cache snapshot with trailing JSON was accepted")
	}
}

func TestDecodeOptionsSnapshotRejectsDuplicateFields(t *testing.T) {
	raw := `{"schemaVersion":1,"schemaVersion":1,"generation":2,"language":"en-US","codes":["user.gender"],"options":{"user.gender":[]}}`
	if _, err := decodeOptionsSnapshot(raw, []string{"user.gender"}, "en-US", 2); !errors.Is(err, ErrOptionsSnapshotCorrupt) {
		t.Fatal("cache snapshot with duplicate fields was accepted")
	}
}

func TestDecodeOptionsSnapshotRequiresExactCodesAndUniqueValues(t *testing.T) {
	tests := []string{
		`{"schemaVersion":1,"generation":2,"language":"en-US","codes":["user.gender"],"options":{}}`,
		`{"schemaVersion":1,"generation":2,"language":"en-US","codes":["user.gender"],"options":{"user.gender":[],"extra":[]}}`,
		`{"schemaVersion":1,"generation":2,"language":"en-US","codes":["user.gender"],"options":{"user.gender":[{"label":"Male","value":"male"},{"label":"Other","value":"male"}]}}`,
	}
	for _, raw := range tests {
		if _, err := decodeOptionsSnapshot(raw, []string{"user.gender"}, "en-US", 2); !errors.Is(err, ErrOptionsSnapshotCorrupt) {
			t.Fatalf("invalid cache snapshot was accepted: %s", raw)
		}
	}
}

func TestDecodeOptionsSnapshotAcceptsCodesIndependentOfRequestOrder(t *testing.T) {
	raw := `{"schemaVersion":1,"generation":2,"language":"zh-CN","codes":["user.gender","user.status"],"options":{"user.status":[{"label":"启用","value":"enabled"}],"user.gender":[]}}`
	result, err := decodeOptionsSnapshot(raw, []string{"user.status", "user.gender"}, "zh-CN", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 || result["user.status"][0].Value != "enabled" {
		t.Fatalf("result=%+v", result)
	}
}

func TestDecodeOptionsSnapshotRejectsCoordinateMismatchAndInvalidGeneration(t *testing.T) {
	raw := `{"schemaVersion":1,"generation":2,"language":"en-US","codes":["user.gender"],"options":{"user.gender":[]}}`
	for _, test := range []struct {
		name       string
		codes      []string
		language   string
		generation int64
	}{
		{name: "generation", codes: []string{"user.gender"}, language: "en-US", generation: 3},
		{name: "language", codes: []string{"user.gender"}, language: "zh-CN", generation: 2},
		{name: "codes", codes: []string{"user.status"}, language: "en-US", generation: 2},
		{name: "zero generation", codes: []string{"user.gender"}, language: "en-US", generation: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeOptionsSnapshot(raw, test.codes, test.language, test.generation); !errors.Is(err, ErrOptionsSnapshotCorrupt) {
				t.Fatalf("decode error = %v want corrupt snapshot", err)
			}
		})
	}
}

func TestOptionsVariantUsesLanguageAndSortedUniqueCodeSet(t *testing.T) {
	first, err := optionsVariant([]string{"user.status", "user.gender", "user.status"}, "en-US")
	if err != nil {
		t.Fatal(err)
	}
	second, err := optionsVariant([]string{"user.gender", "user.status"}, "en-US")
	if err != nil {
		t.Fatal(err)
	}
	if first != second || !strings.HasPrefix(first, "options:") || len(first) != len("options:")+64 {
		t.Fatalf("variant first=%q second=%q", first, second)
	}
	otherLanguage, err := optionsVariant([]string{"user.gender", "user.status"}, "zh-CN")
	if err != nil {
		t.Fatal(err)
	}
	if otherLanguage == first {
		t.Fatal("language did not affect options variant")
	}
	if _, err := optionsVariant(nil, "en-US"); err == nil {
		t.Fatal("empty code set was accepted")
	}
}
