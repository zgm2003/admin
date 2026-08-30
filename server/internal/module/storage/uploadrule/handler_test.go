package uploadrule

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"admin/server/internal/module/auth/login"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
)

type ruleHTTPService struct {
	query   ListQuery
	creates int
}

func (*ruleHTTPService) IssueCredentials(context.Context, auth.Identity, CredentialInput) (CredentialResponse, error) {
	return CredentialResponse{}, nil
}

func (s *ruleHTTPService) List(_ context.Context, q ListQuery) (pagination.Result[RuleValue], error) {
	s.query = q
	return pagination.Result[RuleValue]{List: []RuleValue{}, Page: q.Page, PageSize: q.PageSize}, nil
}
func (*ruleHTTPService) PageInit(context.Context) (PageInit, error) {
	return PageInit{}, nil
}
func (*ruleHTTPService) Get(context.Context, int64) (RuleValue, error) { return RuleValue{ID: 1}, nil }
func (s *ruleHTTPService) Create(context.Context, CreateInput) (int64, error) {
	s.creates++
	return 1, nil
}
func (*ruleHTTPService) Update(context.Context, int64, UpdateInput) error       { return nil }
func (*ruleHTTPService) UpdateStatus(context.Context, int64, yesno.Value) error { return nil }
func (*ruleHTTPService) Delete(context.Context, int64) error                    { return nil }

func TestRoutesUseExactUploadRulePermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	permissions := []string{}
	pass := func(c *gin.Context) { c.Next() }
	RegisterRoutes(router.Group("/api/admin/v1"), NewHandler(&ruleHTTPService{}), pass, func(code string) gin.HandlerFunc { permissions = append(permissions, code); return pass })
	RegisterCredentialRoute(router.Group("/api/v1"), NewHandler(&ruleHTTPService{}), pass, func(code string) gin.HandlerFunc { permissions = append(permissions, code); return pass })
	want := []string{PermissionList, PermissionList, PermissionCreate, PermissionList, PermissionUpdate, PermissionStatus, PermissionDelete, "storage:object:upload"}
	if !reflect.DeepEqual(permissions, want) {
		t.Fatalf("permissions=%v want=%v", permissions, want)
	}
}

func TestHandlerRejectsInvalidQueriesAndJSON(t *testing.T) {
	service, router := ruleRouter()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/admin/v1/storage/upload-rules?page=1&pageSize=20&platformId=2&cosConfigId=3&keyword=avatar&isEnabled=1", nil))
	if rec.Code != http.StatusOK || service.query.PlatformID == nil || *service.query.PlatformID != 2 || service.query.CosConfigID == nil || *service.query.CosConfigID != 3 || service.query.IsEnabled == nil || *service.query.IsEnabled != yesno.Yes {
		t.Fatalf("status=%d query=%+v body=%s", rec.Code, service.query, rec.Body)
	}
	for _, path := range []string{"/api/admin/v1/storage/upload-rules?page=1&pageSize=20&unknown=1", "/api/admin/v1/storage/upload-rules?page=1&page=2&pageSize=20", "/api/admin/v1/storage/upload-rules?page=1&pageSize=20&platformId=0"} {
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("path=%s status=%d body=%s", path, rec.Code, rec.Body)
		}
	}
	valid := `{"platformId":1,"code":"avatar","name":"Avatar","cosConfigId":1,"pathPrefix":"avatars","maxFileSizeBytes":1024,"maxFileCount":1,"allowedExtensions":["png"],"allowedMimeTypes":["image/png"],"accessMode":"private","isEnabled":1,"remark":""}`
	for _, body := range []string{valid + `{}`, valid[:len(valid)-1] + `,"unknown":1}`, `{"platformId":1}`} {
		rec = ruleJSON(router, http.MethodPost, "/api/admin/v1/storage/upload-rules", body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d response=%s", body, rec.Code, rec.Body)
		}
	}
}

func TestPageInitSerializesEmptyCollectionsAsArrays(t *testing.T) {
	_, router := ruleRouter()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/v1/storage/upload-rules/page-init", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"code":0,"data":{"platforms":[],"configs":[]},"message":"ok"}` {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
func ruleRouter() (*ruleHTTPService, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	s := &ruleHTTPService{}
	r := gin.New()
	pass := func(c *gin.Context) { c.Next() }
	RegisterRoutes(r.Group("/api/admin/v1"), NewHandler(s), pass, func(string) gin.HandlerFunc { return pass })
	return s, r
}
func ruleJSON(r *gin.Engine, m, p, b string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(m, p, bytes.NewBufferString(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	return rec
}
