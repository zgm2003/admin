package recipientrule

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"github.com/gin-gonic/gin"
)

func TestXlsxPreviewSeparatesRawTextFromNumericData(t *testing.T) {
	rawRows := [][]string{
		{" 邮箱 ", " USER@example.com ", "拒绝", " 名称 ", " 原备注 ", "停用"},
		{"domain", "example.org", "允许", "错误类型", "", "启用"},
		{"域名", "EXAMPLE.ORG", "允许", "域名规则", "", "启用"},
	}
	input := matrixXlsx(t, append([][]string{xlsxHeader}, rawRows...))
	content, err := decodeXlsxImportInput(input)
	if err != nil {
		t.Fatal(err)
	}
	preview := parseXlsx(context.Background(), content)
	encoded, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Rows   []map[string]json.RawMessage `json:"rows"`
		Errors []string                     `json:"errors"`
	}
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 0 || len(result.Rows) != 3 {
		t.Fatalf("preview=%s", encoded)
	}
	for index, row := range result.Rows {
		if len(row) != 4 || row["rawValues"] == nil || row["data"] == nil || row["values"] != nil {
			t.Fatalf("preview row must separate rawValues/data: %s", encoded)
		}
		var raw []string
		if err := json.Unmarshal(row["rawValues"], &raw); err != nil || !reflect.DeepEqual(raw, rawRows[index]) {
			t.Fatalf("raw cell text was normalized: raw=%v want=%v err=%v", raw, rawRows[index], err)
		}
	}
	for index, want := range map[int]Input{
		0: {Scope: ScopeEmail, Pattern: "user@example.com", Action: ActionDeny, Name: "名称", Remark: " 原备注 ", IsEnabled: 0},
		2: {Scope: ScopeDomain, Pattern: "example.org", Action: ActionAllow, Name: "域名规则", Remark: "", IsEnabled: 1},
	} {
		var data Input
		if err := json.Unmarshal(result.Rows[index]["data"], &data); err != nil || data != want {
			t.Fatalf("numeric row=%+v want=%+v err=%v", data, want, err)
		}
	}
	if string(result.Rows[1]["data"]) != "null" || string(result.Rows[1]["errors"]) != `["invalid_scope"]` {
		t.Fatalf("invalid row must have null data and retain error: %s", encoded)
	}
}

func TestXlsxImportRoutesRequireIndependentActionBeforeBinding(t *testing.T) {
	for _, endpoint := range []struct{ method, path string }{
		{http.MethodGet, "/recipient-rule/import-template"},
		{http.MethodPost, "/recipient-rule/import/preview"},
		{http.MethodPost, "/recipient-rule/import"},
	} {
		t.Run(endpoint.path, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			authenticated := false
			permission := ""
			RegisterRoutes(router.Group(""), NewHandler(nil), func(ctx *gin.Context) {
				authenticated = true
				ctx.Next()
			}, func(code string) gin.HandlerFunc {
				return func(ctx *gin.Context) { permission = code; ctx.AbortWithStatus(http.StatusForbidden) }
			})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(endpoint.method, endpoint.path, strings.NewReader("bad json")))
			if response.Code != http.StatusForbidden || !authenticated || permission != PermissionImport {
				t.Fatalf("status=%d authenticated=%v permission=%q", response.Code, authenticated, permission)
			}
		})
	}
}

func TestXlsxImportHandlerRejectsMalformedRequestsWithExactEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	pass := func(ctx *gin.Context) { ctx.Next() }
	RegisterRoutes(router.Group(""), NewHandler(NewService(nil)), pass, func(string) gin.HandlerFunc { return pass })
	for _, path := range []string{"/recipient-rule/import/preview", "/recipient-rule/import"} {
		for _, body := range []string{
			"", "{}", "null", "[]", `{"content":null}`, `{"content":12}`, `{"content":"x","extra":1}`, `{"content":"x","content":"y"}`, `{"content":"x"} {}`,
			`{"fileName":null,"contentBase64":"UEsDBA=="}`, `{"fileName":"rules.xlsx","contentBase64":null}`,
			`{"fileName":3,"contentBase64":"UEsDBA=="}`, `{"fileName":"rules.xlsx","contentBase64":true}`,
			`{"fileName":"rules.xlsx"}`, `{"contentBase64":"UEsDBA=="}`,
			`{"fileName":"rules.xlsx","contentBase64":"UEsDBA==","extra":1}`,
			`{"fileName":"rules.xlsx","fileName":"rules.xlsx","contentBase64":"UEsDBA=="}`,
			`{"fileName":"rules.xlsx","contentBase64":"UEsDBA==","contentBase64":"UEsDBA=="}`,
			`{"FileName":"rules.xlsx","contentBase64":"UEsDBA=="}`,
			`{"fileName":"rules.xlsx","ContentBase64":"UEsDBA=="}`,
			`{"fileName":"rules.xlsx","contentBase64":"UEsDBA=="} {}`,
			`{"fileName":"../rules.xlsx","contentBase64":"UEsDBA=="}`,
			`{"fileName":"rules.csv","contentBase64":"UEsDBA=="}`,
			`{"fileName":"rules.xls","contentBase64":"UEsDBA=="}`,
			`{"fileName":"rules.xlsm","contentBase64":"UEsDBA=="}`,
			`{"fileName":"rules.xlsx","contentBase64":""}`,
			`{"fileName":"rules.xlsx","contentBase64":"UEsDBA= "}`,
			`{"fileName":"rules.xlsx","contentBase64":"data:application/xlsx;base64,UEsDBA=="}`,
			`{"fileName":"rules.xlsx","contentBase64":"UEsDBB=="}`,
			`{"fileName":"rules.xlsx","contentBase64":"` + strings.Repeat("A", 4*((XlsxMaxBytes+2)/3)+4) + `"}`,
		} {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
			var envelope map[string]json.RawMessage
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			var code int
			_ = json.Unmarshal(envelope["code"], &code)
			if response.Code != http.StatusBadRequest || len(envelope) != 3 || code != apperror.CodeInvalidRequest || string(envelope["data"]) != "null" || envelope["message"] == nil {
				t.Errorf("path=%s body=%.180q status=%d response=%s", path, body, response.Code, response.Body.String())
			}
		}
	}
}

func TestXlsxFailureMessagesDescribeCurrentContract(t *testing.T) {
	for _, locale := range []i18n.Locale{i18n.ZhCN, i18n.EnUS} {
		for _, key := range []i18n.MessageKey{i18n.KeyMailRuleImportInvalid, i18n.KeyMailRuleImportConflict, i18n.KeyMailRuleExportLimit} {
			message, err := i18n.Translate(locale, key, nil)
			if err != nil || !strings.Contains(message, "XLSX") || strings.Contains(message, "CSV") {
				t.Errorf("locale=%s key=%s message=%q err=%v", locale, key, message, err)
			}
			if key == i18n.KeyMailRuleExportLimit && (!strings.Contains(message, "1000") || !strings.Contains(message, "2 MiB")) {
				t.Errorf("wrong export limit: %q", message)
			}
		}
	}
}
