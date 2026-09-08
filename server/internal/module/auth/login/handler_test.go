package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/auth/client"
	authplatform "admin/server/internal/module/auth/platform"
	user "admin/server/internal/module/user/account"
	"admin/server/internal/shared/apperror"
	"github.com/gin-gonic/gin"
)

func TestRegisterReturns201AndNoCookie(t *testing.T) {
	service := &stubAuthenticationService{registered: Registered{UserID: 1, Username: "admin", Email: "admin@example.com"}}
	responseRecorder := serveAuthRoute(t, service, http.MethodPost, "/api/v1/auth/register", `{"username":"admin","email":"admin@example.com","password":"password","confirmPassword":"password"}`, nil, false)
	assertEnvelopeKeysAndCode(t, responseRecorder, http.StatusCreated, 0, []string{"userId", "username", "email"})
	if len(responseRecorder.Result().Cookies()) != 0 {
		t.Fatal("registration set a cookie")
	}
}

func TestLoginReturnsCredentialAndSecureRefreshCookie(t *testing.T) {
	fixedNow := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	refreshTTL := 14 * 24 * time.Hour
	service := &stubAuthenticationService{credential: Credential{AccessToken: "access", ExpiresIn: 900, RefreshToken: "refresh", RefreshExpiresAt: fixedNow.Add(refreshTTL), IsNewUser: true}}
	responseRecorder := serveAuthRouteAt(t, service, http.MethodPost, "/api/v1/auth/login", `{"loginType":"password","loginAccount":"admin@example.com","password":"password"}`, nil, true, fixedNow)
	assertEnvelopeKeysAndCode(t, responseRecorder, http.StatusOK, 0, []string{"accessToken", "expiresIn", "isNewUser", "passwordSetRequired"})
	assertEnvelopeDataJSON(t, responseRecorder, `{"accessToken":"access","expiresIn":900,"isNewUser":true,"passwordSetRequired":false}`)
	assertRefreshCookie(t, responseRecorder, "refresh", true, int(refreshTTL.Seconds()), fixedNow.Add(refreshTTL))
}

func TestLoginRejectsMissingAndMismatchedCredentialFields(t *testing.T) {
	for _, body := range []string{
		`{"loginType":"password","loginAccount":"admin@example.com"}`,
		`{"loginType":"password","loginAccount":"admin@example.com","password":"password","code":"123456"}`,
		`{"loginType":"email","loginAccount":"admin@example.com","password":"password"}`,
		`{"loginType":"email","loginAccount":"admin@example.com"}`,
	} {
		service := &stubAuthenticationService{credential: Credential{RefreshExpiresAt: time.Now().Add(time.Hour)}}
		responseRecorder := serveAuthRoute(t, service, http.MethodPost, "/api/v1/auth/login", body, nil, false)
		assertEnvelopeKeysAndCode(t, responseRecorder, http.StatusBadRequest, apperror.CodeInvalidRequest, nil)
	}
}

func TestSendCodeRejectsMissingRequiredFields(t *testing.T) {
	for _, body := range []string{
		`{"loginType":"email","scene":"login"}`,
		`{"account":"admin@example.com","scene":"login"}`,
		`{"account":"admin@example.com","loginType":"email"}`,
	} {
		service := &stubAuthenticationService{}
		responseRecorder := serveAuthRoute(t, service, http.MethodPost, "/api/v1/auth/send-code", body, nil, false)
		assertEnvelopeKeysAndCode(t, responseRecorder, http.StatusBadRequest, apperror.CodeInvalidRequest, nil)
		if service.sendCodeCalls != 0 {
			t.Fatalf("body %s reached service", body)
		}
	}
}

func TestSendCodeReturnsChallengeExpiryAndResendWindow(t *testing.T) {
	service := &stubAuthenticationService{sendCodeResult: SendCodeResult{ChallengeID: "challenge-1", ExpiresAt: time.Date(2026, time.September, 7, 10, 5, 0, 0, time.UTC), ResendAfterSeconds: 60}}
	responseRecorder := serveAuthRoute(t, service, http.MethodPost, "/api/v1/auth/send-code", `{"account":"admin@example.com","loginType":"email","scene":"login"}`, nil, false)
	assertEnvelopeKeysAndCode(t, responseRecorder, http.StatusOK, 0, []string{"challengeId", "expiresAt", "resendAfterSeconds"})
	assertEnvelopeDataJSON(t, responseRecorder, `{"challengeId":"challenge-1","expiresAt":"2026-09-07T10:05:00Z","resendAfterSeconds":60}`)
}

