package config

import (
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/response"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigActionRoutesAreIndependent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(NewService(nil, nil))
	router := gin.New()
	RegisterAdminRoutes(router.Group("/api/admin/v1"), handler, func(c *gin.Context) { c.Next() }, func(code string) gin.HandlerFunc {
		return func(c *gin.Context) {
			if code != "payment:config:view" {
				response.Fail(c, apperror.Forbidden(nil))
				c.Abort()
				return
			}
			c.Next()
		}
	})
	for _, r := range []struct{ method, path string }{{"GET", ""}, {"GET", "/1"}, {"POST", ""}, {"PUT", "/1"}, {"POST", "/1/version"}, {"PATCH", "/1/status"}, {"DELETE", "/1"}, {"POST", "/1/test"}} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(r.method, "/api/admin/v1/payment/config"+r.path, bytes.NewBufferString("{}")))
		if w.Code != 403 {
			t.Fatalf("view allowed %s %s: %d", r.method, r.path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/admin/v1/payment/config/options", nil))
	if w.Code != 200 {
		t.Fatal("static options require data access")
	}
	var out struct{ Data Options }
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.Data.Providers) != 1 || out.Data.Providers[0].Value != 1 {
		t.Fatal("options advertised unsupported capability")
	}
	if out.Data.Defaults.NotifyURL != "https://HOST/api/admin/v1/payment/callback/alipay/{configId}/{version}" {
		t.Fatalf("creation options do not prefill callback URL: %q", out.Data.Defaults.NotifyURL)
	}
}

func TestConfigCreateReportsInvalidNotifyURLWithoutEchoingInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	input := map[string]any{
		"name": "Local sandbox", "provider": 1, "environment": 1,
		"appId": "20260001", "sellerId": "20880001", "sortOrder": 0, "status": 1,
		"notifyUrl": "http://abc", "payMethods": []int{1, 2},
		"privateKey": "LOCAL_PRIVATE_KEY", "appCertPEM": "LOCAL_APP_CERT",
		"alipayCertPEM": "LOCAL_ALIPAY_CERT", "rootCertPEM": "LOCAL_ROOT_CERT",
	}
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, locale := range []i18n.Locale{i18n.ZhCN, i18n.EnUS} {
		t.Run(string(locale), func(t *testing.T) {
			router := gin.New()
			router.POST("/config", NewHandler(NewService(nil, nil)).Create)
			request := httptest.NewRequest("POST", "/config", bytes.NewReader(body))
			request = request.WithContext(i18n.WithLocale(request.Context(), locale))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, request)
			assertConfigValidationResponse(t, w, "notifyUrl", "HTTPS", "{configId}/{version}")
			for _, secret := range []string{"http://abc", "LOCAL_PRIVATE_KEY", "LOCAL_APP_CERT", "LOCAL_ALIPAY_CERT", "LOCAL_ROOT_CERT"} {
				if strings.Contains(w.Body.String(), secret) {
					t.Fatal("validation response echoed input")
				}
			}
		})
	}
}

func TestConfigCreateReportsCredentialFieldWithoutEchoingInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	keys, err := secretkey.New(strings.Repeat("p", 64))
	if err != nil {
		t.Fatal(err)
	}
	fixture := validInput(t)
	for _, test := range []struct{ field, value string }{
		{"privateKey", "LOCAL_SECRET=a"},
		{"appCertPEM", "LOCAL_APP_CERT"},
		{"alipayCertPEM", "LOCAL_ALIPAY_CERT"},
		{"rootCertPEM", "LOCAL_ROOT_CERT"},
	} {
		t.Run(test.field, func(t *testing.T) {
			input := map[string]any{
				"name": fixture.Name, "provider": fixture.Provider, "environment": fixture.Environment,
				"appId": fixture.AppID, "sellerId": fixture.SellerID, "sortOrder": 0, "status": 0,
				"notifyUrl": fixture.NotifyURL, "payMethods": fixture.PayMethods,
				"privateKey": fixture.PrivateKey, "appCertPEM": fixture.AppCertPEM,
				"alipayCertPEM": fixture.AlipayCertPEM, "rootCertPEM": fixture.RootCertPEM,
			}
			input[test.field] = test.value
			body, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			router := gin.New()
			router.POST("/config", NewHandler(NewService(nil, keys)).Create)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("POST", "/config", bytes.NewReader(body)))
			assertConfigValidationResponse(t, w, test.field)
			for _, secret := range []string{test.value, fixture.PrivateKey, "BEGIN CERTIFICATE", "BEGIN RSA PRIVATE KEY"} {
				if strings.Contains(w.Body.String(), secret) {
					t.Fatal("validation response leaked a credential")
				}
			}
		})
	}
}

