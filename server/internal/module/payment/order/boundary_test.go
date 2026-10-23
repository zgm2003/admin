package order

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"admin/server/internal/authcontext"
	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/module/payment"
	paymentconfig "admin/server/internal/module/payment/config"
	"admin/server/internal/payment/alipay"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/response"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

type orderReadRepository struct {
	rows        []Model
	environment payment.Environment
	query       ListQuery
	calls       int
}

func (*orderReadRepository) Confirm(context.Context, ConfirmedPayment, *int64, time.Time) (SettlementResult, error) {
	return SettlementResult{}, errors.New("confirmation not used by read fixture")
}
func (r *orderReadRepository) Count(_ context.Context, query ListQuery) (int64, error) {
	r.query = query
	r.calls++
	return int64(len(r.rows)), nil
}
func (r *orderReadRepository) List(_ context.Context, query ListQuery) ([]OrderRecord, error) {
	r.query = query
	r.calls++
	rows := make([]OrderRecord, 0, len(r.rows))
	for _, row := range r.rows {
		rows = append(rows, OrderRecord{Model: row, ConfigEnvironment: r.environment})
	}
	return rows, nil
}
func (r *orderReadRepository) Find(context.Context, int64) (OrderRecord, error) {
	return OrderRecord{Model: r.rows[0], ConfigEnvironment: r.environment}, nil
}

func validReadOrder() Model {
	return Model{ID: 1, OrderNo: "ORDER-1", UserID: 7, PlatformID: 1, PackageName: "Credits", AmountCents: 300, Status: StatusPending, ExpiresAt: time.Now().Add(time.Hour)}
}

func TestPersonalOrderListFailsClosedForWrongStoredIdentity(t *testing.T) {
	repo := &orderReadRepository{rows: []Model{validReadOrder()}}
	service := NewService(repo)
	query := ListQuery{Page: 1, PageSize: 20, UserID: 7, PlatformID: 1, Personal: true}
	if got, err := service.List(context.Background(), query); err != nil || len(got.List) != 1 || repo.calls != 2 || repo.query.UserID != 7 || repo.query.PlatformID != 1 {
		t.Fatalf("valid personal baseline result=%+v query=%+v err=%v", got, repo.query, err)
	}
	for _, change := range []func(*Model){func(row *Model) { row.UserID = 8 }, func(row *Model) { row.PlatformID = 2 }} {
		repo.rows[0] = validReadOrder()
		change(&repo.rows[0])
		if _, err := service.List(context.Background(), query); err == nil {
			t.Fatal("foreign order escaped personal list")
		}
	}
}

func TestOrderHandlersRejectMalformedAndAlternativeIdentityTransport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) { authcontext.Set(c, authcontext.Identity{UserID: 7, PlatformID: 1}); c.Next() })
	handler := NewHandler(NewService(nil))
	router.GET("/admin", handler.List)
	router.GET("/personal", handler.PersonalList)
	router.GET("/personal/:id", handler.PersonalGet)
	router.POST("/personal/:id/pay", handler.Pay)
	router.GET("/options", handler.Options)
	router.POST("/personal", handler.Create)
	router.GET("/admin/:id", handler.Get)
	for _, test := range []struct{ method, path, body string }{
		{"GET", "/admin?page=1&pageSize=20&userId=%zz", ""},
		{"GET", "/personal?page=1&pageSize=20&userId=%zz", ""},
		{"GET", "/personal/1?userId=8", ""},
		{"POST", "/personal/1/pay?platformId=2", ""},
		{"POST", "/personal/1/pay", `{"userId":8}`},
		{"GET", "/options?platformId=2", ""},
		{"POST", "/personal?userId=8", "{}"},
		{"GET", "/admin/1?userId=8", ""},
		{"GET", "/personal/1?", ""},
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(test.method, test.path, strings.NewReader(test.body)))
		if w.Code != 400 {
			t.Errorf("invalid transport accepted: %s status=%d", test.path, w.Code)
		}
	}
}

func TestAdminFiltersRejectEmptySignedAndDuplicateIdentifiers(t *testing.T) {
	for _, raw := range []string{"userId=", "userId=%2B1", "platformId=", "platformId=%2B1", "userId=1&userId=2", "userId=9223372036854775808", "page=9223372036854775807&pageSize=100"} {
		query := raw
		if !strings.HasPrefix(raw, "page=") {
			query = "page=1&pageSize=20&" + raw
		}
		values, err := url.ParseQuery(query)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parseAdminListQuery(values); err == nil {
			t.Errorf("invalid filters accepted: %s", raw)
		}
	}
}

func TestUnimplementedOperationsAreNotAdvertisedOrRegistered(t *testing.T) {
	row := Model{ID: 1, OrderNo: "ORDER-1", UserID: 1, PlatformID: 1, PackageName: "Credits", AmountCents: 300, Status: StatusPending, ExpiresAt: time.Now().Add(time.Hour)}
	value, err := safeValue(context.Background(), row, time.Now(), true)
	if err != nil {
		t.Fatal(err)
	}
	if value.Actions.Sync || value.Actions.Close {
		t.Error("unimplemented sync/close advertised")
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	pass := func(c *gin.Context) { c.Next() }
	require := func(string) gin.HandlerFunc { return pass }
	handler := NewHandler(NewService(nil))
	RegisterAdminRoutes(router.Group("/api/admin/v1"), handler, pass, require)
	for _, route := range []string{"/api/admin/v1/payment/order/1/sync", "/api/admin/v1/payment/order/1/close", "/api/admin/v1/payment/recharge/1/sync", "/api/admin/v1/payment/recharge/1/close"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("POST", route, nil))
		if w.Code != 404 {
			t.Errorf("unimplemented route %s status=%d", route, w.Code)
		}
	}
}

