package queuemonitor

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type validatingServiceStub struct {
	record GrantRecord
	err    error
}

func (s validatingServiceStub) Validate(context.Context, string) (GrantRecord, error) {
	return s.record, s.err
}

func TestGatewayRequiresGrantAndRejectsWrites(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	gateway := NewGateway(validatingServiceStub{record: GrantRecord{UserID: 1, PlatformID: 2, PermissionCode: PermissionList}}, next)

	missing := httptest.NewRecorder()
	gateway.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, UIPath+"/", nil))
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("missing status=%d", missing.Code)
	}

	valid := httptest.NewRequest(http.MethodGet, UIPath+"/", nil)
	valid.AddCookie(&http.Cookie{Name: CookieName, Value: "grant"})
	ok := httptest.NewRecorder()
	gateway.ServeHTTP(ok, valid)
	if ok.Code != http.StatusNoContent {
		t.Fatalf("valid status=%d", ok.Code)
	}

	write := httptest.NewRequest(http.MethodPost, UIPath+"/api/queues/default:pause", nil)
	write.AddCookie(&http.Cookie{Name: CookieName, Value: "grant"})
	rejected := httptest.NewRecorder()
	gateway.ServeHTTP(rejected, write)
	if rejected.Code != http.StatusMethodNotAllowed {
		t.Fatalf("write status=%d", rejected.Code)
	}
}

func TestGatewayDistinguishesInvalidGrantAndDependencyFailure(t *testing.T) {
	request := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, UIPath+"/", nil)
		r.AddCookie(&http.Cookie{Name: CookieName, Value: "grant"})
		return r
	}
	invalid := NewGateway(validatingServiceStub{err: ErrGrantInvalid}, http.NotFoundHandler())
	recorder := httptest.NewRecorder()
	invalid.ServeHTTP(recorder, request())
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("invalid status=%d", recorder.Code)
	}

	failed := NewGateway(validatingServiceStub{err: errors.New("redis unavailable")}, http.NotFoundHandler())
	recorder = httptest.NewRecorder()
	failed.ServeHTTP(recorder, request())
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("failure status=%d", recorder.Code)
	}
}
