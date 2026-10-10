package operationlog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"

	"admin/server/internal/shared/i18n"
	"github.com/gin-gonic/gin"
)

type presentationRepository struct {
	items []Item
	calls int
	ctx   context.Context
}

func (*presentationRepository) Insert(context.Context, TaskPayload) error { return nil }
func (r *presentationRepository) List(ctx context.Context, _ ListQuery) ([]Item, int64, error) {
	r.calls++
	r.ctx = ctx
	return append([]Item{}, r.items...), int64(len(r.items)), nil
}

func TestListOwnsLocalizedActionLabelsWithoutExtraRepositoryReads(t *testing.T) {
	for _, tc := range []struct {
		locale i18n.Locale
		want   []string
	}{
		{i18n.ZhCN, []string{"编辑用户", "编辑邮件配置", "修改手机号", "future.action", ""}},
		{i18n.EnUS, []string{"Edit user", "Edit mail configuration", "Change phone number", "future.action", ""}},
	} {
		repository := &presentationRepository{items: []Item{{Action: "user.update"}, {Action: "mail.config.update"}, {Action: "user.phone.update"}, {Action: "future.action"}, {Action: ""}}}
		ctx := i18n.WithLocale(context.Background(), tc.locale)
		result, err := NewService(repository).List(ctx, ListQuery{Page: 1, PageSize: 20})
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		var response struct {
			List []struct{ Action, ActionLabel string }
		}
		if err = json.Unmarshal(encoded, &response); err != nil {
			t.Fatal(err)
		}
		for index, row := range response.List {
			if row.Action != repository.items[index].Action || row.ActionLabel != tc.want[index] {
				t.Fatalf("locale=%s row=%d got=%+v want=%q", tc.locale, index, row, tc.want[index])
			}
		}
		if repository.calls != 1 || repository.ctx != ctx {
			t.Fatalf("repository calls=%d request context forwarded=%v", repository.calls, repository.ctx == ctx)
		}
	}
}

func TestEveryRecordedRouteActionHasBothLocalizedLabels(t *testing.T) {
	for _, rule := range routeRules {
		zh := actionLabel(i18n.WithLocale(context.Background(), i18n.ZhCN), rule.Action)
		en := actionLabel(i18n.WithLocale(context.Background(), i18n.EnUS), rule.Action)
		if zh == "" || en == "" || zh == rule.Action || en == rule.Action || zh == en {
			t.Errorf("recorded action %q is not localized: zh=%q en=%q", rule.Action, zh, en)
		}
	}
}

func TestListResponseKeepsStrictEnvelopeAndActionCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := &presentationRepository{items: []Item{{ID: 7, Action: "user.update", RequestData: JSON(`{"redacted":"***"}`), ResponseData: JSON(`{"code":0}`)}}}
	router := gin.New()
	RegisterRoutes(router.Group("/api/admin/v1"), NewHandler(NewService(repository)), func(c *gin.Context) {
		c.Request = c.Request.WithContext(i18n.WithLocale(c.Request.Context(), i18n.EnUS))
		c.Next()
	}, func(string) gin.HandlerFunc { return func(c *gin.Context) { c.Next() } })
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/v1/system/operationlog?page=1&pageSize=20", nil))
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil || recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s err=%v", recorder.Code, recorder.Body.String(), err)
	}
	assertJSONKeys(t, envelope, []string{"code", "data", "message"})
	var data map[string]json.RawMessage
	if err := json.Unmarshal(envelope["data"], &data); err != nil {
		t.Fatal(err)
	}
	assertJSONKeys(t, data, []string{"list", "page", "pageSize", "total"})
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(data["list"], &rows); err != nil || len(rows) != 1 {
		t.Fatalf("list=%s err=%v", data["list"], err)
	}
	assertJSONKeys(t, rows[0], []string{"id", "requestId", "userId", "userName", "sessionId", "platform", "method", "route", "module", "action", "actionLabel", "clientIp", "userAgent", "statusCode", "isSuccess", "latencyMs", "requestData", "responseData", "createdAt", "updatedAt"})
	if string(rows[0]["action"]) != `"user.update"` || string(rows[0]["actionLabel"]) != `"Edit user"` || string(rows[0]["requestData"]) != `{"redacted":"***"}` {
		t.Fatalf("action or payload lost fidelity: %s", data["list"])
	}
}

func assertJSONKeys(t *testing.T, value map[string]json.RawMessage, want []string) {
	t.Helper()
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	sort.Strings(want)
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("JSON fields=%v want=%v", keys, want)
	}
}
