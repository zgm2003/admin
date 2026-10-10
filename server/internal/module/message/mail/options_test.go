package mail

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"admin/server/internal/shared/i18n"
	"github.com/gin-gonic/gin"
)

func TestMailOptionsAuthenticatedLocalizedAndNumeric(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, locale := range []i18n.Locale{i18n.ZhCN, i18n.EnUS} {
		router := gin.New()
		auth := func(c *gin.Context) {
			if c.GetHeader("Authorization") != "fixture" {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			c.Request = c.Request.WithContext(i18n.WithLocale(c.Request.Context(), locale))
		}
		RegisterRoutes(router.Group("/mail"), NewHandler(&Service{}), auth, func(string) gin.HandlerFunc {
			return func(c *gin.Context) { c.AbortWithStatus(http.StatusForbidden) }
		})
		unauthorized := httptest.NewRecorder()
		router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/mail/options", nil))
		if unauthorized.Code != http.StatusUnauthorized {
			t.Fatalf("unauthorized = %d", unauthorized.Code)
		}
		request := httptest.NewRequest(http.MethodGet, "/mail/options", nil)
		request.Header.Set("Authorization", "fixture")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		var envelope struct {
			Data struct {
				Scenes []struct {
					Value string `json:"value"`
					Label string `json:"label"`
				} `json:"scenes"`
				Statuses []struct {
					Value int    `json:"value"`
					Label string `json:"label"`
				} `json:"statuses"`
				RuleScopes []struct {
					Value int    `json:"value"`
					Label string `json:"label"`
				} `json:"ruleScopes"`
			} `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if len(envelope.Data.Scenes) != 4 || len(envelope.Data.Statuses) != 3 || len(envelope.Data.RuleScopes) != 2 {
			t.Fatalf("options = %+v", envelope.Data)
		}
		want := "登录验证码"
		if locale == i18n.EnUS {
			want = "Login verification code"
		}
		if envelope.Data.Scenes[0].Label != want || envelope.Data.Statuses[0].Value != 1 || envelope.Data.RuleScopes[0].Value != 0 {
			t.Fatalf("localized numeric options = %+v", envelope.Data)
		}
	}
}
