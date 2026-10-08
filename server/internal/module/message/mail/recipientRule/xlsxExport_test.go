package recipientrule

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/yesno"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const exportXlsxHeader = "类型,邮箱/域名,动作,名称,备注,启用状态\n"

func exportRouter(t *testing.T) (*gorm.DB, *Service, *gin.Engine) {
	t.Helper()
	db, _ := openServiceDatabase(t)
	service := NewService(configuredRepository(db))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	pass := func(ctx *gin.Context) { ctx.Next() }
	RegisterRoutes(router.Group(""), NewHandler(service), pass, func(string) gin.HandlerFunc { return pass })
	return db, service, router
}

func exportData(t *testing.T, router *gin.Engine) (string, string) {
	t.Helper()
	result := httptest.NewRecorder()
	router.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/recipient-rule/export", nil))
	if result.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", result.Code, result.Body.String())
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			FileName      string `json:"fileName"`
			ContentBase64 string `json:"contentBase64"`
		} `json:"data"`
	}
	if err := json.Unmarshal(result.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != 0 {
		t.Fatalf("body=%s", result.Body.String())
	}
	return envelope.Data.FileName, envelope.Data.ContentBase64
}

func TestXlsxExportUsesStableSixColumnFormatAndExcludesDeletedRows(t *testing.T) {
	db, _, router := exportRouter(t)
	now := time.Date(2026, time.September, 30, 1, 2, 3, 0, time.UTC)
	rows := []Model{
		{Scope: ScopeEmail, Pattern: "first@example.com", Action: ActionAllow, Name: "first", Remark: "备注,\"含引号\"\n第二行", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now},
		{Scope: ScopeDomain, Pattern: "example.org", Action: ActionDeny, Name: "second", Remark: "disabled", IsEnabled: yesno.No, CreatedAt: now, UpdatedAt: now},
		{Scope: ScopeEmail, Pattern: "deleted@example.com", Action: ActionDeny, Name: "deleted", Remark: "", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now, DeletedAt: gorm.DeletedAt{Time: now, Valid: true}},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	fileName, content := exportData(t, router)
	if fileName != "mail-recipient-rule.xlsx" {
		t.Fatalf("fileName=%q", fileName)
	}
	parsed := xlsxRows(t, content)
	want := [][]string{
		{"类型", "邮箱/域名", "动作", "名称", "备注", "启用状态"},
		{"邮箱", "first@example.com", "允许", "first", "备注,\"含引号\"\n第二行", "启用"},
		{"域名", "example.org", "拒绝", "second", "disabled", "停用"},
	}
	if !reflect.DeepEqual(parsed, want) {
		t.Fatalf("rows=%#v want=%#v", parsed, want)
	}
}

func TestXlsxExportEscapesSpreadsheetFormulaCells(t *testing.T) {
	db, _, router := exportRouter(t)
	now := time.Now().UTC()
	row := Model{Scope: ScopeEmail, Pattern: "safe@example.com", Action: ActionAllow, Name: "=1+1", Remark: "  @formula", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	_, content := exportData(t, router)
	parsed := xlsxRows(t, content)
	if parsed[1][1] != "safe@example.com" || parsed[1][3] != "=1+1" || parsed[1][4] != "  @formula" {
		t.Fatalf("formula escaping=%#v", parsed[1])
	}
}

func TestXlsxExportReturnsHeaderForEmptyRules(t *testing.T) {
	_, _, router := exportRouter(t)
	_, content := exportData(t, router)
	if rows := xlsxRows(t, content); len(rows) != 1 || !reflect.DeepEqual(rows[0], xlsxHeader) {
		t.Fatalf("rows=%v", rows)
	}
}

func TestXlsxExportRejectsRowsBeyondExplicitLimit(t *testing.T) {
	db, _, router := exportRouter(t)
	now := time.Now().UTC()
	rows := make([]Model, XlsxMaxRows+1)
	for index := range rows {
		rows[index] = Model{Scope: ScopeEmail, Pattern: "u" + time.Unix(int64(index), 0).Format("150405") + "@example.com", Action: ActionDeny, Name: "name", IsEnabled: yesno.Yes, CreatedAt: now, UpdatedAt: now}
	}
	if err := db.CreateInBatches(rows, 200).Error; err != nil {
		t.Fatal(err)
	}
	result := httptest.NewRecorder()
	router.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/recipient-rule/export", nil))
	if result.Code != http.StatusBadRequest || !strings.Contains(result.Body.String(), "1000") {
		t.Fatalf("status=%d body=%s", result.Code, result.Body.String())
	}
}

func TestXlsxExportRouteUsesIndependentPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	authenticated := false
	permission := ""
	RegisterRoutes(router.Group(""), NewHandler(nil), func(ctx *gin.Context) {
		authenticated = true
		ctx.Next()
	}, func(code string) gin.HandlerFunc {
		return func(ctx *gin.Context) {
			permission = code
			ctx.AbortWithStatus(http.StatusForbidden)
		}
	})
	result := httptest.NewRecorder()
	router.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/recipient-rule/export", nil))
	if result.Code != http.StatusForbidden || !authenticated || permission != PermissionExport {
		t.Fatalf("status=%d authenticated=%v permission=%q", result.Code, authenticated, permission)
	}
}

func TestXlsxExportDoesNotMutateMailGeneration(t *testing.T) {
	db, service, router := exportRouter(t)
	service.SetRuntimeCoordinator(runtimeCoordinatorFunc(func(ctx context.Context, change func(context.Context, int64) (cachegeneration.MutationResult, error)) error {
		_, err := change(ctx, 1)
		return err
	}))
	if _, err := service.Create(context.Background(), Input{Scope: ScopeEmail, Pattern: "before@example.com", Action: ActionDeny, Name: "before", IsEnabled: yesno.Yes}); err != nil {
		t.Fatal(err)
	}
	var before int64
	if err := db.Raw("SELECT generation FROM system_config_cache_generation WHERE namespace='message.mail' AND scope_key='global'").Scan(&before).Error; err != nil {
		t.Fatal(err)
	}
	_, _ = exportData(t, router)
	var after int64
	if err := db.Raw("SELECT generation FROM system_config_cache_generation WHERE namespace='message.mail' AND scope_key='global'").Scan(&after).Error; err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("generation changed from %d to %d", before, after)
	}
}

