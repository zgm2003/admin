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
	query     ListQuery
	creates   int
	update    UpdateInput
	objectKey string
}

func (*ruleHTTPService) IssueCredentials(context.Context, auth.Identity, CredentialInput) (CredentialResponse, error) {
	return CredentialResponse{}, nil
}
func (s *ruleHTTPService) ObjectURL(_ context.Context, _ auth.Identity, objectKey string) (ObjectURLResult, error) {
	s.objectKey = objectKey
	return ObjectURLResult{URL: "https://download.example.com/object"}, nil
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
func (s *ruleHTTPService) Update(_ context.Context, _ int64, input UpdateInput) error {
	s.update = input
	return nil
}
func (*ruleHTTPService) UpdateStatus(context.Context, int64, yesno.Value) error { return nil }
func (*ruleHTTPService) Delete(context.Context, int64) error                    { return nil }

func TestRoutesUseExactUploadRulePermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	permissions := []string{}
	pass := func(c *gin.Context) { c.Next() }
	RegisterRoutes(router.Group("/api/admin/v1"), NewHandler(&ruleHTTPService{}), pass, func(code string) gin.HandlerFunc { permissions = append(permissions, code); return pass })
	RegisterCredentialRoute(router.Group("/api/v1"), NewHandler(&ruleHTTPService{}), pass, func(code string) gin.HandlerFunc { permissions = append(permissions, code); return pass })
	want := []string{PermissionList, PermissionList, PermissionCreate, PermissionDetail, PermissionUpdate, PermissionStatus, PermissionDelete, "storage:object:upload"}
	if !reflect.DeepEqual(permissions, want) {
		t.Fatalf("permissions=%v want=%v", permissions, want)
	}
}

func TestObjectURLRouteRequiresAuthenticationWithoutUploadPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &ruleHTTPService{}
	router := gin.New()
	authCalls := 0
	authenticated := func(c *gin.Context) {
		authCalls++
		c.Set("auth.identity", auth.Identity{PlatformID: 7})
		c.Next()
	}
	permissions := []string{}
	RegisterCredentialRoute(router.Group("/api/v1"), NewHandler(service), authenticated, func(code string) gin.HandlerFunc {
		permissions = append(permissions, code)
		return func(c *gin.Context) { c.Next() }
	})

	body := `{"objectKey":"avatar/.admin-storage/v2/p7/r2/c3/v1/2026/09/17/6e7e53334a6da066b130a89cc3f69535.png"}`
	recorder := ruleJSON(router, http.MethodPost, "/api/v1/storage/object-url", body)
	if recorder.Code != http.StatusOK || authCalls != 1 || service.objectKey == "" {
		t.Fatalf("status=%d authCalls=%d key=%q body=%s", recorder.Code, authCalls, service.objectKey, recorder.Body.String())
	}
	if !reflect.DeepEqual(permissions, []string{"storage:object:upload"}) {
		t.Fatalf("permissions=%v", permissions)
	}
	if recorder.Body.String() != `{"code":0,"data":{"url":"https://download.example.com/object","expiresAt":null},"message":"ok"}` {
		t.Fatalf("body=%s", recorder.Body.String())
	}

	for _, invalidBody := range []string{
		`{"objectKey":""}`,
		`{"ruleCode":"avatar","objectKey":"avatar/.admin-storage/v2/p7/r2/c3/v1/2026/09/17/6e7e53334a6da066b130a89cc3f69535.png"}`,
		body + `{}`,
	} {
		response := ruleJSON(router, http.MethodPost, "/api/v1/storage/object-url", invalidBody)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid body=%s status=%d response=%s", invalidBody, response.Code, response.Body.String())
		}
	}
}

func TestObjectURLHandlerRejectsMissingIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	pass := func(c *gin.Context) { c.Next() }
	RegisterCredentialRoute(router.Group("/api/v1"), NewHandler(&ruleHTTPService{}), pass, func(string) gin.HandlerFunc { return pass })
	recorder := ruleJSON(router, http.MethodPost, "/api/v1/storage/object-url", `{"objectKey":"avatar/.admin-storage/v2/p1/r2/c3/v1/2026/09/17/6e7e53334a6da066b130a89cc3f69535.png"}`)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlerRejectsInvalidQueriesAndJSON(t *testing.T) {
	service, router := ruleRouter()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/admin/v1/storage/uploadrule?page=1&pageSize=20&platformId=2&cosConfigId=3&keyword=avatar&isEnabled=1", nil))
	if rec.Code != http.StatusOK || service.query.PlatformID == nil || *service.query.PlatformID != 2 || service.query.CosConfigID == nil || *service.query.CosConfigID != 3 || service.query.IsEnabled == nil || *service.query.IsEnabled != yesno.Yes {
		t.Fatalf("status=%d query=%+v body=%s", rec.Code, service.query, rec.Body)
	}
	for _, path := range []string{"/api/admin/v1/storage/uploadrule?page=1&pageSize=20&unknown=1", "/api/admin/v1/storage/uploadrule?page=1&page=2&pageSize=20", "/api/admin/v1/storage/uploadrule?page=1&pageSize=20&platformId=0"} {
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("path=%s status=%d body=%s", path, rec.Code, rec.Body)
		}
	}
	valid := `{"platformId":1,"codes":["avatar","article-cover"],"name":"Avatar","cosConfigId":1,"maxFileSizeBytes":1024,"allowedExtensions":["png"],"allowedMimeTypes":["image/png"],"accessMode":"private","isEnabled":1,"remark":""}`
	for _, body := range []string{valid + `{}`, valid[:len(valid)-1] + `,"unknown":1}`, `{"platformId":1}`} {
		rec = ruleJSON(router, http.MethodPost, "/api/admin/v1/storage/uploadrule", body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d response=%s", body, rec.Code, rec.Body)
		}
	}
}

func TestPageInitSerializesEmptyCollectionsAsArrays(t *testing.T) {
	_, router := ruleRouter()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/v1/storage/uploadrule/page-init", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"code":0,"data":{"platforms":[],"configs":[]},"message":"ok"}` {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlerBindsCodesWhenUpdatingUploadRule(t *testing.T) {
	service, router := ruleRouter()
	body := `{"codes":[" Avatar-V2 ","profile-photo"],"name":"Avatar","maxFileSizeBytes":1024,"allowedExtensions":["png"],"allowedMimeTypes":["image/png"],"remark":""}`
	recorder := ruleJSON(router, http.MethodPut, "/api/admin/v1/storage/uploadrule/7", body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !reflect.DeepEqual(service.update.Codes, []string{"avatar-v2", "profile-photo"}) {
		t.Fatalf("codes=%v", service.update.Codes)
	}
	// platform/config/access 创建后只读，且 last-write-wins 不引入任何版本字段：严格 DTO 必须拒绝。
	for _, forbidden := range []string{`"cosConfigId":2`, `"accessMode":"public"`, `"platformId":2`, `"revision":1`, `"expectedRevision":1`} {
		legacy := "{" + forbidden + "," + body[1:]
		if rec := ruleJSON(router, http.MethodPut, "/api/admin/v1/storage/uploadrule/7", legacy); rec.Code != http.StatusBadRequest {
			t.Fatalf("legacy field %s status=%d body=%s", forbidden, rec.Code, rec.Body.String())
		}
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
