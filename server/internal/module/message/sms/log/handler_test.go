package log

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type handlerServiceTest struct {
	detailID int64
}

func (s *handlerServiceTest) List(context.Context, ListQuery) (ListResult, error) {
	return ListResult{List: []Item{}, Page: 1, PageSize: 20}, nil
}

func (s *handlerServiceTest) Detail(_ context.Context, id int64) (Detail, error) {
	s.detailID = id
	return Detail{Log: Item{ID: id}}, nil
}

func TestDetailRouteNeedsOnlyLogID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &handlerServiceTest{}
	router := gin.New()
	pass := func(ctx *gin.Context) { ctx.Next() }
	RegisterRoutes(router.Group("/api/admin/v1/message/sms"), NewHandler(service), pass, func(string) gin.HandlerFunc { return pass })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/v1/message/sms/log/7", nil))
	if recorder.Code != http.StatusOK || service.detailID != 7 {
		t.Fatalf("status=%d detailID=%d body=%s", recorder.Code, service.detailID, recorder.Body.String())
	}
}

func TestRegisterRoutesBindsExactLogPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	seen := make(map[string]int, 2)
	router := gin.New()
	RegisterRoutes(router.Group("/api/admin/v1/message/sms"), NewHandler(&handlerServiceTest{}),
		func(c *gin.Context) { c.Next() },
		func(code string) gin.HandlerFunc {
			return func(c *gin.Context) {
				seen[code]++
				c.AbortWithStatus(http.StatusNoContent)
			}
		})

	for _, request := range []struct{ method, path string }{
		{http.MethodGet, "/api/admin/v1/message/sms/log"},
		{http.MethodGet, "/api/admin/v1/message/sms/log/7"},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(request.method, request.path, nil))
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("%s %s status=%d", request.method, request.path, recorder.Code)
		}
	}

	if seen[PermissionList] != 1 || seen[PermissionDetail] != 1 || seen["message:sms:view"] != 0 {
		t.Fatalf("registered permissions = %v", seen)
	}
}

func TestParseListQueryUsesPlatformCodePrefix(t *testing.T) {
	query, err := parseListQuery(map[string][]string{
		"page": {"1"}, "pageSize": {"20"}, "platform": {"ad"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if query.Platform != "ad" {
		t.Fatalf("platform=%q, want code prefix ad", query.Platform)
	}
}

func TestParseListQueryRejectsUnknownRepeatedAndMalformedValues(t *testing.T) {
	tests := []map[string][]string{
		{"unknown": {"x"}},
		{"page": {"1", "2"}},
		{"page": {"zero"}},
		{"page": {"1"}, "pageSize": {"20"}, "from": {"not-a-time"}},
		{"page": {"1"}, "pageSize": {"20"}, "scene": {"login", "forget"}},
	}
	for index, values := range tests {
		if _, err := parseListQuery(values); err == nil {
			t.Fatalf("case %d was accepted: %#v", index, values)
		}
	}
}
