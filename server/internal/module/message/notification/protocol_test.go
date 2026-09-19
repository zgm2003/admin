package notification

import (
	"strings"
	"testing"
)

func TestValidateLinkAcceptsOnlyExplicitSafeUnion(t *testing.T) {
	valid := []struct {
		kind LinkType
		link string
	}{
		{LinkNone, ""},
		{LinkInternal, "/message/notification?tab=all#latest"},
		{LinkExternal, "https://example.com/path?q=1"},
	}
	for _, item := range valid {
		if err := ValidateLink(item.kind, item.link); err != nil {
			t.Fatalf("ValidateLink(%q,%q)=%v", item.kind, item.link, err)
		}
	}
	invalid := []struct {
		kind LinkType
		link string
	}{
		{LinkNone, "/x"}, {LinkInternal, ""}, {LinkInternal, "//evil.test"}, {LinkInternal, `/a\b`},
		{LinkInternal, "/a\n"}, {LinkInternal, "https://example.com"}, {LinkExternal, "//evil.test"},
		{LinkExternal, "http://example.com"}, {LinkExternal, "javascript:alert(1)"}, {LinkExternal, "data:text/plain,x"},
		{LinkExternal, "file:///tmp/x"}, {LinkExternal, "https://user:pass@example.com"}, {LinkExternal, "https://example.com\\x"},
		{LinkType("unknown"), ""},
	}
	for _, item := range invalid {
		if err := ValidateLink(item.kind, item.link); err == nil {
			t.Fatalf("ValidateLink(%q,%q) error=nil", item.kind, item.link)
		}
	}
}

func TestValidateCreateContentBoundsAndEnums(t *testing.T) {
	base := Content{Title: "title", ContentHTML: "<p>content</p>", Summary: "summary", Variant: VariantInfo, Priority: PriorityNormal, LinkType: LinkNone}
	if err := ValidateContent(base); err != nil {
		t.Fatal(err)
	}
	tests := []Content{
		{Title: "", ContentHTML: base.ContentHTML, Summary: base.Summary, Variant: base.Variant, Priority: base.Priority, LinkType: base.LinkType},
		{Title: strings.Repeat("界", 129), ContentHTML: base.ContentHTML, Summary: base.Summary, Variant: base.Variant, Priority: base.Priority, LinkType: base.LinkType},
		{Title: base.Title, ContentHTML: base.ContentHTML, Summary: strings.Repeat("界", 257), Variant: base.Variant, Priority: base.Priority, LinkType: base.LinkType},
		{Title: base.Title, ContentHTML: base.ContentHTML, Summary: base.Summary, Variant: Variant("other"), Priority: base.Priority, LinkType: base.LinkType},
		{Title: base.Title, ContentHTML: base.ContentHTML, Summary: base.Summary, Variant: base.Variant, Priority: Priority("other"), LinkType: base.LinkType},
	}
	for _, input := range tests {
		if err := ValidateContent(input); err == nil {
			t.Fatalf("ValidateContent(%+v) error=nil", input)
		}
	}
}
