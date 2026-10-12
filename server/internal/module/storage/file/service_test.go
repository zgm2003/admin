package file

import (
	"admin/server/internal/module/auth/login"
	"admin/server/internal/shared/apperror"
	"context"
	"errors"
	"net/url"
	"testing"
)

func TestIdentityAndFiltersAreNeverCallerControlled(t *testing.T) {
	s := NewService(nil, nil, nil, nil, nil)
	if _, e := s.List(context.Background(), auth.Identity{}, ListQuery{Page: 1, PageSize: 20}); e == nil {
		t.Fatal("empty identity accepted")
	}
	for _, raw := range []string{"userId=2", "platformId=2", "page=1&pageSize=101", "page=1&page=2"} {
		v, _ := url.ParseQuery(raw)
		if _, e := parseListQuery(v); e == nil {
			t.Fatalf("query accepted %s", raw)
		}
	}
	_, e := s.PrepareAvatar(context.Background(), auth.Identity{UserID: 1, PlatformID: 1}, 2, "not-uuid")
	var app *apperror.Error
	if !errors.As(e, &app) || app.Code != apperror.CodeInvalidRequest {
		t.Fatalf("uuid: %v", e)
	}
}