func TestXlsxExportRejectsLossyStoredTextWithoutPartialFile(t *testing.T) {
	valid := Model{Scope: ScopeEmail, Pattern: "valid@example.com", Action: ActionDeny, Name: "valid", IsEnabled: yesno.Yes}
	for _, tc := range []struct {
		name    string
		invalid Model
	}{
		{"name length", Model{Scope: ScopeEmail, Pattern: "a@example.com", Action: ActionDeny, Name: strings.Repeat("名", 129), IsEnabled: yesno.Yes}},
		{"remark length", Model{Scope: ScopeEmail, Pattern: "a@example.com", Action: ActionDeny, Name: "bad", Remark: strings.Repeat("注", 513), IsEnabled: yesno.Yes}},
		{"XML control", Model{Scope: ScopeEmail, Pattern: "a@example.com", Action: ActionDeny, Name: "bad\x01name", IsEnabled: yesno.Yes}},
		{"invalid pattern", Model{Scope: ScopeEmail, Pattern: "bad-address", Action: ActionDeny, Name: "bad", IsEnabled: yesno.Yes}},
		{"invalid scope", Model{Scope: 2, Pattern: "a@example.com", Action: ActionDeny, Name: "bad", IsEnabled: yesno.Yes}},
		{"invalid UTF8", Model{Scope: ScopeEmail, Pattern: "a@example.com", Action: ActionDeny, Name: string([]byte{0xff}), IsEnabled: yesno.Yes}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, err := encodeXlsx(context.Background(), []Model{valid, tc.invalid})
			if err == nil || file.FileName != "" || file.ContentBase64 != "" {
				t.Fatalf("lossy export succeeded: filename=%q contentBytes=%d err=%v", file.FileName, len(file.ContentBase64), err)
			}
		})
	}
}
