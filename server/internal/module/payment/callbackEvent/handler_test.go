package callbackevent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	authclient "admin/server/internal/module/auth/client"
	operationlog "admin/server/internal/module/system/operationLog"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type callbackHTTPService struct {
	calls             int
	receiveErr        error
	ctx               context.Context
	configID, version int64
}

func (s *callbackHTTPService) Receive(ctx context.Context, configID, version int64, _ url.Values) (ReceiveResult, error) {
	s.calls++
	s.ctx = ctx
	s.configID, s.version = configID, version
	return ReceiveResult{EventID: 1}, s.receiveErr
}
func (*callbackHTTPService) ListByOrder(context.Context, int64, int, int) (pagination.Result[Value], error) {
	return pagination.Result[Value]{List: []Value{}}, nil
}
func (*callbackHTTPService) GetForOrder(context.Context, int64, int64) (Value, error) {
	return Value{}, apperror.NotFound(nil)
}

type callbackLogEnqueuer struct{ calls int }

func (e *callbackLogEnqueuer) Enqueue(context.Context, operationlog.TaskPayload) error {
	e.calls++
	return nil
}

func TestProviderCallbackIsAnonymousExactTextAndNotOperationPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newEventFixture(t)
	router := gin.New()
	logs := &callbackLogEnqueuer{}
	router.Use(operationlog.Middleware(slog.New(slog.NewTextHandler(io.Discard, nil)), logs))
	protected := router.Group("/api/admin/v1")
	protected.Use(authclient.Require())
	protected.GET("/protected", func(c *gin.Context) { c.Status(200) })
	RegisterAdminProviderRoutes(router.Group("/api/admin/v1"), NewHandler(f.service))
	form := f.signed(t, func(v url.Values) { v.Set("passback_params", "TOKEN=PRIVATE_TOKEN") })
	for _, test := range []struct {
		name string
		fail bool
	}{{"committed", false}, {"invalid signature", true}} {
		t.Run(test.name, func(t *testing.T) {
			body := form.Encode()
			if test.fail {
				form.Set("sign", "INVALID_SIGNATURE")
				body = form.Encode()
			}
			req := httptest.NewRequest("POST", "/api/admin/v1/payment/callback/alipay/3/4", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			want := "success"
			if test.fail {
				want = "failure"
			}
			if w.Body.String() != want || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain") {
				t.Fatalf("provider response status=%d type=%s body=%q", w.Code, w.Header().Get("Content-Type"), w.Body.String())
			}
			if !test.fail && w.Code != 200 {
				t.Fatalf("anonymous callback requires client/authorization: %d", w.Code)
			}
		})
	}
	if logs.calls != 0 {
		t.Fatal("provider request was captured as a business operation payload")
	}
}

func TestProviderCallbackRejectsTransportAmbiguityBeforeService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	base := "notify_id=N&out_trade_no=ORDER&trade_no=TRADE&app_id=APP&seller_id=SELLER&trade_status=TRADE_SUCCESS&total_amount=3.00&sign_type=RSA2&sign=S"
	fields := make(url.Values)
	for i := 0; i < 65; i++ {
		fields.Set(fmt.Sprintf("field%d", i), "x")
	}
	for _, test := range []struct {
		name, path, contentType, encoding, body string
		chunked                                 bool
	}{
		{"missing type", "/3/4", "", "", base, false},
		{"wrong type", "/3/4", "application/json", "", base, false},
		{"compressed", "/3/4", "application/x-www-form-urlencoded", "gzip", base, false},
		{"query mix", "/3/4?app_id=OTHER", "application/x-www-form-urlencoded", "", base, false},
		{"empty query marker", "/3/4?", "application/x-www-form-urlencoded", "", base, false},
		{"duplicate", "/3/4", "application/x-www-form-urlencoded", "", base + "&total_amount=3.00", false},
		{"malformed escape", "/3/4", "application/x-www-form-urlencoded", "", base + "&extra=%zz", false},
		{"semicolon", "/3/4", "application/x-www-form-urlencoded", "", base + ";extra=x", false},
		{"oversized known length", "/3/4", "application/x-www-form-urlencoded", "", "extra=" + strings.Repeat("x", 65536), false},
		{"oversized chunked", "/3/4", "application/x-www-form-urlencoded", "", "extra=" + strings.Repeat("x", 65536), true},
		{"too many fields", "/3/4", "application/x-www-form-urlencoded", "", fields.Encode(), false},
		{"empty body", "/3/4", "application/x-www-form-urlencoded", "", "", false},
		{"signed path coordinate", "/+3/4", "application/x-www-form-urlencoded", "", base, false},
		{"overflow version", "/3/9223372036854775808", "application/x-www-form-urlencoded", "", base, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &callbackHTTPService{}
			router := gin.New()
			RegisterAdminProviderRoutes(router.Group("/api/admin/v1"), NewHandler(service))
			req := httptest.NewRequest("POST", "/api/admin/v1/payment/callback/alipay"+test.path, strings.NewReader(test.body))
			req.Header.Set("Content-Type", test.contentType)
			if test.encoding != "" {
				req.Header.Set("Content-Encoding", test.encoding)
			}
			if test.chunked {
				req.ContentLength = -1
				req.TransferEncoding = []string{"chunked"}
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if service.calls != 0 || w.Body.String() != "failure" || w.Code < 400 {
				t.Fatalf("ambiguous transport reached service: calls=%d status=%d body=%q", service.calls, w.Code, w.Body.String())
			}
		})
	}
}

func TestProviderCallbackPreservesRequestContextAndSanitizesErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &callbackHTTPService{receiveErr: apperror.DependencyUnavailable(errors.New("PRIVATE_TOKEN SQL DSN"))}
	router := gin.New()
	RegisterAdminProviderRoutes(router.Group("/api/admin/v1"), NewHandler(service))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("POST", "/api/admin/v1/payment/callback/alipay/3/4", strings.NewReader("sign=x")).WithContext(ctx)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if service.ctx != ctx || service.configID != 3 || service.version != 4 || w.Code != http.StatusServiceUnavailable || w.Body.String() != "failure" {
		t.Fatalf("response status=%d body=%q ctx=%v", w.Code, w.Body.String(), service.ctx == ctx)
	}
}

func TestCallbackAuditRoutesRequireIndependentActionsAndParentScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	pass := func(c *gin.Context) { c.Next() }
	RegisterAdminRoutes(router.Group("/api/admin/v1"), NewHandler(&callbackHTTPService{}), pass, func(code string) gin.HandlerFunc {
		return func(c *gin.Context) {
			if code != "payment:order:detail" {
				response.Fail(c, apperror.Forbidden(nil))
				c.Abort()
				return
			}
			c.Next()
		}
	})
	for _, path := range []string{"/api/admin/v1/payment/order/11/callback?page=1&pageSize=20", "/api/admin/v1/payment/order/11/callback/1"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 403 {
			t.Fatalf("order permission leaked audit capability: %d", w.Code)
		}
	}
	f := newEventFixture(t)
	row := validEventRow(t)
	id := int64(11)
	row.OrderID = &id
	f.repo.rows = map[int64]Model{1: row}
	if _, err := f.service.GetForOrder(context.Background(), 12, 1); err == nil {
		t.Fatal("callback detail escaped parent order")
	}
	if got, err := f.service.GetForOrder(context.Background(), 11, 1); err != nil || got.ID != 1 {
		t.Fatalf("valid parent audit inaccessible: %+v err=%v", got, err)
	}
}
