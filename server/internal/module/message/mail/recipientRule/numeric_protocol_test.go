package recipientrule

import (
	"admin/server/internal/shared/cacheGeneration"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRecipientRuleModelUsesNumericZeroBasedEnums(t *testing.T) {
	for _, tc := range []struct {
		row           Model
		scope, action string
	}{
		{Model{Scope: ScopeEmail, Action: ActionDeny}, "0", "0"},
		{Model{Scope: ScopeDomain, Action: ActionAllow}, "1", "1"},
	} {
		raw, err := json.Marshal(tc.row)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		if string(fields["scope"]) != tc.scope || string(fields["action"]) != tc.action {
			t.Errorf("numeric enum JSON = %s", raw)
		}
	}
	for _, name := range []string{"Scope", "Action"} {
		field, _ := reflect.TypeOf(Model{}).FieldByName(name)
		if field.Type.Kind() != reflect.Int16 || !strings.Contains(field.Tag.Get("gorm"), "type:smallint") {
			t.Errorf("%s type/tag = %v %s", name, field.Type, field.Tag)
		}
	}
}

func TestRecipientRuleHandlersRequireStrictNumericEnumsAndPresence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	valid := `{"scope":0,"pattern":"user@example.com","action":0,"name":"rule","remark":"","isEnabled":0}`
	validDomain := `{"scope":1,"pattern":"example.com","action":1,"name":"rule","remark":"","isEnabled":1}`
	invalid := []string{
		strings.Replace(valid, `"scope":0`, `"scope":"email"`, 1),
		strings.Replace(valid, `"scope":0`, `"scope":"0"`, 1),
		strings.Replace(valid, `"action":0`, `"action":"deny"`, 1),
		strings.Replace(valid, `"scope":0`, `"scope":2`, 1),
		strings.Replace(valid, `"action":0`, `"action":-1`, 1),
		strings.Replace(valid, `"scope":0`, `"scope":0.5`, 1),
		strings.Replace(valid, `"action":0`, `"action":false`, 1),
		strings.Replace(valid, `"scope":0,`, "", 1),
		strings.Replace(valid, `"action":0,`, "", 1),
		strings.Replace(valid, `,"isEnabled":0`, "", 1),
		strings.Replace(valid, `"scope":0`, `"scope":null`, 1),
		strings.Replace(valid, `"action":0`, `"action":null`, 1),
		strings.Replace(valid, `"isEnabled":0`, `"isEnabled":null`, 1),
		strings.Replace(valid, `"isEnabled":0`, `"isEnabled":2`, 1),
	}
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		router := gin.New()
		handler := NewHandler(NewService(nil))
		if method == http.MethodPost {
			router.POST("/rule", handler.Create)
		} else {
			router.PUT("/rule/:id", handler.Update)
		}
		path := "/rule"
		if method == http.MethodPut {
			path += "/1"
		}
		for _, tc := range append(invalid, valid, validDomain) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(method, path, strings.NewReader(tc))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			want := http.StatusBadRequest
			if tc == valid || tc == validDomain {
				want = http.StatusServiceUnavailable
			}
			if recorder.Code != want {
				t.Errorf("%s %s status=%d want=%d body=%s", method, tc, recorder.Code, want, recorder.Body.String())
			}
		}
	}
}

func TestXlsxImportPersistsNumericEnumsAndExportsStableTextTokens(t *testing.T) {
	database, ctx := openServiceDatabase(t)
	service := NewService(configuredRepository(database))
	service.SetRuntimeCoordinator(runtimeCoordinatorFunc(func(ctx context.Context, change func(context.Context, int64) (cachegeneration.MutationResult, error)) error {
		_, err := change(ctx, 1)
		return err
	}))
	content := importHeader + "email,user@example.com,deny,first,,1\ndomain,example.org,allow,second,,0\n"
	if _, err := service.ImportXlsx(ctx, xlsxInput(t, content)); err != nil {
		t.Fatal(err)
	}
	rows, err := service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("imported rows=%d want 2", len(rows))
	}
	for index, row := range rows {
		raw, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		want := "0"
		if index == 1 {
			want = "1"
		}
		if string(fields["scope"]) != want || string(fields["action"]) != want {
			t.Errorf("imported JSON=%s", raw)
		}
	}
	exported, err := service.ExportXlsx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rowsOut := xlsxRows(t, exported.ContentBase64)
	if len(rowsOut) != 3 || rowsOut[1][0] != "邮箱" || rowsOut[1][2] != "拒绝" || rowsOut[2][0] != "域名" || rowsOut[2][2] != "允许" {
		t.Fatalf("export=%v", rowsOut)
	}
}
