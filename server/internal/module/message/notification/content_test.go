package notification

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeContentUsesNotificationAllowlist(t *testing.T) {
	raw := `<p onclick="bad()">Hello <strong>bold</strong><em>em</em><u>u</u></p><h2>H2</h2><h3>H3</h3><ul><li>one</li></ul><ol><li>two</li></ol><script>alert(1)</script><style>bad</style><img src=x onerror=bad()><video></video><table><tr><td>x</td></tr></table><code>x</code>`
	got, err := SanitizeContent(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"onclick", "script", "style", "img", "video", "table", "code", "onerror"} {
		if strings.Contains(strings.ToLower(got), forbidden) {
			t.Fatalf("sanitized content contains %q: %s", forbidden, got)
		}
	}
	for _, allowed := range []string{"<p>", "<strong>", "<em>", "<u>", "<h2>", "<h3>", "<ul>", "<ol>", "<li>"} {
		if !strings.Contains(got, allowed) {
			t.Fatalf("sanitized content missing %q: %s", allowed, got)
		}
	}
}

func TestSanitizeContentAllowsOnlyHardenedHTTPSAnchors(t *testing.T) {
	got, err := SanitizeContent(`<p><a href="https://example.com/a?q=1" target="self" rel="bad">safe</a><a href="http://evil.test">http</a><a href="javascript:alert(1)">js</a></p>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `href="https://example.com/a?q=1"`) || !strings.Contains(got, `target="_blank"`) || !strings.Contains(got, `rel="noopener noreferrer"`) {
		t.Fatalf("safe anchor not hardened: %s", got)
	}
	if strings.Contains(got, "http://") || strings.Contains(got, "javascript:") {
		t.Fatalf("unsafe anchor survived: %s", got)
	}
}

func TestSanitizeContentRejectsEmptyAndOversizeVisibleContent(t *testing.T) {
	for _, raw := range []string{"", "   ", "<p><br></p>", "<script>alert(1)</script>"} {
		if _, err := SanitizeContent(raw); err == nil {
			t.Fatalf("SanitizeContent(%q) error=nil", raw)
		}
	}
	if _, err := SanitizeContent("<p>" + strings.Repeat("界", 16385) + "</p>"); err == nil {
		t.Fatal("oversize sanitized content accepted")
	}
}

func TestSummaryFromHTMLCollapsesWhitespaceAndTruncatesRunes(t *testing.T) {
	raw := "<p>你好   世界</p><p>" + strings.Repeat("界", 260) + "</p>"
	summary, err := SummaryFromHTML(raw)
	if err != nil {
		t.Fatal(err)
	}
	if utf8.RuneCountInString(summary) != 256 || !strings.HasPrefix(summary, "你好 世界 ") || !utf8.ValidString(summary) {
		t.Fatalf("summary runes=%d value=%q", utf8.RuneCountInString(summary), summary)
	}
}
