package scheduler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"admin/server/internal/shared/i18n"
	"github.com/gin-gonic/gin"
)

func TestSchedulerOptionsAuthenticatedLocalizedWithoutRepository(t *testing.T) {
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
		RegisterRoutes(router.Group(""), NewHandler(NewService(nil, TaskCatalog{})), auth, func(string) gin.HandlerFunc {
			return func(c *gin.Context) { c.AbortWithStatus(http.StatusForbidden) }
		})
		unauthorized := httptest.NewRecorder()
		router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/system/scheduler/options", nil))
		if unauthorized.Code != http.StatusUnauthorized {
			t.Fatalf("unauthorized = %d", unauthorized.Code)
		}
		request := httptest.NewRequest(http.MethodGet, "/system/scheduler/options", nil)
		request.Header.Set("Authorization", "fixture")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		var envelope struct {
			Data struct {
				Tasks       []json.RawMessage `json:"tasks"`
				JobStatuses []struct {
					Value int    `json:"value"`
					Label string `json:"label"`
				} `json:"jobStatuses"`
				RunStatuses []struct {
					Value int    `json:"value"`
					Label string `json:"label"`
				} `json:"runStatuses"`
				CronPresets []struct {
					Value string `json:"value"`
					Label string `json:"label"`
				} `json:"cronPresets"`
			} `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Data.Tasks == nil || len(envelope.Data.JobStatuses) != 6 || len(envelope.Data.RunStatuses) != 3 || len(envelope.Data.CronPresets) != 10 {
			t.Fatalf("options = %+v", envelope.Data)
		}
		want := "已调度"
		if locale == i18n.EnUS {
			want = "Scheduled"
		}
		if envelope.Data.JobStatuses[0].Label != want || envelope.Data.JobStatuses[0].Value != 1 {
			t.Fatalf("localized numeric statuses = %+v", envelope.Data.JobStatuses)
		}
	}
}
