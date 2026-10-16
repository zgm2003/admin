package transaction

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"

	"admin/server/internal/authcontext"
	"github.com/gin-gonic/gin"
)

type scopedTransactionRepository struct {
	row      Model
	queries  []ListQuery
	countErr error
	listErr  error
}

func (r *scopedTransactionRepository) Find(context.Context, int64) (Model, error) { return r.row, nil }
func (r *scopedTransactionRepository) Count(_ context.Context, q ListQuery) (int64, error) {
	r.queries = append(r.queries, q)
	return 1, r.countErr
}
func (r *scopedTransactionRepository) List(_ context.Context, q ListQuery) ([]Model, error) {
	r.queries = append(r.queries, q)
	return []Model{r.row}, r.listErr
}

func TestPersonalTransactionServiceFailsClosedForInvalidIdentityAndForeignRows(t *testing.T) {
	for _, q := range []ListQuery{{Page: 1, PageSize: 20}, {Page: 1, PageSize: 20, UserID: 42}, {Page: 1, PageSize: 20, PlatformID: 2}, {Page: 1, PageSize: 20, UserID: -1, PlatformID: 2}} {
		repo := &scopedTransactionRepository{}
		if _, err := NewService(repo).PersonalList(context.Background(), q); err == nil || len(repo.queries) != 0 {
			t.Fatalf("invalid query=%+v err=%v reads=%v", q, err, repo.queries)
		}
	}
	baseline := Model{
		ID: 12, TransactionNo: "PTidentityfixture", UserID: 42, PlatformID: 2,
		AmountUnits: 3_000_000, BalanceAfterUnits: -700_000_000, OrderID: ptrInt64(99),
		Username: "named-user", PlatformCode: "fixture-platform", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0),
	}
	query := ListQuery{Page: 1, PageSize: 20, UserID: 42, PlatformID: 2}
	valid, err := NewService(&scopedTransactionRepository{row: baseline}).PersonalList(context.Background(), query)
	if err != nil || len(valid.List) != 1 || valid.List[0].ID != 12 || valid.List[0].Amount != "0.03" || valid.List[0].BalanceBefore != "-7.03" {
		t.Fatalf("legal personal baseline=%+v err=%v", valid, err)
	}
	for _, identity := range []struct {
		name               string
		userID, platformID int64
	}{{"foreign-user", 99, 2}, {"foreign-platform", 42, 3}} {
		t.Run(identity.name, func(t *testing.T) {
			row := baseline
			row.UserID, row.PlatformID = identity.userID, identity.platformID
			service := NewService(&scopedTransactionRepository{row: row})
			projected, err := service.Get(context.Background(), row.ID)
			if err != nil || projected.ID != 12 || projected.Amount != "0.03" || projected.BalanceBefore != "-7.03" {
				t.Fatalf("foreign row must remain independently projectable: row=%+v value=%+v err=%v", row, projected, err)
			}
			result, err := service.PersonalList(context.Background(), query)
			cause := errors.Unwrap(err)
			if result.List != nil || cause == nil || cause.Error() != "transaction identity mismatch" {
				t.Fatalf("foreign row must fail specifically on identity: row=%+v result=%+v err=%v cause=%v", row, result, err, cause)
			}
		})
	}
}

func TestPersonalTransactionServicePreservesErrorsAndEmptyLists(t *testing.T) {
	want := errors.New("storage unavailable")
	for _, fixture := range []struct {
		countErr, listErr error
		reads             int
	}{{want, nil, 1}, {nil, want, 2}} {
		repo := &scopedTransactionRepository{countErr: fixture.countErr, listErr: fixture.listErr}
		result, err := NewService(repo).PersonalList(context.Background(), ListQuery{Page: 1, PageSize: 20, UserID: 42, PlatformID: 2})
		if !errors.Is(err, want) || result.List != nil || len(repo.queries) != fixture.reads {
			t.Fatalf("result=%+v err=%v reads=%v", result, err, repo.queries)
		}
	}
	result, err := NewService(&fakeTransactionRepository{}).PersonalList(context.Background(), ListQuery{Page: 2, PageSize: 20, UserID: 42, PlatformID: 2})
	if err != nil || result.List == nil || len(result.List) != 0 || result.Page != 2 || result.PageSize != 20 {
		t.Fatalf("empty result=%+v err=%v", result, err)
	}
}

func TestAdminTransactionJSONKeepsIdentityAndNullableOrder(t *testing.T) {
	for _, source := range []struct{ orderID, redeemCodeID *int64 }{{ptrInt64(99), nil}, {nil, ptrInt64(88)}} {
		repo := &scopedTransactionRepository{row: Model{ID: 12, TransactionNo: "PTfixture", UserID: 42, PlatformID: 2, AmountUnits: 1, BalanceAfterUnits: 0, OrderID: source.orderID, RedeemCodeID: source.redeemCodeID}}
		projected, err := NewService(repo).Get(context.Background(), 12)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(projected)
		if err != nil {
			t.Fatal(err)
		}
		var row map[string]any
		if err := json.Unmarshal(encoded, &row); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"userId", "username", "platformId", "platformCode", "orderId"} {
			if _, ok := row[key]; !ok {
				t.Fatalf("admin field %q missing: %s", key, encoded)
			}
		}
		if row["userId"] != float64(42) || row["platformId"] != float64(2) || row["amount"] != "0.00000001" || row["balanceBefore"] != "-0.00000001" || row["balanceAfter"] != "0" {
			t.Fatalf("admin row = %v", row)
		}
		if _, ok := row["redeemCodeId"]; ok {
			t.Fatal("not-yet-implemented redeemCodeId exposed")
		}
	}
}

