package mail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRateLimitPoliciesHandlerReturnsSevenPolicies(t *testing.T) {
	db, ctx := openMailRepositoryDatabase(t)
	service := NewService(NewRepository(db), nil, nil, nil, nil, stubRateLimitPolicyStore{catalog: defaultPolicyCatalog()})
	handler := NewHandler(service)

	recorder := serveRateLimitHandler(t, handler, http.MethodGet, "/rate-limit-policies", "", nil)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Version  int64             `json:"version"`
			Policies []RateLimitPolicy `json:"policies"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != 0 || len(envelope.Data.Policies) != 7 {
		t.Fatalf("envelope = %+v", envelope)
	}
	if envelope.Data.Version < 1 {
		t.Fatalf("version = %d, want >= 1", envelope.Data.Version)
	}
	_ = ctx
}

func TestUpdateRateLimitPolicyHandlerReturnsUpdatedPolicy(t *testing.T) {
	db, ctx := openMailRepositoryDatabase(t)
	service := NewService(NewRepository(db), nil, nil, nil, nil, stubRateLimitPolicyStore{
		catalog: policyCatalogWith(map[string][2]int{"business_email_minute": {2, 120}}),
	})
	handler := NewHandler(service)

	recorder := serveRateLimitHandler(t, handler, http.MethodPut, "/rate-limit-policies/business_email_minute", `{"limit":2,"windowSeconds":120}`, gin.Params{{Key: "key", Value: "business_email_minute"}})

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Version int64           `json:"version"`
			Policy  RateLimitPolicy `json:"policy"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != 0 || envelope.Data.Policy.Key != "business_email_minute" {
		t.Fatalf("envelope = %+v", envelope)
	}
	if envelope.Data.Policy.Limit != 2 || envelope.Data.Policy.WindowSeconds != 120 {
		t.Fatalf("policy = %+v", envelope.Data.Policy)
	}
	_ = ctx
}

func TestUpdateRateLimitPolicyHandlerRejectsInvalidInput(t *testing.T) {
	db, ctx := openMailRepositoryDatabase(t)
	service := NewService(NewRepository(db), nil, nil, nil, nil, stubRateLimitPolicyStore{catalog: defaultPolicyCatalog()})
	handler := NewHandler(service)

	for _, body := range []string{
		`{"limit":0,"windowSeconds":60}`,
		`{"limit":1,"windowSeconds":0}`,
		`{"limit":1,"windowSeconds":60,"extra":true}`,
		`not-json`,
	} {
		recorder := serveRateLimitHandler(t, handler, http.MethodPut, "/rate-limit-policies/business_email_minute", body, gin.Params{{Key: "key", Value: "business_email_minute"}})
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %q status = %d, want 400", body, recorder.Code)
		}
	}

	recorder := serveRateLimitHandler(t, handler, http.MethodPut, "/rate-limit-policies/unknown", `{"limit":1,"windowSeconds":60}`, gin.Params{{Key: "key", Value: "unknown"}})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unknown key status = %d, want 400", recorder.Code)
	}
	_ = ctx
}

func TestUpdateRateLimitPolicyHandlerMapsStoreFailureToUnavailable(t *testing.T) {
	db, ctx := openMailRepositoryDatabase(t)
	service := NewService(NewRepository(db), nil, nil, nil, nil, stubRateLimitPolicyStore{err: context.Canceled})
	handler := NewHandler(service)

	recorder := serveRateLimitHandler(t, handler, http.MethodPut, "/rate-limit-policies/business_email_minute", `{"limit":2,"windowSeconds":120}`, gin.Params{{Key: "key", Value: "business_email_minute"}})
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
	_ = ctx
}

func serveRateLimitHandler(t *testing.T, handler *Handler, method, path, body string, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		c.Request.Header.Set("Content-Type", "application/json")
	}
	if params != nil {
		c.Params = params
	}
	switch {
	case method == http.MethodGet:
		handler.RateLimitPolicies(c)
	case method == http.MethodPut:
		handler.UpdateRateLimitPolicy(c)
	default:
		t.Fatalf("unsupported method %s", method)
	}
	return recorder
}