func TestOrderRoutesRequireExactActionNotPagePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	pass := func(c *gin.Context) { c.Next() }
	require := func(code string) gin.HandlerFunc {
		return func(c *gin.Context) {
			if code != PermissionView && code != PersonalView {
				response.Fail(c, apperror.Forbidden(nil))
				c.Abort()
				return
			}
			c.Next()
		}
	}
	handler := NewHandler(NewService(nil))
	RegisterAdminRoutes(router.Group("/api/admin/v1"), handler, pass, require)
	for _, item := range []struct{ method, path string }{
		{"GET", "/api/admin/v1/payment/order"}, {"GET", "/api/admin/v1/payment/order/1"},
		{"GET", "/api/admin/v1/payment/recharge/options"}, {"GET", "/api/admin/v1/payment/recharge"}, {"GET", "/api/admin/v1/payment/recharge/1"}, {"POST", "/api/admin/v1/payment/recharge"}, {"POST", "/api/admin/v1/payment/recharge/1/pay"},
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(item.method, item.path, nil))
		if w.Code != 403 {
			t.Errorf("page permission allowed %s %s: %d", item.method, item.path, w.Code)
		}
	}
}

func TestPurchaseOptionsUseConfiguredChannelsAndLocalizedNumericMethods(t *testing.T) {
	service := createServiceFixture(&createRepositoryStub{})
	got, err := service.Options(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available {
		t.Fatal("configured sandbox checkout is unavailable")
	}
	for _, locale := range []i18n.Locale{i18n.ZhCN, i18n.EnUS} {
		got, err = service.Options(i18n.WithLocale(context.Background(), locale))
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(got.PayMethods)
		if err != nil {
			t.Fatal(err)
		}
		var methods []struct {
			Value payment.PayMethod `json:"value"`
			Label string            `json:"label"`
		}
		if err := json.Unmarshal(body, &methods); err != nil || len(methods) != 2 || methods[0].Value != payment.PayMethodWeb || methods[0].Label == "" || methods[1].Value != payment.PayMethodH5 {
			t.Fatalf("methods not localized numeric options: %s err=%v", body, err)
		}
	}
}

type multiChannelRuntime struct {
	rows     []paymentconfig.CatalogItem
	versions map[int64]paymentconfig.BoundCredential
}

func (r multiChannelRuntime) Catalog(context.Context) ([]paymentconfig.CatalogItem, error) {
	return r.rows, nil
}
func (r multiChannelRuntime) Credential(_ context.Context, id, _ int64) (paymentconfig.BoundCredential, error) {
	return r.versions[id], nil
}
func (r multiChannelRuntime) Credentials(ctx context.Context, id, version int64) (alipay.Credentials, error) {
	bound, err := r.Credential(ctx, id, version)
	return alipay.Credentials{AppID: bound.AppID, SellerID: bound.SellerID, Environment: bound.Environment, NotifyURL: bound.Version.NotifyURL}, err
}

// A purchase method advertised across channels must resolve to a channel that
// supports that method, not blindly to the first configured channel.
func TestCreateSelectsFirstChannelSupportingRequestedMethod(t *testing.T) {
	repo := &createRepositoryStub{}
	service := createServiceFixture(repo)
	first := paymentconfig.BoundCredential{ConfigID: 3, Provider: payment.ProviderAlipay, AppID: "20260001", SellerID: "20880001", Environment: payment.EnvironmentSandbox, Version: paymentconfig.CredentialVersion{ConfigID: 3, Version: 4, PayMethods: paymentconfig.PayMethods{payment.PayMethodWeb}, NotifyURL: "https://pay.example.test/3/4"}}
	second := first
	second.ConfigID = 5
	second.Version.ConfigID = 5
	second.Version.PayMethods = paymentconfig.PayMethods{payment.PayMethodH5}
	second.Version.NotifyURL = "https://pay.example.test/5/4"
	service.runtime = multiChannelRuntime{rows: []paymentconfig.CatalogItem{{ID: 3, Provider: payment.ProviderAlipay, Environment: payment.EnvironmentSandbox, CurrentVersion: 4, Status: yesno.Yes}, {ID: 5, Provider: payment.ProviderAlipay, Environment: payment.EnvironmentSandbox, CurrentVersion: 4, Status: yesno.Yes}}, versions: map[int64]paymentconfig.BoundCredential{3: first, 5: second}}
	_, err := service.Create(context.Background(), auth.Identity{UserID: 7, PlatformID: 1}, CreateInput{RequestID: "00000000-0000-4000-8000-000000000001", PackageID: 8, PayMethod: payment.PayMethodH5, ReturnURL: "https://app.example.test/paid"})
	if err != nil || repo.created == nil || repo.created.ConfigID != 5 {
		t.Fatalf("advertised H5 method unavailable: created=%+v err=%v", repo.created, err)
	}
}
