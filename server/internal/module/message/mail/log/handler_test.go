package log

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

type handlerServiceStub struct {
	listCalls int
	filter    ListQuery
	page      int
	size      int
}

func (s *handlerServiceStub) List(_ context.Context, filter ListQuery, page, size int) ([]ListRow, int64, error) {
	s.listCalls++
	s.filter = filter
	s.page = page
	s.size = size
	return []ListRow{}, 0, nil
}

func (*handlerServiceStub) Get(context.Context, int64) (Detail, error) {
	return Detail{}, nil
}

func TestHandlerListUsesDefaultAndExplicitPagination(t *testing.T) {
	for _, test := range []struct {
		name       string
		query      string
		page, size int
	}{
		{name: "default", page: 1, size: 20},
		{name: "explicit", query: "?page=3&pageSize=100", page: 3, size: 100},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &handlerServiceStub{}
			response := requestLogList(service, test.query)
			if response.Code != http.StatusOK || service.listCalls != 1 || service.page != test.page || service.size != test.size {
				t.Fatalf("status=%d calls=%d page=%d size=%d", response.Code, service.listCalls, service.page, service.size)
			}
		})
	}
}

func TestHandlerListRejectsInvalidQueryWithoutCallingService(t *testing.T) {
	tooLongPlatform := strings.Repeat("界", 50)
	tooLongEmail := strings.Repeat("界", 255)
	tooLongScene := strings.Repeat("界", 33)
	tooLongStatus := strings.Repeat("界", 17)
	for _, test := range []struct {
		name  string
		query string
	}{
		{name: "page text", query: "?page=abc"},
		{name: "page zero", query: "?page=0"},
		{name: "page size text", query: "?pageSize=abc"},
		{name: "page size zero", query: "?pageSize=0"},
		{name: "page size over limit", query: "?pageSize=101"},
		{name: "unknown", query: "?unknown=value"},
		{name: "repeated", query: "?scene=login&scene=forget"},
		{name: "platform rune limit", query: "?platform=" + url.QueryEscape(tooLongPlatform)},
		{name: "email rune limit", query: "?toEmail=" + url.QueryEscape(tooLongEmail)},
		{name: "scene rune limit", query: "?scene=" + url.QueryEscape(tooLongScene)},
		{name: "status rune limit", query: "?status=" + url.QueryEscape(tooLongStatus)},
		{name: "invalid from", query: "?from=2026-09-09"},
		{name: "invalid to", query: "?to=not-a-time"},
		{name: "reversed range", query: "?from=2026-09-10T00%3A00%3A00Z&to=2026-09-09T00%3A00%3A00Z"},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &handlerServiceStub{}
			response := requestLogList(service, test.query)
			if response.Code != http.StatusBadRequest || service.listCalls != 0 {
				t.Fatalf("status=%d body=%s calls=%d", response.Code, response.Body.String(), service.listCalls)
			}
			var envelope struct {
				Code    int             `json:"code"`
				Data    json.RawMessage `json:"data"`
				Message string          `json:"message"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Code != 10001 || string(envelope.Data) != "null" || envelope.Message == "" {
				t.Fatalf("invalid error envelope: %+v", envelope)
			}
		})
	}
}

func TestHandlerListTrimsFiltersAndParsesRFC3339Nano(t *testing.T) {
	service := &handlerServiceStub{}
	from := "2026-09-09T01:02:03.123456789+08:00"
	to := "2026-09-09T02:03:04.987654321+08:00"
	query := "?platform=%20Admin%20&toEmail=%20User%40Example.com%20&scene=%20login%20&status=%20sent%20&from=" + url.QueryEscape(from) + "&to=" + url.QueryEscape(to)
	response := requestLogList(service, query)
	if response.Code != http.StatusOK || service.listCalls != 1 {
		t.Fatalf("status=%d body=%s calls=%d", response.Code, response.Body.String(), service.listCalls)
	}
	if service.filter.Platform != "Admin" || service.filter.ToEmail != "User@Example.com" || service.filter.Scene != "login" || service.filter.Status != "sent" {
		t.Fatalf("filter was not trimmed: %+v", service.filter)
	}
	wantFrom, _ := time.Parse(time.RFC3339Nano, from)
	wantTo, _ := time.Parse(time.RFC3339Nano, to)
	if service.filter.From == nil || !service.filter.From.Equal(wantFrom) || service.filter.To == nil || !service.filter.To.Equal(wantTo) {
		t.Fatalf("parsed range=%v..%v", service.filter.From, service.filter.To)
	}
}

func TestHandlerListAcceptsEmptyTimesAndUnicodeAtRuneLimits(t *testing.T) {
	service := &handlerServiceStub{}
	query := "?platform=" + url.QueryEscape(strings.Repeat("界", 49)) + "&scene=" + url.QueryEscape(strings.Repeat("场", 32)) + "&from=&to="
	response := requestLogList(service, query)
	if response.Code != http.StatusOK || service.listCalls != 1 || service.filter.From != nil || service.filter.To != nil {
		t.Fatalf("status=%d calls=%d filter=%+v", response.Code, service.listCalls, service.filter)
	}
}

func requestLogList(service handlerService, query string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/log", NewHandler(service).List)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/log"+query, nil))
	return response
}
