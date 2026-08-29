// Package auth keeps the historical import path source-compatible while the
// implementation lives under auth/login, auth/client, auth/platform and
// auth/state. Schema changes are deliberately owned by manual SQL migration.
package auth

import (
	"context"

	authlogin "admin/server/internal/module/auth/login"
	usersession "admin/server/internal/module/user/session"
	projectredis "admin/server/internal/redis"
	"gorm.io/gorm"
)

type Session = usersession.Session
type SessionAuthority = usersession.Authority
type SessionCreate = usersession.CreateInput
type SessionCache = authlogin.SessionCache
type SessionSnapshot = authlogin.SessionSnapshot
type TokenIdentity = authlogin.TokenIdentity
type Identity = authlogin.Identity

const SessionSnapshotSchemaVersion = 1

func NewSessionCache(redis *projectredis.Client) *SessionCache {
	return authlogin.NewSessionCache(redis)
}

func SessionKey(platform string, sessionID int64) string {
	return authlogin.SessionKey(platform, sessionID)
}

// Deprecated: database schema changes are performed by the manual migration.
// These no-op functions exist only for legacy test packages during transition.
func PrepareSessionSchema(context.Context, *gorm.DB) error { return nil }

// Deprecated: database schema changes are performed by the manual migration.
// These no-op functions exist only for legacy test packages during transition.
func EnsureSchema(context.Context, *gorm.DB) error { return nil }
