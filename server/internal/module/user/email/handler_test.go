package email

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type handlerStub struct {
	send SendCodeInput
	bind BindOrChangeInput
}

func (s *handlerStub) SendCode(_ context.Context, _ Actor, input SendCodeInput) (SendCodeResult, error) {
	s.send = input
	return SendCodeResult{ChallengeID: "challenge", ResendAfterSeconds: 1}, nil
}
func (s *handlerStub) BindOrChange(_ context.Context, _ Actor, input BindOrChangeInput) (EmailResult, error) {
	s.bind = input
	return EmailResult{Email: "user@example.com"}, nil
}
func (*handlerStub) ListChangeLogs(context.Context, int64, int, int) (ChangeLogPage, error) {
	return ChangeLogPage{List: []ChangeLogItem{}}, nil
}

func TestHandlerStrictlyBindsEmailRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &handlerStub{}
	handler := NewHandler(stub, func(*gin.Context) (Actor, bool) { return actor(), true })
	router := gin.New()
	router.POST("/email/send-code", handler.SendCode)
	router.PUT("/email", handler.BindOrChange)
	req := httptest.NewRequest(http.MethodPost, "/email/send-code", strings.NewReader(`{"target":"next","email":"user@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || stub.send.Target != TargetNext || stub.send.Email == nil {
		t.Fatalf("status=%d input=%+v", rec.Code, stub.send)
	}
	req = httptest.NewRequest(http.MethodPut, "/email", strings.NewReader(`{"nextEmail":"user@example.com","nextChallengeId":"next","nextCode":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || stub.bind.NextEmail != "user@example.com" {
		t.Fatalf("status=%d input=%+v", rec.Code, stub.bind)
	}
	for _, body := range []string{`{"target":"current","email":"x@example.com"}`, `{"target":"next","email":"x@example.com","unknown":true}`} {
		req = httptest.NewRequest(http.MethodPost, "/email/send-code", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d", body, rec.Code)
		}
	}
}

func TestHandlerListsEmailChangeLogsWithStrictPathAndPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &handlerStub{}
	handler := NewHandler(stub, func(*gin.Context) (Actor, bool) { return actor(), true })
	router := gin.New()
	router.GET("/user/account/:id/email-change-log", handler.ListChangeLogs)
	req := httptest.NewRequest(http.MethodGet, "/user/account/7/email-change-log?page=1&pageSize=20", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"list":[]`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, path := range []string{"/user/account/0/email-change-log?page=1&pageSize=20", "/user/account/7/email-change-log?page=1&pageSize=101", "/user/account/7/email-change-log?page=1&pageSize=20&extra=1"} {
		req = httptest.NewRequest(http.MethodGet, path, nil)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("path=%s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
	}
}
