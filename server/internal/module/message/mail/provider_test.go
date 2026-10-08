package mail

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestProviderErrorRetainsWrappedCodeAndSummary(t *testing.T) {
	cause := NewProviderError("FailedOperation.InsufficientQuota", "quota exhausted")
	result := providerError(fmt.Errorf("send mail: %w", cause))
	if result.Code != "FailedOperation.InsufficientQuota" || result.Summary != "quota exhausted" {
		t.Fatalf("provider error=%+v, want original provider diagnostic", result)
	}
}

func TestProviderErrorBoundsSummaryWithoutSplittingUTF8(t *testing.T) {
	for _, input := range []string{strings.Repeat("邮件", 100), "invalid\xfftext", "invalid\x00text", "  \n  "} {
		failure := NewProviderError("ses_error", input)
		if !utf8.ValidString(failure.Summary) || len(failure.Summary) > 512 || strings.ContainsRune(failure.Summary, 0) {
			t.Fatalf("summary has invalid UTF-8 or exceeds storage bound: %q", failure.Summary)
		}
	}
}

func TestProviderErrorNormalizesCodesThatCannotBeStoredSafely(t *testing.T) {
	for _, code := range []string{"", "  ", strings.Repeat("x", 129), "error\x00code", "error\ncode", "invalid\xffcode"} {
		failure := NewProviderError(code, "provider diagnostic")
		if failure.Code != "provider_error" || failure.Summary != "provider diagnostic" {
			t.Fatalf("code=%q failure=%+v", code, failure)
		}
	}
}
