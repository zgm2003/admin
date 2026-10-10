package config

import (
	"admin/server/internal/shared/i18n"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOptionsAreLocalizedAuthenticatedAndStatic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	labels := make([]string, 0, 2)
	for _, locale := range []i18n.Locale{i18n.ZhCN, i18n.EnUS} {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Request = c.Request.WithContext(i18n.WithLocale(c.Request.Context(), locale))
			c.Next()
		})
		permissionCalls := 0
		RegisterRoutes(router.Group("/api/admin/v1"), NewHandler(NewService(nil, nil, nil)),
			func(c *gin.Context) {
				if c.GetHeader("Authorization") != "Bearer fixture" {
					c.AbortWithStatus(http.StatusUnauthorized)
					return
				}
				c.Next()
			},
			func(string) gin.HandlerFunc {
				return func(c *gin.Context) { permissionCalls++; c.AbortWithStatus(http.StatusForbidden) }
			})
		request := httptest.NewRequest(http.MethodGet, "/api/admin/v1/config/options", nil)
		denied := httptest.NewRecorder()
		router.ServeHTTP(denied, request)
		if denied.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated options status = %d", denied.Code)
		}
		request.Header.Set("Authorization", "Bearer fixture")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("options status = %d body=%s", recorder.Code, recorder.Body.String())
		}
		var envelope struct {
			Code int `json:"code"`
			Data struct {
				Regions []struct {
					Value json.RawMessage `json:"value"`
					Label string          `json:"label"`
				} `json:"regions"`
			} `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Code != 0 || len(envelope.Data.Regions) != 1 {
			t.Fatalf("options = %s", recorder.Body.String())
		}
		labels = append(labels, envelope.Data.Regions[0].Label)
		for index, option := range envelope.Data.Regions {
			if option.Label == "" {
				t.Fatal("empty option label")
			}
			_ = index
			var value string
			if err := json.Unmarshal(option.Value, &value); err != nil || value == "" {
				t.Fatalf("option value must be string: %s", option.Value)
			}
		}

		if permissionCalls != 0 {
			t.Fatalf("options invoked CRUD permission middleware %d times", permissionCalls)
		}
	}
	if labels[0] == labels[1] {
		t.Fatalf("labels are not localized: %v", labels)
	}
}

func TestStaticOptionsRespectContextLocaleAndDoNotShareMutableResults(t *testing.T) {
	service := &Service{}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Options(canceled); err == nil {
		t.Fatal("canceled options returned success")
	}
	if _, err := service.Options(i18n.WithLocale(context.Background(), i18n.Locale("invalid"))); err == nil {
		t.Fatal("unknown locale returned success")
	}
	first, err := service.Options(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	first.Regions[0].Label = "mutated"
	second, err := service.Options(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if second.Regions[0].Label == "mutated" {
		t.Fatal("option result shares mutable storage across requests")
	}
}
