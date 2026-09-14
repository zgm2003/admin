package queuemonitor

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type issuingServiceStub struct {
	result IssuedGrant
	err    error
}

func (s issuingServiceStub) Issue(context.Context, Subject) (IssuedGrant, error) {
	return s.result, s.err
}

func TestHandlerGrantSetsScopedHTTPOnlyCookieWithoutCredentialBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	expires := time.Now().UTC().Add(GrantTTL)
	h := NewHandler(issuingServiceStub{result: IssuedGrant{credential: "secret-grant", ExpiresAt: expires}}, false, func(*gin.Context) (Subject, bool) {
		return Subject{UserID: 1, PlatformID: 2}, true
	})
	router := gin.New()
	router.POST("/grant", h.Grant)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/grant", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != CookieName || cookie.Value != "secret-grant" || cookie.Path != UIPath || !cookie.HttpOnly || cookie.Secure || cookie.MaxAge != 60 || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie = %+v", cookie)
	}
	if strings.Contains(recorder.Body.String(), "secret-grant") {
		t.Fatal("response leaks grant")
	}
	var body struct {
		Code int `json:"code"`
		Data struct {
			ExpiresAt string `json:"expiresAt"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || body.Code != 0 || body.Data.ExpiresAt == "" {
		t.Fatalf("body = %s err=%v", recorder.Body, err)
	}
}

func TestHandlerGrantFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(issuingServiceStub{err: errors.New("redis unavailable")}, true, func(*gin.Context) (Subject, bool) { return Subject{UserID: 1, PlatformID: 2}, true })
	router := gin.New()
	router.POST("/grant", h.Grant)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/grant", nil))
	if recorder.Code != http.StatusServiceUnavailable || len(recorder.Result().Cookies()) != 0 {
		t.Fatalf("status=%d cookies=%v", recorder.Code, recorder.Result().Cookies())
	}
}
