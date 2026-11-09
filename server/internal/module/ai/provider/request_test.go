package provider

import (
	"net/url"
	"testing"

	"admin/server/internal/shared/yesno"
)

func TestCreateRequestInputUsesHTTPSAndDefaults(t *testing.T) {
	name := " OpenAI official "
	baseURL := " https://api.openai.com/v1 "
	key := " sk-test "
	vendor, protocol := VendorOpenAI, ProtocolOpenAI
	input, err := (createRequest{
		Name: &name, Vendor: &vendor, Protocol: &protocol,
		BaseURL: &baseURL, APIKey: &key,
	}).input()
	if err != nil {
		t.Fatalf("input() error = %v", err)
	}
	if input.Name != "OpenAI official" || input.BaseURL != "https://api.openai.com/v1" || input.APIKey != "sk-test" {
		t.Fatalf("input() = %+v", input)
	}
	if input.TimeoutMS != defaultTimeoutMS || input.SortOrder != defaultSortOrder || input.Status != yesno.No {
		t.Fatalf("defaults = %+v", input)
	}
}

func TestCreateRequestParsesTokenMultiplier(t *testing.T) {
	name := "proxy"
	baseURL := "https://api.example.com/v1"
	key := "sk-test"
	vendor, protocol := VendorOpenAI, ProtocolOpenAI
	rate := "0.2"
	input, err := (createRequest{
		Name: &name, Vendor: &vendor, Protocol: &protocol,
		BaseURL: &baseURL, APIKey: &key, TokenRateMultiplier: &rate,
	}).input()
	if err != nil {
		t.Fatal(err)
	}
	if input.TokenRatePPM != 200000 {
		t.Fatalf("TokenRatePPM = %d, want 200000", input.TokenRatePPM)
	}
	for _, invalid := range []string{"-1", "100.1", "abc", "1.0000001"} {
		bad := invalid
		if _, err := (createRequest{
			Name: &name, Vendor: &vendor, Protocol: &protocol,
			BaseURL: &baseURL, APIKey: &key, TokenRateMultiplier: &bad,
		}).input(); err == nil {
			t.Fatalf("multiplier %q accepted", invalid)
		}
	}
}

func TestCreateRequestRejectsNonHTTPSAndMissingKey(t *testing.T) {
	name := "proxy"
	vendor, protocol := VendorOpenAI, ProtocolOpenAI
	for _, baseURL := range []string{"http://api.example.com", "https://api.example.com?x=1", "https://user:pass@api.example.com"} {
		urlValue := baseURL
		if _, err := (createRequest{
			Name: &name, Vendor: &vendor, Protocol: &protocol,
			BaseURL: &urlValue, APIKey: stringPointer("key"),
		}).input(); err == nil {
			t.Fatalf("base URL %q accepted", baseURL)
		}
	}
	baseURL := "https://api.example.com"
	emptyKey := "  "
	if _, err := (createRequest{
		Name: &name, Vendor: &vendor, Protocol: &protocol,
		BaseURL: &baseURL, APIKey: &emptyKey,
	}).input(); err == nil {
		t.Fatal("empty API key accepted")
	}
}

func TestUpdateRequestBlankAPIKeyMeansKeepCurrent(t *testing.T) {
	name := "proxy"
	baseURL := "https://api.example.com/v1"
	blank := "  "
	vendor, protocol := VendorOpenAI, ProtocolOpenAI
	status := yesno.Yes
	timeout, sortOrder := int32(30000), int32(10)
	input, err := (updateRequest{
		Name: &name, Vendor: &vendor, Protocol: &protocol, BaseURL: &baseURL,
		APIKey: &blank, TimeoutMS: &timeout, SortOrder: &sortOrder, Status: &status,
	}).input()
	if err != nil {
		t.Fatalf("input() error = %v", err)
	}
	if input.APIKey != nil {
		t.Fatalf("blank API key must keep current credential, got %q", *input.APIKey)
	}
}

func TestParseListQueryParsesFilters(t *testing.T) {
	query, err := parseListQuery(url.Values{
		"page": {"2"}, "pageSize": {"50"}, "keyword": {" proxy "},
		"vendor": {"2"}, "protocol": {"4"}, "status": {"1"},
	})
	if err != nil {
		t.Fatalf("parseListQuery() error = %v", err)
	}
	if query.Page != 2 || query.PageSize != 50 || query.Keyword != "proxy" ||
		query.Vendor != VendorAnthropic || query.Protocol != ProtocolOpenAIResponses ||
		query.Status == nil || *query.Status != yesno.Yes {
		t.Fatalf("parseListQuery() = %+v", query)
	}
	for _, values := range []url.Values{
		{"page": {"1"}, "pageSize": {"20"}, "vendor": {"99"}},
		{"page": {"1"}, "pageSize": {"20"}, "status": {"2"}},
		{"page": {"1"}, "pageSize": {"20"}, "unknown": {"x"}},
	} {
		if _, err := parseListQuery(values); err == nil {
			t.Fatalf("query %v accepted", values)
		}
	}
}

func stringPointer(value string) *string { return &value }
