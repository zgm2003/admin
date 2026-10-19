package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	authcaptcha "admin/server/internal/module/auth/captcha"
	authclient "admin/server/internal/module/auth/client"
	auth "admin/server/internal/module/auth/login"
	notification "admin/server/internal/module/message/notification"
	callbackevent "admin/server/internal/module/payment/callbackEvent"
	order "admin/server/internal/module/payment/order"
	redeemcode "admin/server/internal/module/payment/redeemCode"
	transaction "admin/server/internal/module/payment/transaction"
	wallet "admin/server/internal/module/payment/wallet"
	storagefile "admin/server/internal/module/storage/file"
	setting "admin/server/internal/module/system/setting"
	useremail "admin/server/internal/module/user/email"
	userphone "admin/server/internal/module/user/phone"
	"admin/server/internal/module/user/profile"
	"github.com/gin-gonic/gin"
)

func platformRouteDependencies() routerDependencies {
	return routerDependencies{
		Captcha:      &authcaptcha.Handler{},
		CORSOrigin:   "http://localhost:16300",
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		AuthOrigin:   func(c *gin.Context) { c.Next() },
		Authenticate: func(c *gin.Context) { c.AbortWithStatus(http.StatusNoContent) },
		RequirePermission: func(string) gin.HandlerFunc {
			return func(c *gin.Context) { c.AbortWithStatus(http.StatusNoContent) }
		},
		Account: &profile.Handler{}, Email: &useremail.Handler{}, Phone: &userphone.Handler{},
		File: &storagefile.Handler{}, Setting: &setting.Handler{}, Notification: &notification.Handler{},
		PaymentWallet: &wallet.Handler{}, PaymentTransaction: &transaction.Handler{},
		PaymentOrder: &order.Handler{}, PaymentRedeemCode: &redeemcode.Handler{},
		PaymentCallback: callbackevent.NewHandler(nil),
	}
}

func TestCanvasRouteSurfaceDoesNotInheritAdminCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := buildRouter(platformRouteDependencies())
	var got []string
	for _, route := range router.Routes() {
		if strings.HasPrefix(route.Path, "/api/canvas/v1/") {
			got = append(got, route.Method+" "+route.Path)
		}
	}
	want := []string{
		"GET /api/canvas/v1/auth/login-config",
		"GET /api/canvas/v1/auth/captcha",
		"POST /api/canvas/v1/auth/send-code",
		"POST /api/canvas/v1/auth/login",
		"POST /api/canvas/v1/auth/password/forgot",
		"POST /api/canvas/v1/auth/password/reset",
		"POST /api/canvas/v1/auth/refresh",
		"POST /api/canvas/v1/auth/logout",
		"GET /api/canvas/v1/auth/me",
		"GET /api/canvas/v1/auth/policy",
		"GET /api/canvas/v1/access",
		"GET /api/canvas/v1/user/profile/options",
		"GET /api/canvas/v1/user/profile",
		"PUT /api/canvas/v1/user/profile",
		"POST /api/canvas/v1/user/password",
		"POST /api/canvas/v1/user/password/set",
		"POST /api/canvas/v1/user/password/send-code",
		"PUT /api/canvas/v1/user/password/by-code",
		"POST /api/canvas/v1/user/email/send-code",
		"PUT /api/canvas/v1/user/email",
		"POST /api/canvas/v1/user/phone/send-code",
		"PUT /api/canvas/v1/user/phone",
		"POST /api/canvas/v1/storage/upload-credential",
		"POST /api/canvas/v1/storage/object-url",
		"POST /api/canvas/v1/storage/file/:id/complete",
		"GET /api/canvas/v1/system/setting/legal/:document",
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Canvas route surface differs:\ngot %v\nwant %v", got, want)
	}
}

func TestPlatformPrefixesRejectCrossPlatformClientsBeforeAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		platform, header string
		status           int
	}{
		{"admin", "admin", http.StatusNoContent},
		{"canvas", "canvas", http.StatusNoContent},
		{"admin", "canvas", http.StatusForbidden},
		{"canvas", "admin", http.StatusForbidden},
		{"canvas", "portal", http.StatusForbidden},
		{"admin", "", http.StatusBadRequest},
		{"canvas", "Canvas", http.StatusBadRequest},
	} {
		t.Run(tc.platform+"/"+tc.header, func(t *testing.T) {
			deps := platformRouteDependencies()
			calls := 0
			deps.Authenticate = func(c *gin.Context) {
				calls++
				client, ok := authclient.FromContext(c)
				if !ok || client.Platform != tc.platform {
					t.Errorf("client=%+v present=%v", client, ok)
				}
				c.AbortWithStatus(http.StatusNoContent)
			}
			router := buildRouter(deps)
			req := httptest.NewRequest("GET", "/api/"+tc.platform+"/v1/access", nil)
			if tc.header != "" {
				req.Header.Set(authclient.PlatformHeader, tc.header)
			}
			req.Header.Set(authclient.DeviceIDHeader, "550e8400-e29b-41d4-a716-446655440000")
			out := httptest.NewRecorder()
			router.ServeHTTP(out, req)
			if out.Code != tc.status || (out.Code == http.StatusNoContent) != (calls == 1) {
				t.Fatalf("status=%d authCalls=%d body=%s", out.Code, calls, out.Body)
			}
		})
	}
}