func TestSendCodeRejectsOversizedChallengeID(t *testing.T) {
	service := &stubAuthenticationService{}
	body := `{"account":"admin@example.com","loginType":"email","scene":"login","challengeId":"` + strings.Repeat("a", 129) + `"}`
	responseRecorder := serveAuthRoute(t, service, http.MethodPost, "/api/v1/auth/send-code", body, nil, false)
	assertEnvelopeKeysAndCode(t, responseRecorder, http.StatusBadRequest, apperror.CodeInvalidRequest, nil)
	if service.sendCodeCalls != 0 {
		t.Fatal("oversized challenge id reached service")
	}
}

func TestLoginConfigUsesLowerCamelCaseOptionFields(t *testing.T) {
	service := &stubAuthenticationService{loginConfig: authplatform.LoginConfig{
		LoginTypes: []authplatform.LoginTypeOption{{Value: authplatform.LoginTypePassword}},
	}}
	responseRecorder := serveAuthRoute(t, service, http.MethodGet, "/api/v1/auth/login-config", "", nil, false)
	assertEnvelopeKeysAndCode(t, responseRecorder, http.StatusOK, 0, []string{"loginTypes", "allowRegister"})
	var envelope struct {
		Data struct {
			LoginTypes []map[string]json.RawMessage `json:"loginTypes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.LoginTypes) != 1 || envelope.Data.LoginTypes[0]["value"] == nil || envelope.Data.LoginTypes[0]["label"] == nil {
		t.Fatalf("login config option fields = %s", responseRecorder.Body.String())
	}
}

func TestLoginAcceptsOnlyLoginTypeAndAccount(t *testing.T) {
	service := &stubAuthenticationService{credential: Credential{
		AccessToken: "access", ExpiresIn: 900, RefreshToken: "refresh", RefreshExpiresAt: time.Now().Add(time.Hour),
	}}
	success := serveAuthRoute(t, service, http.MethodPost, "/api/v1/auth/login", `{"loginType":"password","loginAccount":" Admin@Example.COM ","password":"password"}`, nil, false)
	assertEnvelopeKeysAndCode(t, success, http.StatusOK, 0, []string{"accessToken", "expiresIn", "isNewUser", "passwordSetRequired"})
	if service.loginInput.LoginAccount != " Admin@Example.COM " || service.loginInput.LoginType != authplatform.LoginTypePassword {
		t.Fatalf("handler changed login input before service: %+v", service.loginInput)
	}

	legacy := serveAuthRoute(t, service, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"password"}`, nil, false)
	assertEnvelopeKeysAndCode(t, legacy, http.StatusBadRequest, apperror.CodeInvalidRequest, nil)
}

func TestRefreshRejectsEveryNonEmptyBody(t *testing.T) {
	for _, body := range []string{" ", "{}", `{"value":1}`} {
		service := &stubAuthenticationService{}
		responseRecorder := serveAuthRoute(t, service, http.MethodPost, "/api/v1/auth/refresh", body, &http.Cookie{Name: refreshCookieName("admin"), Value: "refresh"}, false)
		if responseRecorder.Code != http.StatusBadRequest || service.refreshCalls != 0 {
			t.Errorf("body %q status=%d calls=%d", body, responseRecorder.Code, service.refreshCalls)
		}
	}
}

func TestRefreshRotatesCookieWithRemainingLifetime(t *testing.T) {
	fixedNow := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	service := &stubAuthenticationService{credential: Credential{AccessToken: "new-access", ExpiresIn: 900, RefreshToken: "new-refresh", RefreshExpiresAt: fixedNow.Add(30 * time.Minute)}}
	responseRecorder := serveAuthRouteAt(t, service, http.MethodPost, "/api/v1/auth/refresh", "", &http.Cookie{Name: refreshCookieName("admin"), Value: "old-refresh"}, false, fixedNow)
	assertEnvelopeKeysAndCode(t, responseRecorder, http.StatusOK, 0, []string{"accessToken", "expiresIn", "isNewUser", "passwordSetRequired"})
	assertRefreshCookie(t, responseRecorder, "new-refresh", false, 1800, fixedNow.Add(30*time.Minute))
	if service.refreshInput.RefreshToken != "old-refresh" {
		t.Fatalf("Refresh input = %+v", service.refreshInput)
	}
}

func TestLogoutExpiresCookieEvenWhenRedisDeleteFails(t *testing.T) {
	service := &stubAuthenticationService{
		authenticateIdentity: Identity{UserID: 1, SessionID: 2, Platform: "admin", Version: 1},
		logoutErr:            apperror.DependencyUnavailable(errors.New("redis down")),
	}
	headers := map[string]string{"Authorization": "Bearer token"}
	responseRecorder := serveAuthRouteWithHeaders(t, service, http.MethodPost, "/api/v1/auth/logout", "", &http.Cookie{Name: refreshCookieName("admin"), Value: "refresh"}, false, time.Now(), headers)
	assertEnvelopeKeysAndCode(t, responseRecorder, http.StatusServiceUnavailable, apperror.CodeDependencyUnavailable, nil)
	cookies := responseRecorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != refreshCookieName("admin") || cookies[0].MaxAge >= 0 || cookies[0].Path != refreshCookiePath || !cookies[0].HttpOnly {
		t.Fatalf("expired cookie = %+v", cookies)
	}
}

func TestLogoutExpiresOnlySelectedPlatformCookie(t *testing.T) {
	service := &stubAuthenticationService{
		authenticateIdentity: Identity{UserID: 1, SessionID: 2, Platform: "app", Version: 1},
	}
	headers := map[string]string{
		"Authorization":           "Bearer token",
		authclient.PlatformHeader: "app",
	}
	responseRecorder := serveAuthRouteWithHeaders(t, service, http.MethodPost, "/api/v1/auth/logout", "", &http.Cookie{Name: refreshCookieName("app"), Value: "refresh"}, false, time.Now(), headers)
	assertEnvelopeKeysAndCode(t, responseRecorder, http.StatusOK, 0, nil)
	cookies := responseRecorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != refreshCookieName("app") || cookies[0].MaxAge >= 0 {
		t.Fatalf("expired cookie = %+v", cookies)
	}
}

func TestMeReturnsClosedCurrentUserShape(t *testing.T) {
	phone := "+86 138-0000-0000"
	for _, test := range []struct {
		name     string
		phone    *string
		wantJSON string
	}{
		{name: "null phone", wantJSON: `{"userId":1,"username":"admin","email":"admin@example.com","phone":null,"avatar":"avatar/profile.png","passwordSetRequired":false}`},
		{name: "stored phone", phone: &phone, wantJSON: `{"userId":1,"username":"admin","email":"admin@example.com","phone":"+86 138-0000-0000","avatar":"avatar/profile.png","passwordSetRequired":false}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &stubAuthenticationService{
				authenticateIdentity: Identity{UserID: 1, SessionID: 2, Platform: "admin", Version: 1},
				current:              user.Current{ID: 1, Username: "admin", Email: "admin@example.com", Phone: test.phone, Avatar: "avatar/profile.png"},
			}
			headers := map[string]string{"Authorization": "Bearer token"}
			responseRecorder := serveAuthRouteWithHeaders(t, service, http.MethodGet, "/api/v1/auth/me", "", nil, false, time.Now(), headers)
			assertEnvelopeDataJSON(t, responseRecorder, test.wantJSON)
		})
	}
}

func TestAuthHandlersRejectUnknownJSONFields(t *testing.T) {
	for _, route := range []string{"/api/v1/auth/register", "/api/v1/auth/login", "/api/v1/auth/password/forgot"} {
		service := &stubAuthenticationService{}
		body := `{"email":"admin@example.com","password":"password","unknown":true}`
		if strings.HasSuffix(route, "register") {
			body = `{"username":"admin","email":"admin@example.com","password":"password","confirmPassword":"password","unknown":true}`
		}
		responseRecorder := serveAuthRoute(t, service, http.MethodPost, route, body, nil, false)
		if responseRecorder.Code != http.StatusBadRequest {
			t.Errorf("route %s status=%d", route, responseRecorder.Code)
		}
	}
}

func TestAuthHandlersPassExactClientMetadata(t *testing.T) {
	service := &stubAuthenticationService{registered: Registered{UserID: 1}, credential: Credential{AccessToken: "access", ExpiresIn: 900, RefreshToken: "refresh", RefreshExpiresAt: time.Now().Add(time.Hour)}}
	serveAuthRoute(t, service, http.MethodPost, "/api/v1/auth/register", `{"username":"admin","email":"admin@example.com","password":"password","confirmPassword":"password"}`, nil, false)
	serveAuthRoute(t, service, http.MethodPost, "/api/v1/auth/login", `{"loginType":"password","loginAccount":"admin@example.com","password":"password"}`, nil, false)
	serveAuthRoute(t, service, http.MethodPost, "/api/v1/auth/refresh", "", &http.Cookie{Name: refreshCookieName("admin"), Value: "refresh"}, false)
	want := authclient.Client{Platform: "admin", DeviceID: "550e8400-e29b-41d4-a716-446655440000", ClientIP: "192.0.2.1", UserAgent: ""}
	for name, got := range map[string]authclient.Client{
		"register": service.registerInput.Client,
		"login":    service.loginInput.Client,
		"refresh":  service.refreshInput.Client,
	} {
		if got != want {
			t.Errorf("%s client = %+v, want %+v", name, got, want)
		}
	}
}

func TestForgotPasswordRejectsMissingRequiredFields(t *testing.T) {
	service := &stubAuthenticationService{}
	responseRecorder := serveAuthRoute(t, service, http.MethodPost, "/api/v1/auth/password/forgot", `{}`, nil, false)
	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if service.sendCodeCalls != 0 {
		t.Fatalf("service reached with invalid body, calls=%d", service.sendCodeCalls)
	}
}

func TestForgotPasswordReturnsChallengeExpiryAndResendWindow(t *testing.T) {
	fixedNow := time.Date(2026, time.September, 8, 9, 0, 0, 0, time.UTC)
	service := &stubAuthenticationService{sendCodeResult: SendCodeResult{ChallengeID: "challenge-1", ExpiresAt: fixedNow.Add(5 * time.Minute), ResendAfterSeconds: 60}}
	responseRecorder := serveAuthRouteAt(t, service, http.MethodPost, "/api/v1/auth/password/forgot", `{"email":"user@example.com"}`, nil, false, fixedNow)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if !strings.Contains(responseRecorder.Body.String(), `"challengeId":"challenge-1"`) ||
		!strings.Contains(responseRecorder.Body.String(), `"resendAfterSeconds":60`) {
		t.Fatalf("body=%s", responseRecorder.Body.String())
	}
}

func TestPlatformRefreshCookiesDoNotOverwriteEachOther(t *testing.T) {
	fixedNow := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	service := &stubAuthenticationService{credential: Credential{AccessToken: "access", ExpiresIn: 900, RefreshToken: "refresh", RefreshExpiresAt: fixedNow.Add(time.Hour)}}
	admin := serveAuthRouteAt(t, service, http.MethodPost, "/api/v1/auth/login", `{"loginType":"password","loginAccount":"admin@example.com","password":"password"}`, nil, false, fixedNow)
	if got := admin.Result().Cookies()[0].Name; got != "admin_refresh_admin" {
		t.Fatalf("admin cookie = %q", got)
	}

	gin.SetMode(gin.TestMode)
	handler := NewHandler(service, false)
	handler.now = func() time.Time { return fixedNow }
	router := gin.New()
	apiRoutes := router.Group("/api/v1", authclient.Require())
	RegisterRoutes(apiRoutes, handler, RequireOrigin("http://localhost:16300"), Authenticate(service))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"loginType":"password","loginAccount":"admin@example.com","password":"password"}`))
	request.Header.Set("Origin", "http://localhost:16300")
	request.Header[authclient.PlatformHeader] = []string{"app"}
	request.Header[authclient.DeviceIDHeader] = []string{"550e8400-e29b-41d4-a716-446655440000"}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if got := recorder.Result().Cookies()[0].Name; got != "admin_refresh_app" {
		t.Fatalf("app cookie = %q", got)
	}
}

type stubAuthenticationService struct {
	registered           Registered
	registerErr          error
	credential           Credential
	loginErr             error
	refreshErr           error
	refreshCalls         int
	refreshInput         RefreshInput
	registerInput        RegisterInput
	loginInput           LoginInput
	loginConfig          authplatform.LoginConfig
	sendCodeCalls        int
	sendCodeResult       SendCodeResult
	authenticateIdentity Identity
	authenticateErr      error
	authenticateCalls    int
	authenticateToken    string
	authenticateClient   authclient.Client
	logoutErr            error
	current              user.Current
	currentErr           error
}

func (s *stubAuthenticationService) Register(_ context.Context, input RegisterInput) (Registered, error) {
	s.registerInput = input
	return s.registered, s.registerErr
}

func (s *stubAuthenticationService) Login(_ context.Context, input LoginInput) (Credential, error) {
	s.loginInput = input
	return s.credential, s.loginErr
}

func (s *stubAuthenticationService) LoginConfig(_ context.Context, _ authclient.Client) (authplatform.LoginConfig, error) {
	return s.loginConfig, nil
}

func (s *stubAuthenticationService) SendCode(_ context.Context, _ SendCodeInput) (SendCodeResult, error) {
	s.sendCodeCalls++
	return s.sendCodeResult, nil
}

func (s *stubAuthenticationService) ForgotPassword(_ context.Context, _ ForgotPasswordInput) (SendCodeResult, error) {
	s.sendCodeCalls++
	return s.sendCodeResult, nil
}

func (s *stubAuthenticationService) ResetPassword(_ context.Context, _ ResetPasswordInput) error {
	s.sendCodeCalls++
	return nil
}

func (s *stubAuthenticationService) Refresh(_ context.Context, input RefreshInput) (Credential, error) {
	s.refreshCalls++
	s.refreshInput = input
	return s.credential, s.refreshErr
}

func (s *stubAuthenticationService) Authenticate(_ context.Context, token string, client authclient.Client) (Identity, error) {
	s.authenticateCalls++
	s.authenticateToken = token
	s.authenticateClient = client
	return s.authenticateIdentity, s.authenticateErr
}

func (s *stubAuthenticationService) Logout(context.Context, Identity, authclient.Client) error {
	return s.logoutErr
}

func (s *stubAuthenticationService) CurrentUser(context.Context, Identity) (user.Current, error) {
	return s.current, s.currentErr
}

func serveAuthRoute(t *testing.T, service *stubAuthenticationService, method, path, body string, cookie *http.Cookie, secure bool) *httptest.ResponseRecorder {
	t.Helper()
	return serveAuthRouteAt(t, service, method, path, body, cookie, secure, time.Now())
}

func serveAuthRouteAt(t *testing.T, service *stubAuthenticationService, method, path, body string, cookie *http.Cookie, secure bool, now time.Time) *httptest.ResponseRecorder {
	t.Helper()
	return serveAuthRouteWithHeaders(t, service, method, path, body, cookie, secure, now, nil)
}

func serveAuthRouteWithHeaders(t *testing.T, service *stubAuthenticationService, method, path, body string, cookie *http.Cookie, secure bool, now time.Time, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := NewHandler(service, secure)
	handler.now = func() time.Time { return now }
	router := gin.New()
	apiRoutes := router.Group("/api/v1", authclient.Require())
	RegisterRoutes(apiRoutes, handler, RequireOrigin("http://localhost:16300"), Authenticate(service))
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Origin", "http://localhost:16300")
	request.Header.Set("Content-Type", "application/json")
	request.Header[authclient.PlatformHeader] = []string{"admin"}
	request.Header[authclient.DeviceIDHeader] = []string{"550e8400-e29b-41d4-a716-446655440000"}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	responseRecorder := httptest.NewRecorder()
	router.ServeHTTP(responseRecorder, request)
	return responseRecorder
}

func assertEnvelopeKeysAndCode(t *testing.T, responseRecorder *httptest.ResponseRecorder, wantStatus, wantCode int, dataKeys []string) {
	t.Helper()
	if responseRecorder.Code != wantStatus {
		t.Fatalf("status=%d body=%s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope) != 3 || envelope["code"] == nil || envelope["data"] == nil || envelope["message"] == nil {
		t.Fatalf("envelope keys = %v", envelope)
	}
	var code int
	if err := json.Unmarshal(envelope["code"], &code); err != nil || code != wantCode {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if dataKeys != nil {
		var data map[string]json.RawMessage
		if err := json.Unmarshal(envelope["data"], &data); err != nil {
			t.Fatal(err)
		}
		if len(data) != len(dataKeys) {
			t.Fatalf("data keys = %v", data)
		}
		for _, key := range dataKeys {
			if data[key] == nil {
				t.Fatalf("data missing %q: %v", key, data)
			}
		}
	}
}

func assertEnvelopeDataJSON(t *testing.T, responseRecorder *httptest.ResponseRecorder, want string) {
	t.Helper()
	assertEnvelopeKeysAndCode(t, responseRecorder, http.StatusOK, 0, nil)
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	var gotValue, wantValue any
	if err := json.Unmarshal(envelope.Data, &gotValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("data=%s want=%s", envelope.Data, want)
	}
}

func assertRefreshCookie(t *testing.T, responseRecorder *httptest.ResponseRecorder, value string, secure bool, maxAge int, expires time.Time) {
	t.Helper()
	cookies := responseRecorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %+v", cookies)
	}
	cookie := cookies[0]
	if cookie.Name != refreshCookieName("admin") || cookie.Value != value || cookie.Path != refreshCookiePath || cookie.Domain != "" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Secure != secure || cookie.MaxAge != maxAge || !cookie.Expires.Equal(expires) {
		t.Fatalf("refresh cookie = %+v", cookie)
	}
}
