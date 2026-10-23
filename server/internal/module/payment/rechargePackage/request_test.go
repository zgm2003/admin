package rechargepackage

import (
	"admin/server/internal/shared/yesno"
	"encoding/json"
	"net/url"
	"testing"
)

func TestCreateRequestParsesAmountAsExactPositiveCents(t *testing.T) {
	for _, test := range []struct {
		raw, wantAmount string
	}{
		{`{"name":"Starter","amount":"10.50","badge":"Popular","sortOrder":1,"status":0}`, "10.50"},
		{`{"name":"Whole","amount":"10","sortOrder":0,"status":1}`, "10"},
	} {
		var request createRequest
		if err := json.Unmarshal([]byte(test.raw), &request); err != nil {
			t.Fatal(err)
		}
		input, err := request.input()
		if err != nil || input.AmountCents != 1050 && test.wantAmount == "10.50" {
			t.Fatalf("input=%+v err=%v", input, err)
		}
		if test.wantAmount == "10" && input.AmountCents != 1000 {
			t.Fatalf("amount cents=%d", input.AmountCents)
		}
	}
}

func TestCreateRequestRejectsInvalidAmountAndEnums(t *testing.T) {
	for _, raw := range []string{
		`{"name":"X","amount":"","sortOrder":0,"status":0}`,
		`{"name":"X","amount":"0","sortOrder":0,"status":0}`,
		`{"name":"X","amount":"-1","sortOrder":0,"status":0}`,
		`{"name":"X","amount":"1.001","sortOrder":0,"status":0}`,
		`{"name":"X","amount":"1","sortOrder":0,"status":"1"}`,
		`{"name":"X","amount":"1","sortOrder":0,"status":2}`,
	} {
		var request createRequest
		if err := json.Unmarshal([]byte(raw), &request); err != nil {
			continue
		}
		if _, err := request.input(); err == nil {
			t.Fatalf("accepted invalid request %s", raw)
		}
	}
}

func TestMetadataRequestDoesNotAcceptImmutableFields(t *testing.T) {
	var request updateRequest
	if err := json.Unmarshal([]byte(`{"name":"Updated","amount":"2.00","badge":null,"sortOrder":3,"status":1}`), &request); err != nil {
		t.Fatal(err)
	}
	if _, err := request.input(); err != nil {
		t.Fatalf("metadata rejected: %v", err)
	}
}

func TestListQueryIsStrict(t *testing.T) {
	q, err := parseListQuery(url.Values{"page": {"1"}, "pageSize": {"20"}, "status": {"1"}})
	if err != nil || q.Page != 1 || q.PageSize != 20 || q.Status == nil || *q.Status != yesno.Yes {
		t.Fatalf("query=%+v err=%v", q, err)
	}
	for _, values := range []url.Values{
		{"page": {"1"}, "pageSize": {"20"}, "unknown": {"x"}},
		{"page": {"1", "2"}, "pageSize": {"20"}},
		{"page": {"1"}, "pageSize": {"20"}, "status": {"01"}},
	} {
		if _, err := parseListQuery(values); err == nil {
			t.Fatalf("accepted invalid query: %v", values)
		}
	}
}

func TestPackageSortOrderMatchesDatabaseRange(t *testing.T) {
	for _, sortOrder := range []int32{-1, -2147483648} {
		if err := validateCreate(CreateInput{Name: "Package", AmountCents: 100, SortOrder: sortOrder, Status: yesno.No}); err == nil {
			t.Fatalf("create accepted negative sortOrder %d", sortOrder)
		}
		if err := validateUpdate(UpdateInput{Name: "Package", AmountCents: 100, SortOrder: sortOrder}); err == nil {
			t.Fatalf("update accepted negative sortOrder %d", sortOrder)
		}
	}
	for _, sortOrder := range []int32{0, 2147483647} {
		if err := validateUpdate(UpdateInput{Name: "Package", AmountCents: 100, SortOrder: sortOrder}); err != nil {
			t.Fatalf("update rejected valid sortOrder %d: %v", sortOrder, err)
		}
	}
}
