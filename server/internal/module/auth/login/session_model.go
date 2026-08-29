package auth

import (
	authstate "admin/server/internal/module/auth/state"
	usersession "admin/server/internal/module/user/session"
	projectredis "admin/server/internal/redis"
)

// Deprecated test migration aliases. Runtime persistence is owned by
// user/session and these aliases are removed with the legacy schema tests.
type Session = usersession.Session
type SessionAuthority = usersession.Authority
type SessionCreate = usersession.CreateInput

type SessionCache = authstate.SessionCache
type SessionSnapshot = authstate.SessionSnapshot

const sessionSnapshotSchemaVersion = authstate.SessionSnapshotSchemaVersion

func NewSessionCache(redis *projectredis.Client) *SessionCache {
	return authstate.NewSessionCache(redis)
}

func SessionKey(platform string, sessionID int64) string {
	return authstate.SessionKey(platform, sessionID)
}
