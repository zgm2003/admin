package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"admin/server/internal/shared/i18n"
	"github.com/gin-gonic/gin"
)

func TestOptionsOwnLocalizedValuesWithoutDependencies(t *testing.T) {
	zh := notificationOptions(i18n.WithLocale(context.Background(), i18n.ZhCN))
	en := notificationOptions(i18n.WithLocale(context.Background(), i18n.EnUS))
	if len(zh.Variants) != 4 || len(zh.Priorities) != 2 || len(en.Variants) != 4 {
		t.Fatalf("option sizes: %#v / %#v", zh, en)
	}
	if zh.Variants[0].Value != VariantInfo || zh.Priorities[0].Value != PriorityNormal {
		t.Fatal("options must preserve protocol constants")
	}
	for index, item := range zh.Variants {
		if item.Value != en.Variants[index].Value || item.Label == "" || item.Label == en.Variants[index].Label {
			t.Fatalf("invalid localized variant: %#v", item)
		}
	}
	for index, item := range zh.Priorities {
		if item.Value != en.Priorities[index].Value || item.Label == "" || item.Label == en.Priorities[index].Label {
			t.Fatalf("invalid localized priority: %#v", item)
		}
	}
}

func TestOptionsRouteRequiresIndependentListAction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	var codes []string
	RegisterRoutes(router.Group("/api/v1"), NewHandler(nil), func(c *gin.Context) { c.Next() }, func(code string) gin.HandlerFunc {
		codes = append(codes, code)
		return func(c *gin.Context) {
			if c.Request.Header.Get("X-Test-Allow") != code {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Request = c.Request.WithContext(i18n.WithLocale(c.Request.Context(), i18n.EnUS))
			c.Next()
		}
	})
	for _, allow := range []string{"message:notification:view", PermissionList} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/message/notification/options", nil)
		request.Header.Set("X-Test-Allow", allow)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if allow != PermissionList {
			if response.Code != http.StatusForbidden {
				t.Fatalf("page permission accepted: %d", response.Code)
			}
			continue
		}
		if response.Code != http.StatusOK {
			t.Fatalf("options status=%d body=%s", response.Code, response.Body.String())
		}
		var envelope struct {
			Data struct {
				Variants []struct {
					Value Variant
					Label string
				}
			}
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if len(envelope.Data.Variants) != 4 || envelope.Data.Variants[0].Label != "Info" {
			t.Fatalf("options envelope=%s", response.Body.String())
		}
	}
}
