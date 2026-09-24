package loginlog

import (
	"net/url"
	"testing"
)

func TestParseListQueryUsesNumericEventAndLoginTypes(t *testing.T) {
	query, err := parseListQuery(url.Values{
		"page": {"1"}, "pageSize": {"20"}, "eventType": {"3"}, "loginType": {"2"}, "account": {"user"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if query.EventType != EventLogout || query.LoginType == nil || *query.LoginType != LoginEmail || query.Account != "user" {
		t.Fatalf("query = %+v", query)
	}
}

func TestParseListQueryRejectsStringEnumsAndUnknownNumbers(t *testing.T) {
	for _, values := range []url.Values{
		{"eventType": {"logout"}},
		{"eventType": {"9"}},
		{"loginType": {"password"}},
		{"loginType": {"0"}},
	} {
		if _, err := parseListQuery(values); err == nil {
			t.Fatalf("parseListQuery(%v) accepted invalid values", values)
		}
	}
}
