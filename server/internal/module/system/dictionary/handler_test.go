package dictionary

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

type routeDictionaryService struct{}

func (routeDictionaryService) List(context.Context, ListQuery) (ListResult, error) {
	return ListResult{}, nil
}
func (routeDictionaryService) Get(context.Context, int64) (Detail, error)             { return Detail{}, nil }
func (routeDictionaryService) Create(context.Context, CreateInput) (int64, error)     { return 1, nil }
func (routeDictionaryService) Update(context.Context, int64, UpdateInput) error       { return nil }
func (routeDictionaryService) UpdateStatus(context.Context, int64, yesno.Value) error { return nil }
func (routeDictionaryService) Delete(context.Context, int64) error                    { return nil }
func (routeDictionaryService) Options(context.Context, []string, string) (OptionResult, error) {
	return OptionResult{}, nil
}
func (routeDictionaryService) CreateItem(context.Context, int64, CreateItemInput) (int64, error) {
	return 1, nil
}
func (routeDictionaryService) UpdateItem(context.Context, int64, int64, UpdateItemInput) error {
	return nil
}
func (routeDictionaryService) UpdateItemStatus(context.Context, int64, int64, yesno.Value) error {
	return nil
}
func (routeDictionaryService) DeleteItem(context.Context, int64, int64) error { return nil }

func TestRegisterRoutesRequiresDictionaryActionPermissions(t *testing.T) {
	router := gin.New()
	seen := make([]string, 0, 11)
	RegisterRoutes(router.Group("/api/admin/v1"), NewHandler(routeDictionaryService{}), func(c *gin.Context) { c.Next() }, func(code string) gin.HandlerFunc {
		seen = append(seen, code)
		return func(c *gin.Context) { c.Next() }
	})
	for _, path := range []string{
		"/api/admin/v1/system/dictionary?page=1&pageSize=20",
		"/api/admin/v1/system/dictionary/options?codes=user.gender",
		"/api/admin/v1/system/dictionary/1",
		"/api/admin/v1/system/dictionary/1/item/2",
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	}
	for _, expected := range []string{PermissionList, PermissionDetail, PermissionUpdate} {
		found := false
		for _, actual := range seen {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("permission %q was not registered: %v", expected, seen)
		}
	}
}
