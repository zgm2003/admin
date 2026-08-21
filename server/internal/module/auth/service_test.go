package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/authclient"
	"admin/server/internal/module/authplatform"
	"admin/server/internal/module/authstate"
	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

func TestRegisterUsesPlatformPolicyAndRequiresRedis(t *testing.T) {
	redisClient := openAuthRedis(t)
	users := &fakeUserStore{createFn: func(_ context.Context, input user.CreateInput) (user.User, error) {
		return user.User{ID: 81001, Username: input.Username, Email: input.Email, IsEnabled: yesno.Yes}, nil
	}}
	service := newRedisTestService(t, redisClient, users, &fakeRoleStore{defaultRole: role.Role{ID: 3}}, &fakeSessionStore{}, &fakePolicyStore{policy: testPolicy()})
	registered, err := service.Register(context.Background(), RegisterInput{
		Username: "  张三_01  ", Email: " USER@Example.COM ", Password: "password123", ConfirmPassword: "password123", Client: testAuthClient(),
	})
	if err != nil || registered.UserID != 81001 || users.created.RoleID != 3 || users.created.PasswordHash == "" {
		t.Fatalf("Register() = %+v,%v input=%+v", registered, err, users.created)
	}

	disabled := testPolicy()
	disabled.AllowRegister = false
	service = newRedisTestService(t, redisClient, users, &fakeRoleStore{defaultRole: role.Role{ID: 3}}, &fakeSessionStore{}, &fakePolicyStore{policy: disabled})
	if _, err := service.Register(context.Background(), RegisterInput{
		Username: "valid_user", Email: "valid@example.com", Password: "password123", ConfirmPassword: "password123", Client: testAuthClient(),
	}); appErrorCode(err) != apperror.CodeForbidden {
		t.Fatalf("registration-disabled error = %v", err)
	}
}

func TestRegisterMapsValidationAndConflicts(t *testing.T) {
	redisClient := openAuthRedis(t)
	for _, input := range []RegisterInput{
		{Username: "ab", Email: "user@example.com", Password: "password", ConfirmPassword: "password", Client: testAuthClient()},
		{Username: "valid_user", Email: "not-an-email", Password: "password", ConfirmPassword: "password", Client: testAuthClient()},
		{Username: "valid_user", Email: "user@example.com", Password: "password", ConfirmPassword: "different", Client: testAuthClient()},
	} {
		service := newRedisTestService(t, redisClient, &fakeUserStore{}, &fakeRoleStore{}, &fakeSessionStore{}, &fakePolicyStore{policy: testPolicy()})
		if _, err := service.Register(context.Background(), input); appErrorCode(err) != apperror.CodeInvalidRequest {
			t.Errorf("Register(%+v) error = %v", input, err)
		}
	}
	for _, repositoryError := range []error{user.ErrUsernameConflict, user.ErrEmailConflict} {
		users := &fakeUserStore{createErr: repositoryError}
		service := newRedisTestService(t, redisClient, users, &fakeRoleStore{defaultRole: role.Role{ID: 1}}, &fakeSessionStore{}, &fakePolicyStore{policy: testPolicy()})
		_, err := service.Register(context.Background(), RegisterInput{
			Username: "valid_user", Email: "user@example.com", Password: "password", ConfirmPassword: "password", Client: testAuthClient(),
		})
		if appErrorCode(err) != apperror.CodeConflict {
			t.Errorf("repository error %v mapped to %v", repositoryError, err)
		}
	}
}

