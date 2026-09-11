package sms

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"admin/server/internal/authcontext"
	"github.com/gin-gonic/gin"
)

type adminServiceTest struct{}

func (adminServiceTest) PageInit(context.Context) (PageInitResult, error) {
	return PageInitResult{Scenes: []SceneOption{}}, nil
}

func (adminServiceTest) SendAdminTest(context.Context, AdminTestInput) (AdminTestResult, error) {
	return AdminTestResult{LogID: 7, Status: "sent", RequestID: "request-1", SerialNo: "serial-1"}, nil
}

func TestAdminTestResponseIncludesFinalStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		authcontext.Set(ctx, authcontext.Identity{UserID: 1, PlatformID: 2})
		ctx.Next()
	})
	pass := func(ctx *gin.Context) { ctx.Next() }
	RegisterRoutes(router.Group("/api/admin/v1/message/sms"), NewHandler(adminServiceTest{}), pass, func(string) gin.HandlerFunc { return pass })
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/v1/message/sms/test", strings.NewReader(`{"toPhone":"15671628271","scene":"login"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data["status"] != "sent" {
		t.Fatalf("data=%v, want sent status", envelope.Data)
	}
}

func TestRegisterRoutesBindsExactAggregatePermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	seen := make(map[string]int, 2)
	router := gin.New()
	RegisterRoutes(router.Group("/api/admin/v1/message/sms"), NewHandler(adminServiceTest{}),
		func(c *gin.Context) { c.Next() },
		func(code string) gin.HandlerFunc {
			return func(c *gin.Context) {
				seen[code]++
				c.AbortWithStatus(http.StatusNoContent)
			}
		})

	for _, request := range []struct{ method, path string }{
		{http.MethodGet, "/api/admin/v1/message/sms/page-init"},
		{http.MethodPost, "/api/admin/v1/message/sms/test"},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(request.method, request.path, nil))
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("%s %s status=%d", request.method, request.path, recorder.Code)
		}
	}

	if seen[PermissionList] != 1 || seen[PermissionTest] != 1 || seen[PermissionView] != 0 {
		t.Fatalf("registered permissions = %v", seen)
	}
}