func assertConfigValidationResponse(t *testing.T, w *httptest.ResponseRecorder, fragments ...string) {
	t.Helper()
	var response struct {
		Code    int
		Data    any
		Message string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if w.Code != 400 || response.Code != 10001 || response.Data != nil {
		t.Fatalf("invalid error envelope: %d %s", w.Code, w.Body.String())
	}
	for _, fragment := range fragments {
		if !strings.Contains(response.Message, fragment) {
			t.Fatalf("missing %q in validation message %q", fragment, response.Message)
		}
	}
}

func TestConfigCreateDistinguishesCertificateMismatch(t *testing.T) {
	keys, err := secretkey.New(strings.Repeat("p", 64))
	if err != nil {
		t.Fatal(err)
	}
	fixture := validInput(t)
	for _, test := range []struct {
		name, messageKey, field string
		change                  func(*CreateInput)
	}{
		{"application identity", "payment.config.appCertIdentityMismatch", "APPID", func(c *CreateInput) { c.AppID = "20269999" }},
		{"private key mismatch", "payment.config.privateKeyMismatch", "privateKey", func(c *CreateInput) { c.PrivateKey = validInput(t).PrivateKey }},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := fixture
			test.change(&input)
			_, err := NewService(nil, keys).Create(context.Background(), input)
			var public *apperror.Error
			if !errors.As(err, &public) || string(public.MessageKey) != test.messageKey {
				t.Fatalf("wrong validation semantic: %v", err)
			}
			for _, locale := range []i18n.Locale{i18n.ZhCN, i18n.EnUS} {
				message, err := i18n.Translate(locale, public.MessageKey, public.Params)
				if err != nil || !strings.Contains(message, test.field) {
					t.Fatalf("missing field explanation: %q %v", message, err)
				}
			}
		})
	}
}
func TestConfigHandlerRejectsMalformedWithoutRepository(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterAdminRoutes(router.Group("/api/admin/v1"), NewHandler(NewService(nil, nil)), func(c *gin.Context) { c.Next() }, func(string) gin.HandlerFunc { return func(c *gin.Context) { c.Next() } })
	for _, r := range []struct{ method, path, body string }{{"POST", "", `{"provider":"1"}`}, {"POST", "", `{"provider":1,"provider":2}`}, {"POST", "/1/version", `{"notifyUrl":"x","payMethods":[1]}`}, {"PUT", "/1", `{"name":"x","sortOrder":0,"provider":1}`}, {"PUT", "/0", `{}`}, {"PATCH", "/1/status", `{"status":"1"}`}, {"DELETE", "/1", `{}`}, {"GET", "?page=1&pageSize=20&status=01", ``}} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(r.method, "/api/admin/v1/payment/config"+r.path, bytes.NewBufferString(r.body)))
		if w.Code != 400 {
			t.Fatalf("malformed %s %s: %d %s", r.method, r.path, w.Code, w.Body.String())
		}
		if !bytes.Contains(w.Body.Bytes(), []byte(`"code":10001`)) {
			t.Fatal("invalid envelope")
		}
	}
}
