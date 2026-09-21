package notificationtask

import (
	"net/url"
	"testing"
	"time"
)

func TestParseListQueryAcceptsApprovedFilters(t *testing.T) {
	query, err := parseListQuery(url.Values{
		"page":         {"2"},
		"pageSize":     {"50"},
		"platformId":   {"7"},
		"status":       {"4"},
		"audienceType": {"role"},
		"keyword":      {" maintenance "},
		"from":         {"2026-09-18T00:00:00Z"},
		"to":           {"2026-09-19T00:00:00Z"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if query.Page != 2 || query.PageSize != 50 || query.PlatformID == nil || *query.PlatformID != 7 || query.Status != StatusProcessing || query.AudienceType != AudienceRole || query.Keyword != "maintenance" {
		t.Fatalf("query=%+v", query)
	}
	if query.From == nil || query.To == nil || !query.From.Equal(time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)) || !query.To.Equal(time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("time range=%v..%v", query.From, query.To)
	}
}

func TestParseListQueryRejectsMalformedFilters(t *testing.T) {
	tests := []url.Values{
		{"platformId": {"0"}},
		{"audienceType": {"all"}},
		{"keyword": {string(make([]byte, 129))}},
		{"from": {"2026-09-19T00:00:00Z"}, "to": {"2026-09-18T00:00:00Z"}},
		{"from": {"not-a-time"}},
		{"status": {"draft"}},
		{"status": {"8"}},
		{"unknown": {"value"}},
	}
	for _, values := range tests {
		if _, err := parseListQuery(values); err == nil {
			t.Fatalf("query accepted: %v", values)
		}
	}
}