func TestPlatformRoutesDoNotRegisterLegacySharedOrUnknownPrefixes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := buildRouter(platformRouteDependencies())
	for _, route := range router.Routes() {
		if strings.HasPrefix(route.Path, "/api/") && !strings.HasPrefix(route.Path, "/api/admin/v1/") && !strings.HasPrefix(route.Path, "/api/canvas/v1/") {
			t.Errorf("unscoped API remains: %s %s", route.Method, route.Path)
		}
	}
	for _, route := range []struct{ method, path string }{
		{"GET", "/api/v1/access"}, {"POST", "/api/v1/auth/login"},
		{"POST", "/api/v1/payment/callback/alipay/1/1"},
		{"GET", "/api/portal/v1/access"},
		{"GET", "/api/canvas/v1/permission/menu"},
		{"GET", "/api/canvas/v1/user/account"},
		{"GET", "/api/canvas/v1/user/account/7/email-change-log"},
		{"GET", "/api/canvas/v1/storage/file"},
		{"GET", "/api/canvas/v1/payment/order"},
		{"GET", "/api/canvas/v1/payment/recharge"},
		{"GET", "/api/canvas/v1/message/notification"},
		{"POST", "/api/canvas/v1/realtime/ticket"},
	} {
		out := httptest.NewRecorder()
		req := httptest.NewRequest(route.method, route.path, nil)
		req.Header.Set(authclient.PlatformHeader, "canvas")
		req.Header.Set(authclient.DeviceIDHeader, "550e8400-e29b-41d4-a716-446655440000")
		router.ServeHTTP(out, req)
		if out.Code != http.StatusNotFound {
			t.Errorf("%s %s = %d, want 404", route.method, route.path, out.Code)
		}
	}
}

type cookiePlatformService struct{ apiAuthService }

func (cookiePlatformService) Login(context.Context, auth.LoginInput) (auth.Credential, error) {
	return auth.Credential{AccessToken: "token", ExpiresIn: 900, RefreshToken: "refresh", RefreshExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (cookiePlatformService) Refresh(context.Context, auth.RefreshInput) (auth.Credential, error) {
	return auth.Credential{AccessToken: "new-token", ExpiresIn: 900, RefreshToken: "rotated", RefreshExpiresAt: time.Now().Add(time.Hour)}, nil
}

func TestAuthenticationCookiesStayWithinTheirPlatformPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, platform := range []string{"admin", "canvas"} {
		t.Run(platform, func(t *testing.T) {
			deps := platformRouteDependencies()
			deps.Auth = auth.NewHandler(cookiePlatformService{}, false)
			deps.Authenticate = auth.Authenticate(cookiePlatformService{})
			router := buildRouter(deps)
			for _, operation := range []string{"login", "refresh", "logout"} {
				body := ""
				if operation == "login" {
					body = `{"loginType":"password","loginAccount":"user@example.test","password":"password"}`
				}
				req := httptest.NewRequest("POST", "/api/"+platform+"/v1/auth/"+operation, strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer token")
				req.Header.Set(authclient.PlatformHeader, platform)
				req.Header.Set(authclient.DeviceIDHeader, "550e8400-e29b-41d4-a716-446655440000")
				if operation == "refresh" {
					req.AddCookie(&http.Cookie{Name: "auth_refresh_" + platform, Value: "refresh"})
				}
				out := httptest.NewRecorder()
				router.ServeHTTP(out, req)
				if out.Code != http.StatusOK {
					t.Fatalf("%s status=%d body=%s", operation, out.Code, out.Body)
				}
				cookies := out.Result().Cookies()
				if len(cookies) != 1 || cookies[0].Name != "auth_refresh_"+platform || cookies[0].Path != "/api/"+platform+"/v1/auth" || !cookies[0].HttpOnly {
					t.Fatalf("%s cookies=%+v", operation, cookies)
				}
				if operation == "logout" && cookies[0].MaxAge != -1 {
					t.Fatalf("logout did not expire platform cookie: %+v", cookies[0])
				}
			}
		})
	}
}
