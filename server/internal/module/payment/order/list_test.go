package order

import (
	"net/url"
	"testing"
)

func TestParseOrderFilters(t *testing.T) {
	values := url.Values{
		"page":     {"1"},
		"pageSize": {"20"},
		"keyword":  {" PO2026 "},
		"status":   {"2"},
		"from":     {"2026-11-01T00:00:00+08:00"},
		"to":       {"2026-11-30T23:59:59+08:00"},
	}
	query, err := parseListQuery(values, 7, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if query.Keyword != "PO2026" || query.Status == nil || *query.Status != StatusPaid {
		t.Fatalf("unexpected order query: %+v", query)
	}
	if query.From == nil || query.To == nil {
		t.Fatalf("missing date bounds: %+v", query)
	}
}

func TestParseOrderFiltersRejectsInvalidInput(t *testing.T) {
	for name, values := range map[string]url.Values{
		"unknown key":    {"page": {"1"}, "pageSize": {"20"}, "userId": {"7"}},
		"unknown status": {"page": {"1"}, "pageSize": {"20"}, "status": {"9"}},
		"bad date":       {"page": {"1"}, "pageSize": {"20"}, "from": {"2026-11-01"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseListQuery(values, 7, 2, true); err == nil {
				t.Fatal("expected invalid order query to be rejected")
			}
		})
	}
}
