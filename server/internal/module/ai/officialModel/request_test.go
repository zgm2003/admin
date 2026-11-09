package officialmodel

import (
	"net/url"
	"testing"
)

func TestParseListQuery(t *testing.T) {
	query, err := parseListQuery(url.Values{
		"page": {"2"}, "pageSize": {"50"}, "keyword": {" gpt-5 "}, "vendor": {"1"},
	})
	if err != nil {
		t.Fatalf("parseListQuery() error = %v", err)
	}
	if query.Page != 2 || query.PageSize != 50 || query.Keyword != "gpt-5" || query.Vendor != VendorOpenAI {
		t.Fatalf("parseListQuery() = %+v", query)
	}
	for _, values := range []url.Values{
		{"page": {"1"}, "pageSize": {"20"}, "status": {"1"}},
		{"page": {"1"}, "pageSize": {"20"}, "vendor": {"openai"}},
		{"page": {"1"}, "pageSize": {"20"}, "vendor": {"99"}},
		{"page": {"0"}, "pageSize": {"20"}},
	} {
		if _, err := parseListQuery(values); err == nil {
			t.Fatalf("parseListQuery(%v) accepted invalid query", values)
		}
	}
}

func TestUpdatePriceRequest(t *testing.T) {
	version := int64(3)
	input, output := "5", "30.00"
	cacheRead, cacheWrite := "0.5", ""
	got, err := (updatePriceRequest{
		ExpectedVersion: &version, InputCNY: &input, OutputCNY: &output,
		CacheReadCNY: &cacheRead, CacheWriteCNY: &cacheWrite,
	}).input()
	if err != nil {
		t.Fatalf("input() error = %v", err)
	}
	if got.ExpectedVersion != 3 || got.InputCNYMicros != 5_000_000 || got.OutputCNYMicros != 30_000_000 {
		t.Fatalf("input() = %+v", got)
	}
	if got.CacheReadCNY == nil || *got.CacheReadCNY != 500_000 || got.CacheWriteCNY != nil {
		t.Fatalf("cache input = %+v", got)
	}
	if _, err := (updatePriceRequest{InputCNY: &input, OutputCNY: &output}).input(); err == nil {
		t.Fatal("missing version accepted")
	}
	bad := "abc"
	if _, err := (updatePriceRequest{ExpectedVersion: &version, InputCNY: &bad, OutputCNY: &output}).input(); err == nil {
		t.Fatal("invalid price accepted")
	}
}
