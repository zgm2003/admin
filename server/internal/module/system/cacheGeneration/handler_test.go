package cachegeneration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type stubService struct {
	result ListResult
	err    error
	query  ListQuery
}

func (s *stubService) List(_ context.Context, query ListQuery) (ListResult, error) {
	s.query = query
	if s.err != nil {
		return ListResult{}, s.err
	}
	return s.result, nil
}

func TestParseListQueryDefaultsAndValidates(t *testing.T) {
	query, err := parseListQuery(url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	if query.Page != 1 || query.PageSize != 20 || query.Keyword != "" || query.PublishState != "" {
		t.Fatalf("default query = %+v", query)
	}

	valid, err := parseListQuery(url.Values{
		"page": {"2"}, "pageSize": {"50"}, "keyword": {" system.setting "}, "publishState": {PublishStateRetrying},
	})
	if err != nil {
		t.Fatalf("valid query error = %v", err)
	}
	if valid.Page != 2 || valid.PageSize != 50 || valid.Keyword != "system.setting" || valid.PublishState != PublishStateRetrying {
		t.Fatalf("valid query = %+v", valid)
	}

	invalid := []url.Values{
		{"page": {"0"}},
		{"page": {"x"}},
		{"pageSize": {"0"}},
		{"pageSize": {"101"}},
		{"publishState": {"unknown"}},
		{"keyword": {strings.Repeat("k", maxKeywordRunes+1)}},
		{"unknown": {"1"}},
		{"page": {"1", "2"}},
	}
	for _, values := range invalid {
		if _, err := parseListQuery(values); err == nil {
			t.Fatalf("query %v was accepted", values)
		}
	}
}

func TestHandlerListReturnsStrictResponseShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	publishedAt := time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC)
	publishedGeneration := int64(12)
	stub := &stubService{result: ListResult{
		Items: []Item{{
			Namespace: "system.setting", ScopeKey: "global", Generation: 12, Status: StatusReady,
			PendingCount: 0, LatestAttempts: 1, LastError: "",
			LatestPublishedGeneration: &publishedGeneration, LatestPublishedAt: &publishedAt, UpdatedAt: publishedAt,
		}},
		Total: 1, Page: 1, PageSize: 20,
	}}
	router := gin.New()
	router.GET("/system/cachegeneration", NewHandler(stub).List)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/system/cachegeneration?publishState=ready", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if stub.query.PublishState != PublishStateReady {
		t.Fatalf("handler query = %+v", stub.query)
	}

	var envelope struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			List     []map[string]any `json:"list"`
			Total    int64            `json:"total"`
			Page     int              `json:"page"`
			PageSize int              `json:"pageSize"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != 0 || envelope.Data.Total != 1 || len(envelope.Data.List) != 1 {
		t.Fatalf("envelope = %s", recorder.Body.String())
	}
	row := envelope.Data.List[0]
	wantKeys := []string{
		"namespace", "scopeKey", "generation", "status", "pendingCount", "oldestPendingAt",
		"latestAttempts", "lastError", "latestPublishedGeneration", "latestPublishedAt", "updatedAt",
	}
	if len(row) != len(wantKeys) {
		t.Fatalf("row keys = %v want %v", row, wantKeys)
	}
	for _, key := range wantKeys {
		if _, ok := row[key]; !ok {
			t.Fatalf("row lacks %q: %v", key, row)
		}
	}
	if row["oldestPendingAt"] != nil {
		t.Fatalf("oldestPendingAt = %v want null", row["oldestPendingAt"])
	}
	if row["status"] != StatusReady || row["updatedAt"] != "2026-09-16T08:00:00Z" {
		t.Fatalf("row = %v", row)
	}
}

func TestHandlerListRejectsInvalidQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/system/cachegeneration", NewHandler(&stubService{}).List)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/system/cachegeneration?publishState=invalid", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "10001") {
		t.Fatalf("body = %s want invalid request code", recorder.Body.String())
	}
}
