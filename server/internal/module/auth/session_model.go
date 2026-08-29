package auth

import usersession "admin/server/internal/module/user/session"

// Deprecated test migration aliases. Runtime persistence is owned by
// user/session and these aliases are removed with the legacy schema tests.
type Session = usersession.Session
type SessionAuthority = usersession.Authority
type SessionCreate = usersession.CreateInput
