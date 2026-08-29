package profile

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/auth/login"
	"github.com/gin-gonic/gin"
)

type profileServiceStub struct {
	current Value
	updated Value
	actor   int64
	target  int64
	input   Input
}

func (s *profileServiceStub) Current(context.Context, int64) (Value, error) {
	return s.current, nil
}

func (s *profileServiceStub) Update(_ context.Context, actor, target int64, input Input) (Value, error) {
	s.actor, s.target, s.input = actor, target, input
	return s.updated, nil
}

type passwordServiceStub struct {
	identity auth.Identity
	input    auth.ChangePasswordInput
}

func (s *passwordServiceStub) ChangePassword(_ context.Context, identity auth.Identity, input auth.ChangePasswordInput) error {
	s.identity, s.input = identity, input
	return nil
}

func TestProfileRoutesReadAndUpdateCurrentAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	profile := &profileServiceStub{
		current: Value{UserID: 7, Username: "alice", Email: "alice@example.com", Phone: nil},
		updated: Value{UserID: 7, Username: "alice-new", Email: "alice@example.com", Phone: nil, UpdatedAt: time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)},
	}
	password := &passwordServiceStub{}
	router := gin.New()
	registerTestRoutes(router, profile, password, true)

	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/admin/v1/account/profile", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("GET status=%d body=%s", get.Code, get.Body)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(get.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(envelope["data"], &got); err != nil {
		t.Fatal(err)
	}
	if got["userId"] != float64(7) || got["email"] != "alice@example.com" {
		t.Fatalf("profile=%v", got)
	}

	put := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/admin/v1/account/profile", strings.NewReader(`{"username":"alice-new","phone":"+86 138-0000-0000","birthday":"2026-08-28","gender":1}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(put, request)
	if put.Code != http.StatusOK {
		t.Fatalf("PUT status=%d body=%s", put.Code, put.Body)
	}
	if profile.actor != 7 || profile.target != 7 || profile.input.Username != "alice-new" || profile.input.Phone == nil || profile.input.Birthday == nil || profile.input.Birthday.Format("2006-01-02") != "2026-08-28" || profile.input.Gender != 1 {
		t.Fatalf("update actor=%d target=%d input=%+v", profile.actor, profile.target, profile.input)
	}
}

func TestPasswordRoutePassesCurrentIdentityAndCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	profile := &profileServiceStub{}
	password := &passwordServiceStub{}
	router := gin.New()
	registerTestRoutes(router, profile, password, true)
	request := httptest.NewRequest(http.MethodPost, "/api/admin/v1/account/password", strings.NewReader(`{"currentPassword":"old-pass","newPassword":"new-pass","confirmPassword":"new-pass"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body)
	}
	if password.identity.UserID != 7 || password.identity.SessionID != 8 || password.identity.Platform != "admin" || password.input.CurrentPassword != "old-pass" || password.input.NewPassword != "new-pass" {
		t.Fatalf("identity=%+v input=%+v", password.identity, password.input)
	}
}

func registerTestRoutes(router *gin.Engine, profile profileService, password passwordService, authenticated bool) {
	pass := func(c *gin.Context) { c.Next() }
	actor := func(*gin.Context) (int64, bool) { return 7, authenticated }
	identity := func(c *gin.Context) {
		c.Set("auth.identity", auth.Identity{UserID: 7, SessionID: 8, PlatformID: 1, Platform: "admin"})
		c.Next()
	}
	RegisterRoutes(router.Group("/api/admin/v1"), NewHandler(profile, password, actor), func(c *gin.Context) {
		identity(c)
		pass(c)
	})
}
