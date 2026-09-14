package queuemonitor

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type grantValidator interface {
	Validate(context.Context, string) (GrantRecord, error)
}

type Gateway struct {
	service grantValidator
	next    http.Handler
}

func NewGateway(service grantValidator, next http.Handler) http.Handler {
	return &Gateway{service: service, next: next}
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if g == nil || g.service == nil || g.next == nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	cookie, err := r.Cookie(CookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	record, err := g.service.Validate(r.Context(), cookie.Value)
	if err != nil {
		if errors.Is(err, ErrGrantInvalid) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		} else {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		}
		return
	}
	if record.PermissionCode != PermissionList {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	g.next.ServeHTTP(w, r)
}