func TestPersonalTransactionJSONContainsOnlyPersonalFieldsAndAuthenticatedScope(t *testing.T) {
	repo := &scopedTransactionRepository{row: Model{ID: 12, TransactionNo: "PTfixture", UserID: 42, PlatformID: 2, Username: "private-user", PlatformCode: "private-platform", AmountUnits: 3_000_000, BalanceAfterUnits: -700_000_000, OrderID: ptrInt64(99), CreatedAt: time.Unix(1, 0)}}
	response := transactionRequest(NewHandler(NewService(repo)).PersonalList, "page=1&pageSize=20", &authcontext.Identity{UserID: 42, PlatformID: 2})
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		Data struct {
			List []map[string]any `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.List) != 1 {
		t.Fatalf("list = %v", body.Data.List)
	}
	row := body.Data.List[0]
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"amount", "balanceAfter", "balanceBefore", "createdAt", "id", "sourceType", "transactionNo"}) {
		t.Fatalf("personal transaction fields = %v", keys)
	}
	if row["amount"] != "0.03" || row["balanceBefore"] != "-7.03" || row["balanceAfter"] != "-7" || row["sourceType"] != float64(1) {
		t.Fatalf("row = %v", row)
	}
	if len(repo.queries) != 2 {
		t.Fatalf("queries = %v", repo.queries)
	}
	for _, q := range repo.queries {
		if q.UserID != 42 || q.PlatformID != 2 || !q.Personal {
			t.Fatalf("scope = %+v", q)
		}
	}
}

func TestPersonalTransactionRejectsAlternativeIdentityAndInvalidAuthentication(t *testing.T) {
	for _, query := range []string{"page=1&pageSize=20&userId=99", "page=1&pageSize=20&platformId=3", "page=1&pageSize=20&unknown=1", "page=1&page=2&pageSize=20", "page=1&pageSize=20&userId=%zz", "page=1&pageSize=20&userId=42;platformId=3"} {
		repo := &scopedTransactionRepository{}
		response := transactionRequest(NewHandler(NewService(repo)).PersonalList, query, &authcontext.Identity{UserID: 42, PlatformID: 2})
		if response.Code != http.StatusBadRequest || len(repo.queries) != 0 {
			t.Fatalf("query %q: status=%d reads=%v", query, response.Code, repo.queries)
		}
	}
	for _, identity := range []*authcontext.Identity{nil, {}, {UserID: -1, PlatformID: 2}, {UserID: 42}, {UserID: 42, PlatformID: -1}} {
		repo := &scopedTransactionRepository{}
		response := transactionRequest(NewHandler(NewService(repo)).PersonalList, "page=1&pageSize=20", identity)
		if response.Code != http.StatusUnauthorized || len(repo.queries) != 0 {
			t.Fatalf("identity %+v: status=%d reads=%v", identity, response.Code, repo.queries)
		}
	}
}

func TestAdminTransactionListRejectsMalformedQueryBeforeReading(t *testing.T) {
	for _, query := range []string{"page=1&pageSize=20&userId=%zz", "page=1&pageSize=20&userId=42;platformId=3"} {
		repo := &scopedTransactionRepository{}
		response := transactionRequest(NewHandler(NewService(repo)).List, query, nil)
		if response.Code != http.StatusBadRequest || len(repo.queries) != 0 {
			t.Fatalf("query %q: status=%d reads=%v", query, response.Code, repo.queries)
		}
	}
}

func TestAdminTransactionQueryRejectsEmptyDuplicateUnknownAndInvalidDecimal(t *testing.T) {
	for _, suffix := range []string{"userId=", "platformId=", "userId=1&userId=2", "unknown=1", "userId=+1", "userId=-1", "userId=0", "userId=1.0", "userId=0x10", "userId=9223372036854775808", "platformId=%20", "page=2", "pageSize="} {
		t.Run(suffix, func(t *testing.T) {
			values, err := url.ParseQuery("page=1&pageSize=20&" + suffix)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := parseAdminListQuery(values); err == nil {
				t.Fatalf("invalid query accepted: %s", suffix)
			}
		})
	}
	if _, err := parseAdminListQuery(url.Values{"page": {strconv.Itoa(int(^uint(0) >> 1))}, "pageSize": {"100"}}); err == nil {
		t.Fatal("overflowing offset accepted")
	}
	q, err := parseAdminListQuery(url.Values{"page": {"101"}, "pageSize": {"20"}, "userId": {"00042"}, "platformId": {"2"}})
	if err != nil || q.Page != 101 || q.UserID != 42 || q.PlatformID != 2 || q.Personal {
		t.Fatalf("valid query=%+v, err=%v", q, err)
	}
}

func TestServiceRejectsPersonalTransactionQueryWithoutIdentity(t *testing.T) {
	for _, q := range []ListQuery{{Page: 1, PageSize: 20, Personal: true}, {Page: 1, PageSize: 20, Personal: true, UserID: 42}, {Page: 1, PageSize: 20, Personal: true, PlatformID: 2}} {
		repo := &scopedTransactionRepository{}
		if _, err := NewService(repo).List(context.Background(), q); err == nil || len(repo.queries) != 0 {
			t.Fatalf("query=%+v, err=%v, reads=%v", q, err, repo.queries)
		}
	}
}

func transactionRequest(handler gin.HandlerFunc, query string, identity *authcontext.Identity) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(http.MethodGet, "/payment/wallet/transaction?"+query, nil)
	if identity != nil {
		authcontext.Set(c, *identity)
	}
	handler(c)
	return response
}
