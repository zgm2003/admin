package auth

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

func TestRegisterCreatesEnabledUserWithDefaultRole(t *testing.T) {
	var stored user.CreateInput
	users := &fakeUserStore{createFn: func(_ context.Context, input user.CreateInput) (user.User, error) {
		stored = input
		return user.User{ID: 7, Username: input.Username, Email: input.Email, IsEnabled: yesno.Yes}, nil
	}}
	service := newTestService(t, users, &fakeRoleStore{defaultRole: role.Role{ID: 5}}, &fakeSessionStore{}, &fakePointerStore{})

	registered, err := service.Register(context.Background(), RegisterInput{
		Username:        "  张三_01  ",
		Email:           "  USER@Example.COM ",
		Password:        "password123",
		ConfirmPassword: "password123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if stored.Username != "张三_01" || stored.Email != "user@example.com" || stored.RoleID != 5 || stored.PasswordHash == "" {
		t.Fatalf("CreateWithRole input = %+v", stored)
	}
	if registered.UserID != 7 || registered.Username != stored.Username || registered.Email != stored.Email {
		t.Fatalf("registered = %+v", registered)
	}
}

func TestRegisterRejectsInvalidUsernameEmailAndPassword(t *testing.T) {
	tests := []RegisterInput{
		{Username: "ab", Email: "user@example.com", Password: "password", ConfirmPassword: "password"},
		{Username: "bad name", Email: "user@example.com", Password: "password", ConfirmPassword: "password"},
		{Username: "valid_user", Email: "not-an-email", Password: "password", ConfirmPassword: "password"},
		{Username: "valid_user", Email: "user@example.com", Password: "password", ConfirmPassword: "different"},
		{Username: "valid_user", Email: "user@example.com", Password: "short", ConfirmPassword: "short"},
	}
	service := newTestService(t, &fakeUserStore{}, &fakeRoleStore{}, &fakeSessionStore{}, &fakePointerStore{})
	for _, input := range tests {
		if _, err := service.Register(context.Background(), input); appErrorCode(err) != apperror.CodeInvalidRequest {
			t.Errorf("Register(%+v) error = %v", input, err)
		}
	}
}

func TestRegisterMapsUsernameAndEmailConflicts(t *testing.T) {
	for _, test := range []struct {
		repositoryError error
		wantKey         i18n.MessageKey
	}{
		{repositoryError: user.ErrUsernameConflict, wantKey: i18n.KeyUsernameConflict},
		{repositoryError: user.ErrEmailConflict, wantKey: i18n.KeyEmailConflict},
	} {
		users := &fakeUserStore{createFn: func(context.Context, user.CreateInput) (user.User, error) {
			return user.User{}, test.repositoryError
		}}
		service := newTestService(t, users, &fakeRoleStore{defaultRole: role.Role{ID: 1}}, &fakeSessionStore{}, &fakePointerStore{})
		_, err := service.Register(context.Background(), RegisterInput{
			Username: "valid_user", Email: "user@example.com", Password: "password", ConfirmPassword: "password",
		})
		var appErr *apperror.Error
		if !errors.As(err, &appErr) || appErr.Code != apperror.CodeConflict || appErr.MessageKey != test.wantKey {
			t.Errorf("repository error %v mapped to %v", test.repositoryError, err)
		}
	}
}

func TestRegisterDoesNotCreateSession(t *testing.T) {
	sessions := &fakeSessionStore{}
	users := &fakeUserStore{createFn: func(_ context.Context, input user.CreateInput) (user.User, error) {
		return user.User{ID: 1, Username: input.Username, Email: input.Email}, nil
	}}
	service := newTestService(t, users, &fakeRoleStore{defaultRole: role.Role{ID: 1}}, sessions, &fakePointerStore{})
	_, err := service.Register(context.Background(), RegisterInput{
		Username: "valid_user", Email: "user@example.com", Password: "password", ConfirmPassword: "password",
	})
	if err != nil {
		t.Fatal(err)
	}
	if sessions.createCalls != 0 {
		t.Fatalf("session create calls = %d", sessions.createCalls)
	}
}

func TestLoginReturnsCredentialAndCurrentSession(t *testing.T) {
	passwordHash, err := HashPassword("password")
	if err != nil {
		t.Fatal(err)
	}
	fixedNow := time.Date(2026, time.August, 17, 11, 0, 0, 0, time.UTC)
	callOrder := make([]string, 0, 2)
	users := &fakeUserStore{credential: user.Credential{ID: 9, Username: "admin", Email: "admin@example.com", PasswordHash: passwordHash, IsEnabled: yesno.Yes}}
	sessions := &fakeSessionStore{createFn: func(_ context.Context, input SessionCreate, now time.Time) (Session, error) {
		callOrder = append(callOrder, "session")
		return Session{ID: 12, UserID: input.UserID, Version: 1, RefreshExpiresAt: input.RefreshExpiresAt}, nil
	}}
	pointers := &fakePointerStore{setFn: func(_ context.Context, key, value string, ttl time.Duration) error {
		callOrder = append(callOrder, "pointer")
		if key != "auth:current-session:9" || value != "12" || ttl != RefreshTTL {
			t.Fatalf("SetString(%q,%q,%v)", key, value, ttl)
		}
		return nil
	}}
	service := newTestService(t, users, &fakeRoleStore{}, sessions, pointers)
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now

	credential, err := service.Login(context.Background(), LoginInput{Username: "admin", Password: "password", ClientIP: "127.0.0.1", UserAgent: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if credential.AccessToken == "" || credential.ExpiresIn != int(AccessTTL.Seconds()) || credential.RefreshToken == "" || !credential.RefreshExpiresAt.Equal(fixedNow.Add(RefreshTTL)) {
		t.Fatalf("credential = %+v", credential)
	}
	if strings.Join(callOrder, ",") != "session,pointer" {
		t.Fatalf("call order = %v", callOrder)
	}
}

func TestLoginUsesTheSamePublicErrorForUnknownUserAndWrongPassword(t *testing.T) {
	wrongHash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatal(err)
	}
	services := []*Service{
		newTestService(t, &fakeUserStore{credentialErr: gorm.ErrRecordNotFound}, &fakeRoleStore{}, &fakeSessionStore{}, &fakePointerStore{}),
		newTestService(t, &fakeUserStore{credential: user.Credential{ID: 1, PasswordHash: wrongHash, IsEnabled: yesno.Yes}}, &fakeRoleStore{}, &fakeSessionStore{}, &fakePointerStore{}),
	}
	var first *apperror.Error
	for index, service := range services {
		_, loginErr := service.Login(context.Background(), LoginInput{Username: "missing", Password: "wrong"})
		var appErr *apperror.Error
		if !errors.As(loginErr, &appErr) || appErr.Code != apperror.CodeUnauthorized {
			t.Fatalf("login error %d = %v", index, loginErr)
		}
		if first == nil {
			first = appErr
		} else if appErr.HTTPStatus != first.HTTPStatus || appErr.Code != first.Code || appErr.MessageKey != first.MessageKey || !reflect.DeepEqual(appErr.Params, first.Params) {
			t.Fatalf("public credential errors differ: %+v vs %+v", first, appErr)
		}
	}
}

func TestLoginRejectsDisabledUser(t *testing.T) {
	service := newTestService(t, &fakeUserStore{credential: user.Credential{ID: 1, IsEnabled: yesno.No}}, &fakeRoleStore{}, &fakeSessionStore{}, &fakePointerStore{})
	if _, err := service.Login(context.Background(), LoginInput{Username: "disabled", Password: "password"}); appErrorCode(err) != apperror.CodeForbidden {
		t.Fatalf("disabled user error = %v", err)
	}
}

func TestLoginRevokesNewSessionWhenRedisWriteFails(t *testing.T) {
	passwordHash, err := HashPassword("password")
	if err != nil {
		t.Fatal(err)
	}
	sessions := &fakeSessionStore{createFn: func(_ context.Context, input SessionCreate, _ time.Time) (Session, error) {
		return Session{ID: 44, UserID: input.UserID, Version: 1, RefreshExpiresAt: input.RefreshExpiresAt}, nil
	}}
	pointers := &fakePointerStore{setFn: func(context.Context, string, string, time.Duration) error {
		return errors.New("redis unavailable")
	}}
	service := newTestService(t, &fakeUserStore{credential: user.Credential{ID: 3, PasswordHash: passwordHash, IsEnabled: yesno.Yes}}, &fakeRoleStore{}, sessions, pointers)

	if _, err := service.Login(context.Background(), LoginInput{Username: "admin", Password: "password"}); appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Login() error = %v", err)
	}
	if len(sessions.revokedIDs) != 1 || sessions.revokedIDs[0] != 44 {
		t.Fatalf("revoked session IDs = %v", sessions.revokedIDs)
	}
}

func TestAuthenticateChecksPointerAndDatabaseVersion(t *testing.T) {
	fixedNow := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	want := Identity{UserID: 5, SessionID: 8, Version: 2}
	sessions := &fakeSessionStore{activeIdentity: want}
	pointers := &fakePointerStore{getValue: "8", getFound: true}
	service := newTestService(t, &fakeUserStore{}, &fakeRoleStore{}, sessions, pointers)
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now
	accessToken, _, err := service.jwt.Issue(want)
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Authenticate(context.Background(), accessToken)
	if err != nil {
		t.Fatal(err)
	}
	if got != want || sessions.activeSessionID != want.SessionID || sessions.activeVersion != want.Version {
		t.Fatalf("Authenticate() = %+v, calls session=%d version=%d", got, sessions.activeSessionID, sessions.activeVersion)
	}
	if pointers.getKey != "auth:current-session:5" {
		t.Fatalf("pointer key = %q", pointers.getKey)
	}
}

func TestAuthenticateRebuildsOnlyAMissingPointer(t *testing.T) {
	fixedNow := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	want := Identity{UserID: 5, SessionID: 8, Version: 2}
	sessions := &fakeSessionStore{
		activeIdentity: want,
		currentSession: Session{ID: 8, UserID: 5, Version: 2, RefreshExpiresAt: fixedNow.Add(time.Hour)},
	}
	pointers := &fakePointerStore{getFound: false}
	service := newTestService(t, &fakeUserStore{}, &fakeRoleStore{}, sessions, pointers)
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now
	accessToken, _, err := service.jwt.Issue(want)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.Authenticate(context.Background(), accessToken); err != nil {
		t.Fatal(err)
	}
	if sessions.findCurrentCalls != 1 || pointers.setKey != "auth:current-session:5" || pointers.setValue != "8" || pointers.setTTL != time.Hour {
		t.Fatalf("rebuild calls=%d pointer=%q,%q,%v", sessions.findCurrentCalls, pointers.setKey, pointers.setValue, pointers.setTTL)
	}
}

func TestAuthenticateReturns503ForRedisErrors(t *testing.T) {
	pointers := &fakePointerStore{getErr: errors.New("redis down")}
	sessions := &fakeSessionStore{}
	service := newTestService(t, &fakeUserStore{}, &fakeRoleStore{}, sessions, pointers)
	accessToken, _, err := service.jwt.Issue(Identity{UserID: 1, SessionID: 2, Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(context.Background(), accessToken); appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if sessions.findCurrentCalls != 0 {
		t.Fatal("Redis error was treated as a cache miss")
	}
}

func TestAuthenticateRejectsAReplacedSession(t *testing.T) {
	pointers := &fakePointerStore{getValue: "99", getFound: true}
	service := newTestService(t, &fakeUserStore{}, &fakeRoleStore{}, &fakeSessionStore{}, pointers)
	accessToken, _, err := service.jwt.Issue(Identity{UserID: 1, SessionID: 2, Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(context.Background(), accessToken); appErrorCode(err) != apperror.CodeUnauthorized {
		t.Fatalf("replaced session error = %v", err)
	}
}

func TestAuthenticateRejectsDisabledUserOrRole(t *testing.T) {
	sessions := &fakeSessionStore{activeErr: gorm.ErrRecordNotFound}
	service := newTestService(t, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePointerStore{getValue: "2", getFound: true})
	accessToken, _, err := service.jwt.Issue(Identity{UserID: 1, SessionID: 2, Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(context.Background(), accessToken); appErrorCode(err) != apperror.CodeUnauthorized {
		t.Fatalf("inactive identity error = %v", err)
	}
}

func TestRefreshRotatesHashAndIncrementsVersion(t *testing.T) {
	fixedNow := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	originalExpiry := fixedNow.Add(2 * time.Hour)
	sessions := &fakeSessionStore{
		refreshSession: Session{ID: 4, UserID: 3, Version: 7, RefreshExpiresAt: originalExpiry},
		rotateSession:  Session{ID: 4, UserID: 3, Version: 8, RefreshExpiresAt: originalExpiry},
		rotated:        true,
	}
	service := newTestService(t, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePointerStore{getValue: "4", getFound: true})
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now

	credential, err := service.Refresh(context.Background(), RefreshInput{RefreshToken: "old-refresh", ClientIP: "127.0.0.2", UserAgent: "agent"})
	if err != nil {
		t.Fatal(err)
	}
	if sessions.rotateOldHash != service.hashRefreshToken("old-refresh") || sessions.rotateNewHash == sessions.rotateOldHash || len(sessions.rotateNewHash) != 64 {
		t.Fatalf("rotation hashes old=%q new=%q", sessions.rotateOldHash, sessions.rotateNewHash)
	}
	if credential.RefreshToken == "" || credential.RefreshToken == "old-refresh" || !credential.RefreshExpiresAt.Equal(originalExpiry) {
		t.Fatalf("credential = %+v", credential)
	}
	identity, err := service.jwt.Parse(credential.AccessToken)
	if err != nil || identity != (Identity{UserID: 3, SessionID: 4, Version: 8}) {
		t.Fatalf("new access identity = %+v,%v", identity, err)
	}
}

func TestRefreshKeepsAbsoluteRefreshExpiry(t *testing.T) {
	fixedNow := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	originalExpiry := fixedNow.Add(30 * time.Minute)
	sessions := &fakeSessionStore{
		refreshSession: Session{ID: 4, UserID: 3, Version: 1, RefreshExpiresAt: originalExpiry},
		rotateSession:  Session{ID: 4, UserID: 3, Version: 2, RefreshExpiresAt: originalExpiry},
		rotated:        true,
	}
	service := newTestService(t, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePointerStore{getValue: "4", getFound: true})
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now
	credential, err := service.Refresh(context.Background(), RefreshInput{RefreshToken: "old-refresh"})
	if err != nil {
		t.Fatal(err)
	}
	if !credential.RefreshExpiresAt.Equal(originalExpiry) {
		t.Fatalf("refresh expiry = %v", credential.RefreshExpiresAt)
	}
}

func TestRefreshRejectsReusedToken(t *testing.T) {
	sessions := &fakeSessionStore{refreshSession: Session{ID: 4, UserID: 3, Version: 1, RefreshExpiresAt: time.Now().Add(time.Hour)}, rotated: false}
	service := newTestService(t, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePointerStore{getValue: "4", getFound: true})
	if _, err := service.Refresh(context.Background(), RefreshInput{RefreshToken: "reused"}); appErrorCode(err) != apperror.CodeUnauthorized {
		t.Fatalf("reused refresh error = %v", err)
	}
}

func TestRefreshRejectsANonCurrentSession(t *testing.T) {
	sessions := &fakeSessionStore{refreshSession: Session{ID: 4, UserID: 3, Version: 1, RefreshExpiresAt: time.Now().Add(time.Hour)}}
	service := newTestService(t, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePointerStore{getValue: "5", getFound: true})
	if _, err := service.Refresh(context.Background(), RefreshInput{RefreshToken: "old"}); appErrorCode(err) != apperror.CodeUnauthorized {
		t.Fatalf("non-current refresh error = %v", err)
	}
	if sessions.rotateCalls != 0 {
		t.Fatal("non-current session was mutated")
	}
}

func TestRefreshReturns503BeforeMutationWhenRedisFails(t *testing.T) {
	sessions := &fakeSessionStore{refreshSession: Session{ID: 4, UserID: 3, Version: 1, RefreshExpiresAt: time.Now().Add(time.Hour)}}
	service := newTestService(t, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePointerStore{getErr: errors.New("redis down")})
	if _, err := service.Refresh(context.Background(), RefreshInput{RefreshToken: "old"}); appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Redis refresh error = %v", err)
	}
	if sessions.rotateCalls != 0 {
		t.Fatal("session mutated after Redis failure")
	}
}

func TestLogoutRevokesPostgreSQLBeforeDeletingPointer(t *testing.T) {
	order := make([]string, 0, 2)
	sessions := &fakeSessionStore{revokeFn: func(context.Context, int64, time.Time) error {
		order = append(order, "revoke")
		return nil
	}}
	pointers := &fakePointerStore{deleteFn: func(_ context.Context, key string) error {
		order = append(order, "delete")
		if key != "auth:current-session:1" {
			t.Fatalf("delete key = %q", key)
		}
		return nil
	}}
	service := newTestService(t, &fakeUserStore{}, &fakeRoleStore{}, sessions, pointers)
	if err := service.Logout(context.Background(), Identity{UserID: 1, SessionID: 2, Version: 1}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "revoke,delete" {
		t.Fatalf("logout order = %v", order)
	}
}

func TestLogoutReturns503WhenPointerDeleteFails(t *testing.T) {
	sessions := &fakeSessionStore{}
	pointers := &fakePointerStore{deleteFn: func(context.Context, string) error { return errors.New("redis down") }}
	service := newTestService(t, &fakeUserStore{}, &fakeRoleStore{}, sessions, pointers)
	if err := service.Logout(context.Background(), Identity{UserID: 1, SessionID: 2, Version: 1}); appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Logout() error = %v", err)
	}
	if len(sessions.revokedIDs) != 1 {
		t.Fatal("PostgreSQL session was not revoked")
	}
}

func TestCurrentUserReturnsOnlyIDUsernameAndEmail(t *testing.T) {
	users := &fakeUserStore{current: user.Current{ID: 1, Username: "admin", Email: "admin@example.com"}}
	service := newTestService(t, users, &fakeRoleStore{}, &fakeSessionStore{}, &fakePointerStore{})
	current, err := service.CurrentUser(context.Background(), Identity{UserID: 1, SessionID: 2, Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if current != users.current {
		t.Fatalf("CurrentUser() = %+v", current)
	}
}

func newTestService(t *testing.T, users userStore, roles roleStore, sessions sessionStore, pointers pointerStore) *Service {
	t.Helper()
	jwtCodec := NewJWT([]byte(strings.Repeat("j", 32)))
	return NewService(users, roles, sessions, pointers, jwtCodec, []byte(strings.Repeat("h", 32)))
}

func appErrorCode(err error) int {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return 0
}

type fakeUserStore struct {
	createFn      func(context.Context, user.CreateInput) (user.User, error)
	credential    user.Credential
	credentialErr error
	current       user.Current
	currentErr    error
}

func (f *fakeUserStore) CreateWithRole(ctx context.Context, input user.CreateInput) (user.User, error) {
	if f.createFn != nil {
		return f.createFn(ctx, input)
	}
	return user.User{}, errors.New("unexpected CreateWithRole call")
}

func (f *fakeUserStore) FindCredentialByUsername(context.Context, string) (user.Credential, error) {
	return f.credential, f.credentialErr
}

func (f *fakeUserStore) FindCurrent(context.Context, int64) (user.Current, error) {
	return f.current, f.currentErr
}

type fakeRoleStore struct {
	defaultRole role.Role
	err         error
}

func (f *fakeRoleStore) FindDefault(context.Context) (role.Role, error) {
	return f.defaultRole, f.err
}

type fakeSessionStore struct {
	createFn         func(context.Context, SessionCreate, time.Time) (Session, error)
	createCalls      int
	revokedIDs       []int64
	revokeErr        error
	revokeFn         func(context.Context, int64, time.Time) error
	activeIdentity   Identity
	activeErr        error
	activeSessionID  int64
	activeVersion    int64
	currentSession   Session
	currentErr       error
	findCurrentCalls int
	refreshSession   Session
	refreshErr       error
	rotateSession    Session
	rotated          bool
	rotateErr        error
	rotateCalls      int
	rotateOldHash    string
	rotateNewHash    string
}

func (f *fakeSessionStore) CreateReplacingActive(ctx context.Context, input SessionCreate, now time.Time) (Session, error) {
	f.createCalls++
	if f.createFn != nil {
		return f.createFn(ctx, input, now)
	}
	return Session{}, errors.New("unexpected CreateReplacingActive call")
}

func (f *fakeSessionStore) FindActiveIdentity(_ context.Context, sessionID int64, version int64, _ time.Time) (Identity, error) {
	f.activeSessionID = sessionID
	f.activeVersion = version
	return f.activeIdentity, f.activeErr
}

func (f *fakeSessionStore) FindCurrentByUser(context.Context, int64, time.Time) (Session, error) {
	f.findCurrentCalls++
	return f.currentSession, f.currentErr
}

func (f *fakeSessionStore) FindByRefreshHash(context.Context, string, time.Time) (Session, error) {
	return f.refreshSession, f.refreshErr
}

func (f *fakeSessionStore) RotateByRefreshHash(_ context.Context, _ int64, oldHash, newHash string, _ time.Time, _, _ string) (Session, bool, error) {
	f.rotateCalls++
	f.rotateOldHash = oldHash
	f.rotateNewHash = newHash
	return f.rotateSession, f.rotated, f.rotateErr
}

func (f *fakeSessionStore) Revoke(ctx context.Context, sessionID int64, now time.Time) error {
	f.revokedIDs = append(f.revokedIDs, sessionID)
	if f.revokeFn != nil {
		return f.revokeFn(ctx, sessionID, now)
	}
	return f.revokeErr
}

type fakePointerStore struct {
	setFn    func(context.Context, string, string, time.Duration) error
	getValue string
	getFound bool
	getErr   error
	getKey   string
	setKey   string
	setValue string
	setTTL   time.Duration
	deleteFn func(context.Context, string) error
}

func (f *fakePointerStore) GetString(_ context.Context, key string) (string, bool, error) {
	f.getKey = key
	return f.getValue, f.getFound, f.getErr
}

func (f *fakePointerStore) SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	f.setKey = key
	f.setValue = value
	f.setTTL = ttl
	if f.setFn != nil {
		return f.setFn(ctx, key, value, ttl)
	}
	return nil
}

func (f *fakePointerStore) Delete(ctx context.Context, key string) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, key)
	}
	return nil
}
