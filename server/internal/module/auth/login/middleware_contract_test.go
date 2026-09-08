package auth

import (
	"context"
	"testing"

	authclient "admin/server/internal/module/auth/client"
)

type authenticateOnly struct{}

func (authenticateOnly) Authenticate(context.Context, string, authclient.Client) (Identity, error) {
	return Identity{}, nil
}

func TestMiddlewareDependsOnlyOnAuthentication(t *testing.T) {
	if Authenticate(authenticateOnly{}) == nil {
		t.Fatal("missing authentication middleware")
	}
}
