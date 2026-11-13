package transaction

import (
	"net/url"
	"testing"
)

func TestParsePersonalLedgerFilters(t *testing.T) {
	values := url.Values{
		"page":     {"2"},
		"pageSize": {"20"},
		"flow":     {"debit"},
		"keyword":  {" AI-14 "},
		"from":     {"2026-11-13T00:00:00+08:00"},
		"to":       {"2026-11-14T00:00:00+08:00"},
	}
	query, err := parseListQuery(values, 7, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if query.Page != 2 || query.PageSize != 20 || query.Flow != FlowDebit || query.Keyword != "AI-14" {
		t.Fatalf("unexpected personal ledger query: %+v", query)
	}
	if query.From == nil || query.To == nil {
		t.Fatalf("unexpected date bounds: %+v", query)
	}
	if _, offset := query.From.Zone(); offset != 8*60*60 {
		t.Fatalf("unexpected date offset: %d", offset)
	}
}

func TestParseAdminLedgerFilters(t *testing.T) {
	values := url.Values{
		"page":       {"1"},
		"pageSize":   {"20"},
		"userId":     {"169"},
		"platformId": {"2"},
		"flow":       {"credit"},
		"keyword":    {"PO2026"},
	}
	query, err := parseAdminListQuery(values)
	if err != nil {
		t.Fatal(err)
	}
	if query.UserID != 169 || query.PlatformID != 2 || query.Flow != FlowCredit || query.Keyword != "PO2026" {
		t.Fatalf("unexpected admin ledger query: %+v", query)
	}
}

func TestParseLedgerFiltersRejectsInvalidInput(t *testing.T) {
	for name, values := range map[string]url.Values{
		"unknown key":   {"page": {"1"}, "pageSize": {"20"}, "source": {"credit"}},
		"unknown flow":  {"page": {"1"}, "pageSize": {"20"}, "flow": {"income"}},
		"bad date":      {"page": {"1"}, "pageSize": {"20"}, "from": {"2026-11-13"}},
		"reversed date": {"page": {"1"}, "pageSize": {"20"}, "from": {"2026-11-14T00:00:00Z"}, "to": {"2026-11-13T00:00:00Z"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseListQuery(values, 7, 2, true); err == nil {
				t.Fatal("expected invalid query to be rejected")
			}
		})
	}
}
