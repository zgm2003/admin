package wallet

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"

	"admin/server/internal/authcontext"
	"github.com/gin-gonic/gin"
)

func (f *fakeWalletRepository) FindAdmin(ctx context.Context, userID int64) (Model, bool, error) {
	return f.Find(ctx, userID)
}

func TestAdminWalletDetailRetainsNamedIdentity(t *testing.T) {
	repo := &fakeWalletRepository{found: true, value: Model{UserID: 42, Username: "named-user", BalanceUnits: -1}}
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/payment/wallet/42", nil)
	c.Params = gin.Params{{Key: "userId", Value: "42"}}
	NewHandler(NewService(repo)).AdminDetail(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data["userId"] != float64(42) || body.Data["username"] != "named-user" || body.Data["balance"] != "-0.00000001" {
		t.Fatalf("admin wallet = %v", body.Data)
	}
}

func TestAdminWalletDetailDoesNotInventMissingWallet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/payment/wallet/42", nil)
	c.Params = gin.Params{{Key: "userId", Value: "42"}}
	NewHandler(NewService(&fakeWalletRepository{})).AdminDetail(c)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("missing admin wallet status=%d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestAdminWalletListRejectsMalformedQuery(t *testing.T) {
	response := walletRequest(NewHandler(NewService(&fakeWalletRepository{})).AdminList, "page=1&pageSize=20&unknown=%zz", nil)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d: %s", response.Code, response.Body.String())
	}
}

func TestCurrentWalletJSONContainsOnlyPersonalFields(t *testing.T) {
	for _, fixture := range []struct {
		name    string
		found   bool
		units   int64
		balance string
	}{
		{"absent", false, 0, "0"}, {"zero", true, 0, "0"},
		{"negative", true, -700_000_001, "-7.00000001"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			repo := &fakeWalletRepository{found: fixture.found, value: Model{UserID: 42, BalanceUnits: fixture.units, Username: "admin-only"}}
			response := walletRequest(NewHandler(NewService(repo)).Current, "", &authcontext.Identity{UserID: 42, PlatformID: 2})
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", response.Code, response.Body.String())
			}
			var body struct {
				Data map[string]any `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			keys := make([]string, 0, len(body.Data))
			for key := range body.Data {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			if !reflect.DeepEqual(keys, []string{"balance", "currency", "hasWallet"}) {
				t.Fatalf("personal wallet fields = %v", keys)
			}
			if body.Data["balance"] != fixture.balance || body.Data["currency"] != "CNY" || body.Data["hasWallet"] != fixture.found {
				t.Fatalf("wallet = %v", body.Data)
			}
		})
	}
}

func TestCurrentWalletRejectsAlternativeIdentityAndInvalidAuthentication(t *testing.T) {
	for _, query := range []string{"userId=99", "platformId=3", "unknown=1", "userId=42&userId=42", "userId=%zz", "userId=42;platformId=3"} {
		repo := &fakeWalletRepository{}
		response := walletRequest(NewHandler(NewService(repo)).Current, query, &authcontext.Identity{UserID: 42, PlatformID: 2})
		if response.Code != http.StatusBadRequest || repo.called != 0 {
			t.Fatalf("query %q: status=%d read=%d", query, response.Code, repo.called)
		}
	}
	for _, identity := range []*authcontext.Identity{nil, {}, {UserID: -1}, {UserID: 42, PlatformID: 0}} {
		repo := &fakeWalletRepository{}
		response := walletRequest(NewHandler(NewService(repo)).Current, "", identity)
		if response.Code != http.StatusUnauthorized || repo.called != 0 {
			t.Fatalf("identity %+v: status=%d read=%d", identity, response.Code, repo.called)
		}
	}
}

func walletRequest(handler gin.HandlerFunc, query string, identity *authcontext.Identity) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(http.MethodGet, "/payment/wallet?"+query, nil)
	if identity != nil {
		authcontext.Set(c, *identity)
	}
	handler(c)
	return response
}