func TestLoginUsesPolicyTTLAndPublishesSessionSnapshot(t *testing.T) {
	redisClient := openAuthRedis(t)
	cleanupAuthRedisKeys(t, redisClient, 82001, "admin", 82002)
	passwordHash, err := HashPassword("password")
	if err != nil {
		t.Fatal(err)
	}
	fixedNow := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	policy := testPolicy()
	policy.AccessTTL = 23 * time.Minute
	policy.RefreshTTL = 5 * time.Minute
	sessions := &fakeSessionStore{createSession: Session{ID: 82002, UserID: 82001, Platform: "admin", DeviceID: testAuthClient().DeviceID, Version: 1, ClientIP: "127.0.0.1"}}
	users := &fakeUserStore{credential: user.Credential{ID: 82001, Username: "admin", Email: "admin@example.com", PasswordHash: passwordHash, IsEnabled: yesno.Yes}}
	service := newRedisTestService(t, redisClient, users, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now

	credential, err := service.Login(context.Background(), LoginInput{Username: "admin", Password: "password", Client: testAuthClient()})
	if err != nil {
		t.Fatal(err)
	}
	if credential.ExpiresIn != int(policy.AccessTTL.Seconds()) || !credential.RefreshExpiresAt.Equal(fixedNow.Add(policy.RefreshTTL)) {
		t.Fatalf("credential = %+v", credential)
	}
	identity, err := service.jwt.Parse(credential.AccessToken)
	if err != nil || identity.SessionID != 82002 || identity.Platform != "admin" {
		t.Fatalf("access identity = %+v,%v", identity, err)
	}
	state, found, err := service.states.ReadSessions(context.Background(), "admin", 82001)
	if err != nil || !found || state.State != authstate.StateReady {
		t.Fatalf("sessions state = %+v,%v,%v", state, found, err)
	}
	snapshot, found, err := service.sessionCache.Read(context.Background(), "admin", 82002)
	if err != nil || !found || snapshot.SessionVersion != 1 || snapshot.SessionsGeneration != state.Generation {
		t.Fatalf("session snapshot = %+v,%v,%v", snapshot, found, err)
	}
	ttl, found, err := redisClient.TTL(context.Background(), SessionKey("admin", 82002))
	if err != nil || !found || ttl > policy.RefreshTTL || ttl < policy.RefreshTTL-30*time.Second {
		t.Fatalf("session snapshot TTL = %v,%v,%v", ttl, found, err)
	}
}

func TestAuthenticateUsesWarmRedisWithoutPostgreSQL(t *testing.T) {
	redisClient := openAuthRedis(t)
	cleanupAuthRedisKeys(t, redisClient, 83001, "admin", 83002)
	policy := testPolicy()
	sessions := &fakeSessionStore{}
	service := newRedisTestService(t, redisClient, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	fixedNow := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now
	installWarmSession(t, service, SessionAuthority{
		Session: Session{ID: 83002, UserID: 83001, Platform: "admin", DeviceID: testAuthClient().DeviceID, Version: 4, ClientIP: "127.0.0.1", RefreshExpiresAt: fixedNow.Add(time.Hour)},
		UserID:  83001, UserIsEnabled: yesno.Yes,
	}, policy)
	rawToken, _, err := service.jwt.Issue(TokenIdentity{UserID: 83001, SessionID: 83002, Platform: "admin", Version: 4}, policy.AccessTTL)
	if err != nil {
		t.Fatal(err)
	}

	identity, err := service.Authenticate(context.Background(), rawToken, testAuthClient())
	if err != nil {
		t.Fatal(err)
	}
	if sessions.authorityCalls != 0 || identity.UserID != 83001 || identity.PolicyVersion != policy.PolicyVersion || identity.AccessCacheTTL != policy.AccessCacheTTL || identity.CacheResult != "hit" {
		t.Fatalf("Authenticate() = %+v calls=%d", identity, sessions.authorityCalls)
	}
}

func TestAuthenticateFallsBackToPostgreSQLAndRebuildsRedis(t *testing.T) {
	redisClient := openAuthRedis(t)
	cleanupAuthRedisKeys(t, redisClient, 84001, "admin", 84002)
	fixedNow := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	policy := testPolicy()
	authority := SessionAuthority{
		Session: Session{ID: 84002, UserID: 84001, Platform: "admin", DeviceID: testAuthClient().DeviceID, Version: 2, ClientIP: "127.0.0.1", RefreshExpiresAt: fixedNow.Add(time.Hour)},
		UserID:  84001, UserIsEnabled: yesno.Yes,
	}
	sessions := &fakeSessionStore{authority: authority}
	service := newRedisTestService(t, redisClient, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now
	rawToken, _, _ := service.jwt.Issue(TokenIdentity{UserID: 84001, SessionID: 84002, Platform: "admin", Version: 2}, policy.AccessTTL)

	first, err := service.Authenticate(context.Background(), rawToken, testAuthClient())
	if err != nil || first.CacheResult != "miss" || sessions.authorityCalls != 1 {
		t.Fatalf("fallback Authenticate() = %+v,%v calls=%d", first, err, sessions.authorityCalls)
	}
	second, err := service.Authenticate(context.Background(), rawToken, testAuthClient())
	if err != nil || second.CacheResult != "hit" || sessions.authorityCalls != 1 {
		t.Fatalf("warm Authenticate() = %+v,%v calls=%d", second, err, sessions.authorityCalls)
	}
}

func TestAuthenticateRepairsCorruptStateAfterPostgreSQLFallback(t *testing.T) {
	redisClient := openAuthRedis(t)
	cleanupAuthRedisKeys(t, redisClient, 84501, "admin", 84502)
	fixedNow := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	policy := testPolicy()
	authority := SessionAuthority{
		Session: Session{ID: 84502, UserID: 84501, Platform: "admin", DeviceID: testAuthClient().DeviceID, Version: 2, ClientIP: "127.0.0.1", RefreshExpiresAt: fixedNow.Add(time.Hour)},
		UserID:  84501, UserIsEnabled: yesno.Yes,
	}
	sessions := &fakeSessionStore{authority: authority}
	service := newRedisTestService(t, redisClient, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now
	if err := redisClient.SetString(context.Background(), authstate.UserStateKey(84501), `{"schemaVersion":1,"state":"ready","unknown":true}`, 0); err != nil {
		t.Fatal(err)
	}
	rawToken, _, _ := service.jwt.Issue(TokenIdentity{UserID: 84501, SessionID: 84502, Platform: "admin", Version: 2}, policy.AccessTTL)

	first, err := service.Authenticate(context.Background(), rawToken, testAuthClient())
	if err != nil || sessions.authorityCalls != 1 {
		t.Fatalf("corrupt fallback = %+v,%v calls=%d", first, err, sessions.authorityCalls)
	}
	second, err := service.Authenticate(context.Background(), rawToken, testAuthClient())
	if err != nil || second.CacheResult != "hit" || sessions.authorityCalls != 1 {
		t.Fatalf("repaired cache = %+v,%v calls=%d", second, err, sessions.authorityCalls)
	}
}

func TestAuthenticateUsesPostgreSQLAuthorityWhenRedisFails(t *testing.T) {
	redisClient := openAuthRedis(t)
	fixedNow := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	policy := testPolicy()
	authority := SessionAuthority{
		Session: Session{ID: 85002, UserID: 85001, Platform: "admin", DeviceID: testAuthClient().DeviceID, Version: 1, ClientIP: "127.0.0.1", RefreshExpiresAt: fixedNow.Add(time.Hour)},
		UserID:  85001, UserIsEnabled: yesno.Yes,
	}
	sessions := &fakeSessionStore{authority: authority}
	service := newRedisTestService(t, redisClient, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now
	rawToken, _, _ := service.jwt.Issue(TokenIdentity{UserID: 85001, SessionID: 85002, Platform: "admin", Version: 1}, policy.AccessTTL)
	if err := redisClient.Close(); err != nil {
		t.Fatal(err)
	}

	identity, err := service.Authenticate(context.Background(), rawToken, testAuthClient())
	if err != nil || identity.UserID != 85001 || identity.CacheResult != "error" || sessions.authorityCalls != 1 {
		t.Fatalf("Redis error fallback = %+v,%v calls=%d", identity, err, sessions.authorityCalls)
	}
	sessions.authorityErr = errors.New("postgres down")
	if _, err := service.Authenticate(context.Background(), rawToken, testAuthClient()); appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Redis and PostgreSQL error = %v", err)
	}
}

func TestAuthenticateRejectsInvalidatingStateWithoutPostgreSQL(t *testing.T) {
	redisClient := openAuthRedis(t)
	cleanupAuthRedisKeys(t, redisClient, 86001, "admin", 86002)
	policy := testPolicy()
	sessions := &fakeSessionStore{}
	service := newRedisTestService(t, redisClient, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	fixedNow := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now
	userFact := authstate.UserFact{UserID: 86001, Generation: "user-ready", IsEnabled: true}
	_, _, _ = service.states.InstallUserReadyIfMissing(context.Background(), userFact)
	lease, err := service.invalidator.Acquire(context.Background(), authstate.MutationFacts{Users: []authstate.UserFact{userFact}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lease.Rollback(context.Background()) })
	rawToken, _, _ := service.jwt.Issue(TokenIdentity{UserID: 86001, SessionID: 86002, Platform: "admin", Version: 1}, policy.AccessTTL)

	_, err = service.Authenticate(context.Background(), rawToken, testAuthClient())
	if appErrorCode(err) != authplatform.CodeSessionUpdating || sessions.authorityCalls != 0 {
		t.Fatalf("invalidating Authenticate() error=%v calls=%d", err, sessions.authorityCalls)
	}
}

func TestAuthenticateRevokesDeviceMismatchOnlyAfterRedisInvalidation(t *testing.T) {
	redisClient := openAuthRedis(t)
	cleanupAuthRedisKeys(t, redisClient, 86501, "admin", 86502)
	policy := testPolicy()
	policy.BindDevice = true
	fixedNow := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	authority := SessionAuthority{
		Session: Session{ID: 86502, UserID: 86501, Platform: "admin", DeviceID: testAuthClient().DeviceID, Version: 1, ClientIP: "127.0.0.1", RefreshExpiresAt: fixedNow.Add(time.Hour)},
		UserID:  86501, UserIsEnabled: yesno.Yes,
	}
	sessions := &fakeSessionStore{authority: authority}
	service := newRedisTestService(t, redisClient, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now
	installWarmSession(t, service, authority, policy)
	rawToken, _, _ := service.jwt.Issue(TokenIdentity{UserID: 86501, SessionID: 86502, Platform: "admin", Version: 1}, policy.AccessTTL)
	mismatched := testAuthClient()
	mismatched.DeviceID = "650e8400-e29b-41d4-a716-446655440000"

	if _, err := service.Authenticate(context.Background(), rawToken, mismatched); appErrorCode(err) != apperror.CodeUnauthorized {
		t.Fatalf("device mismatch error = %v", err)
	}
	if sessions.revokeCalls != 1 {
		t.Fatalf("device mismatch revoke calls = %d", sessions.revokeCalls)
	}
	if _, found, err := service.sessionCache.Read(context.Background(), "admin", 86502); err != nil || found {
		t.Fatalf("revoked snapshot still available = %v,%v", found, err)
	}

	closedRedis := openAuthRedis(t)
	closedSessions := &fakeSessionStore{authority: authority}
	closedService := newRedisTestService(t, closedRedis, &fakeUserStore{}, &fakeRoleStore{}, closedSessions, &fakePolicyStore{policy: policy})
	closedService.now = func() time.Time { return fixedNow }
	closedService.jwt.now = closedService.now
	if err := closedRedis.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := closedService.Authenticate(context.Background(), rawToken, mismatched); appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Redis failure device mismatch error = %v", err)
	}
	if closedSessions.revokeCalls != 0 {
		t.Fatal("device mismatch revoked PostgreSQL after Redis coordination failure")
	}
}

func TestRefreshRotatesWithinSessionInvalidationAndKeepsAbsoluteExpiry(t *testing.T) {
	redisClient := openAuthRedis(t)
	cleanupAuthRedisKeys(t, redisClient, 86801, "admin", 86802)
	policy := testPolicy()
	policy.AccessTTL = 7 * time.Minute
	fixedNow := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	expiresAt := fixedNow.Add(2 * time.Hour)
	authority := SessionAuthority{
		Session: Session{ID: 86802, UserID: 86801, Platform: "admin", DeviceID: testAuthClient().DeviceID, RefreshTokenHash: strings.Repeat("a", 64), Version: 3, ClientIP: "127.0.0.1", RefreshExpiresAt: expiresAt},
		UserID:  86801, UserIsEnabled: yesno.Yes,
	}
	rotated := authority.Session
	rotated.Version++
	sessions := &fakeSessionStore{refresh: authority, rotateSession: rotated, rotateWon: true}
	service := newRedisTestService(t, redisClient, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	service.now = func() time.Time { return fixedNow }
	service.jwt.now = service.now
	_, _, _ = service.states.InstallUserReadyIfMissing(context.Background(), authstate.UserFact{UserID: 86801, Generation: "user-ready", IsEnabled: true})
	_, _, _ = service.states.InstallSessionsReadyIfMissing(context.Background(), authstate.SessionsFact{Platform: "admin", UserID: 86801, Generation: "sessions-ready"})

	credential, err := service.Refresh(context.Background(), RefreshInput{RefreshToken: "old-refresh", Client: testAuthClient()})
	if err != nil {
		t.Fatal(err)
	}
	if sessions.rotateCalls != 1 || credential.ExpiresIn != int(policy.AccessTTL.Seconds()) || !credential.RefreshExpiresAt.Equal(expiresAt) {
		t.Fatalf("Refresh() = %+v rotateCalls=%d", credential, sessions.rotateCalls)
	}
	identity, err := service.jwt.Parse(credential.AccessToken)
	if err != nil || identity.Version != rotated.Version || identity.SessionID != rotated.ID {
		t.Fatalf("rotated identity = %+v,%v", identity, err)
	}
}

func TestLogoutRequiresRedisInvalidationBeforePostgreSQL(t *testing.T) {
	redisClient := openAuthRedis(t)
	cleanupAuthRedisKeys(t, redisClient, 87001, "admin", 87002)
	policy := testPolicy()
	sessions := &fakeSessionStore{}
	service := newRedisTestService(t, redisClient, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	_, _, _ = service.states.InstallSessionsReadyIfMissing(context.Background(), authstate.SessionsFact{Platform: "admin", UserID: 87001, Generation: "sessions-ready"})
	if err := service.Logout(context.Background(), Identity{UserID: 87001, SessionID: 87002, Platform: "admin", Version: 1}, testAuthClient()); err != nil {
		t.Fatal(err)
	}
	if sessions.revokeCalls != 1 {
		t.Fatalf("revoke calls = %d", sessions.revokeCalls)
	}

	closedRedis := openAuthRedis(t)
	closedService := newRedisTestService(t, closedRedis, &fakeUserStore{}, &fakeRoleStore{}, sessions, &fakePolicyStore{policy: policy})
	if err := closedRedis.Close(); err != nil {
		t.Fatal(err)
	}
	if err := closedService.Logout(context.Background(), Identity{UserID: 87001, SessionID: 87003, Platform: "admin", Version: 1}, testAuthClient()); appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Redis failure Logout() = %v", err)
	}
	if sessions.revokeCalls != 1 {
		t.Fatal("PostgreSQL revoke ran after Redis coordination failure")
	}
}

func TestCurrentUserReturnsClosedIdentity(t *testing.T) {
	redisClient := openAuthRedis(t)
	users := &fakeUserStore{current: user.Current{ID: 1, Username: "admin", Email: "admin@example.com"}}
	service := newRedisTestService(t, redisClient, users, &fakeRoleStore{}, &fakeSessionStore{}, &fakePolicyStore{policy: testPolicy()})
	current, err := service.CurrentUser(context.Background(), Identity{UserID: 1, SessionID: 2, Platform: "admin", Version: 1})
	if err != nil || current != users.current {
		t.Fatalf("CurrentUser() = %+v,%v", current, err)
	}
}

func newRedisTestService(t *testing.T, redisClient *projectredis.Client, users userStore, roles roleStore, sessions sessionStore, policies policyStore) *Service {
	t.Helper()
	states := authstate.NewStore(redisClient)
	return NewService(
		users, roles, sessions, policies, states, authstate.NewInvalidator(states), NewSessionCache(redisClient), redisClient,
		NewJWT([]byte(strings.Repeat("j", 32))), []byte(strings.Repeat("h", 32)), slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
}

func testPolicy() authplatform.Policy {
	return authplatform.Policy{
		ID: 1, Code: "admin", Name: "Admin", PolicyVersion: 1, AccessTTL: 15 * time.Minute,
		RefreshTTL: 14 * 24 * time.Hour, SessionCacheTTL: 30 * time.Minute, AccessCacheTTL: 30 * time.Minute,
		MaxSessions: 1, AllowRegister: true, IsEnabled: true, IsBuiltin: true,
	}
}

func installWarmSession(t *testing.T, service *Service, authority SessionAuthority, policy authplatform.Policy) {
	t.Helper()
	userGeneration, _ := authstate.NewGeneration()
	sessionsGeneration, _ := authstate.NewGeneration()
	userFact := authstate.UserFact{UserID: authority.UserID, Generation: userGeneration, IsEnabled: authority.UserIsEnabled == yesno.Yes, Deleted: authority.UserDeleted}
	sessionsFact := authstate.SessionsFact{Platform: authority.Session.Platform, UserID: authority.UserID, Generation: sessionsGeneration}
	if _, _, err := service.states.InstallUserReadyIfMissing(context.Background(), userFact); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.states.InstallSessionsReadyIfMissing(context.Background(), sessionsFact); err != nil {
		t.Fatal(err)
	}
	snapshot := snapshotFromAuthority(authority, policy, userFact.Generation, sessionsFact.Generation)
	if published, err := service.sessionCache.PublishIfCurrent(context.Background(), snapshot, policy.SessionCacheTTL); err != nil || !published {
		t.Fatalf("publish warm session = %v,%v", published, err)
	}
}

func cleanupAuthRedisKeys(t *testing.T, client *projectredis.Client, userID int64, platform string, sessionID int64) {
	t.Helper()
	keys := []string{authstate.UserStateKey(userID), authstate.SessionsStateKey(platform, userID), SessionKey(platform, sessionID)}
	if err := client.DeleteMany(context.Background(), keys); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.DeleteMany(context.Background(), keys) })
}

func appErrorCode(err error) int {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return 0
}

type fakePolicyStore struct {
	policy authplatform.Policy
	err    error
	calls  int
}

func (f *fakePolicyStore) CurrentPolicy(context.Context, string) (authplatform.Policy, error) {
	f.calls++
	return f.policy, f.err
}

type fakeUserStore struct {
	created       user.CreateInput
	createFn      func(context.Context, user.CreateInput) (user.User, error)
	createErr     error
	credential    user.Credential
	credentialErr error
	current       user.Current
	currentErr    error
}

func (f *fakeUserStore) CreateWithRole(ctx context.Context, input user.CreateInput) (user.User, error) {
	f.created = input
	if f.createFn != nil {
		return f.createFn(ctx, input)
	}
	return user.User{}, f.createErr
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
	createSession  Session
	createRevoked  []Session
	createErr      error
	createCalls    int
	authority      SessionAuthority
	authorityErr   error
	authorityCalls int
	refresh        SessionAuthority
	refreshErr     error
	rotateSession  Session
	rotateWon      bool
	rotateErr      error
	rotateCalls    int
	revokeCalls    int
	revokeErr      error
}

func (f *fakeSessionStore) CreateWithinLimit(context.Context, SessionCreate, authplatform.Policy, time.Time) (Session, []Session, error) {
	f.createCalls++
	return f.createSession, f.createRevoked, f.createErr
}

func (f *fakeSessionStore) FindAuthoritative(context.Context, TokenIdentity, time.Time) (SessionAuthority, error) {
	f.authorityCalls++
	return f.authority, f.authorityErr
}

func (f *fakeSessionStore) FindByRefreshHash(context.Context, string, string, time.Time) (SessionAuthority, error) {
	return f.refresh, f.refreshErr
}

func (f *fakeSessionStore) RotateByRefreshHash(context.Context, int64, string, string, string, time.Time, authclient.Client) (Session, bool, error) {
	f.rotateCalls++
	return f.rotateSession, f.rotateWon, f.rotateErr
}

func (f *fakeSessionStore) Revoke(context.Context, int64, time.Time) error {
	f.revokeCalls++
	return f.revokeErr
}

func testAuthClient() authclient.Client {
	return authclient.Client{Platform: "admin", DeviceID: "550e8400-e29b-41d4-a716-446655440000", ClientIP: "127.0.0.1", UserAgent: "test-agent"}
}

var _ = gorm.ErrRecordNotFound
var _ = i18n.KeyUnauthorized
