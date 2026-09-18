package uploadrule

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestGenerateObjectKeyIsSelfDescribingAndRoundTrips(t *testing.T) {
	now := time.Date(2026, 9, 17, 13, 45, 1, 0, time.UTC)
	coordinates := ObjectCoordinates{PlatformID: 1, RuleID: 20, CosConfigID: 10, Version: 3}
	key, err := generateObjectKey("article/cover", coordinates, ".PNG", now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key, "article/cover/.admin-storage/v2/p1/r20/c10/v3/2026/09/17/") || !strings.HasSuffix(key, ".png") {
		t.Fatalf("generated key = %q", key)
	}
	parsed, err := parseV2ObjectKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if parsed != coordinates {
		t.Fatalf("parsed = %+v want %+v", parsed, coordinates)
	}
	// 64 字节上限的 code 仍然可生成、可解析。
	longest := strings.Repeat("a", 32) + "/" + strings.Repeat("b", 31)
	if len(longest) != 64 {
		t.Fatalf("fixture code length = %d", len(longest))
	}
	longKey, err := generateObjectKey(longest, coordinates, "png", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseV2ObjectKey(longKey); err != nil {
		t.Fatalf("longest code parse error = %v", err)
	}
}

func TestGenerateObjectKeyRejectsInvalidInput(t *testing.T) {
	now := time.Now().UTC()
	valid := ObjectCoordinates{PlatformID: 1, RuleID: 1, CosConfigID: 1, Version: 1}
	for _, test := range []struct {
		name   string
		code   string
		coords ObjectCoordinates
		ext    string
	}{
		{"empty code", "", valid, "png"},
		{"65 byte code", strings.Repeat("a", 65), valid, "png"},
		{"uppercase code", "Avatar", valid, "png"},
		{"leading dot segment", ".hidden", valid, "png"},
		{"empty segment", "a//b", valid, "png"},
		{"dotdot", "a..b", valid, "png"},
		{"unicode code", "头像", valid, "png"},
		{"reserved marker", "evil/.admin-storage", valid, "png"},
		{"zero platform", "avatar", ObjectCoordinates{PlatformID: 0, RuleID: 1, CosConfigID: 1, Version: 1}, "png"},
		{"negative version", "avatar", ObjectCoordinates{PlatformID: 1, RuleID: 1, CosConfigID: 1, Version: -1}, "png"},
		{"bad extension", "avatar", valid, "p!ng"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := generateObjectKey(test.code, test.coords, test.ext, now); !errors.Is(err, ErrObjectKeyInvalid) {
				t.Fatalf("generate error = %v want ErrObjectKeyInvalid", err)
			}
		})
	}
}

func TestParseV2ObjectKeyRejectsInvalidKeys(t *testing.T) {
	const valid = "avatar/.admin-storage/v2/p1/r2/c3/v4/2026/09/17/6e7e53334a6da066b130a89cc3f69535.png"
	if _, err := parseV2ObjectKey(valid); err != nil {
		t.Fatalf("valid key rejected: %v", err)
	}
	for _, test := range []struct {
		name string
		key  string
	}{
		{"legacy key", "avatar/2026/09/17/6e7e53334a6da066b130a89cc3f69535.png"},
		{"legacy nested key", "article/cover/2026/09/17/6e7e53334a6da066b130a89cc3f69535.png"},
		{"empty key", ""},
		{"missing marker", "avatar/v2/p1/r2/c3/v4/2026/09/17/6e7e53334a6da066b130a89cc3f69535.png"},
		{"duplicate marker", "avatar/.admin-storage/v2/.admin-storage/p1/r2/c3/v4/2026/09/17/6e7e53334a6da066b130a89cc3f69535.png"},
		{"wrong version segment", strings.Replace(valid, "/v2/", "/v3/", 1)},
		{"missing segment", strings.Replace(valid, "/r2", "", 1)},
		{"extra segment", strings.Replace(valid, "/c3/", "/c3/x/", 1)},
		{"zero identifier", strings.Replace(valid, "/r2/", "/r0/", 1)},
		{"leading zero identifier", strings.Replace(valid, "/r2/", "/r02/", 1)},
		{"non numeric identifier", strings.Replace(valid, "/r2/", "/rx/", 1)},
		{"invalid month", strings.Replace(valid, "/09/17/", "/13/17/", 1)},
		{"invalid day", strings.Replace(valid, "/09/17/", "/09/32/", 1)},
		{"uppercase hex", strings.Replace(valid, "6e7e53334a6da066b130a89cc3f69535", strings.ToUpper("6e7e53334a6da066b130a89cc3f69535"), 1)},
		{"short random", strings.Replace(valid, "6e7e53334a6da066b130a89cc3f69535", "abc", 1)},
		{"missing extension", strings.TrimSuffix(valid, ".png")},
		{"uppercase extension", strings.TrimSuffix(valid, "png") + "PNG"},
		{"dotdot", strings.Replace(valid, "avatar/", "av../", 1)},
		{"backslash", strings.Replace(valid, "avatar/", "av\\atar/", 1)},
		{"control character", strings.Replace(valid, "avatar/", "av\tatar/", 1)},
		{"marker without code", ".admin-storage/v2/p1/r2/c3/v4/2026/09/17/6e7e53334a6da066b130a89cc3f69535.png"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseV2ObjectKey(test.key); !errors.Is(err, ErrObjectKeyInvalid) {
				t.Fatalf("parse error = %v want ErrObjectKeyInvalid", err)
			}
		})
	}
}
