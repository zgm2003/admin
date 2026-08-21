package user_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/module/accessstate"
	"admin/server/internal/module/auth"
	"admin/server/internal/module/authstate"
	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestListServiceValidatesQueryAndReturnsNonNilEmptyPage(t *testing.T) {
	tx, ctx, _ := openUserTransaction(t)
	service := newUserTestService(t, user.NewRepository(tx))
	result, err := service.List(ctx, user.ListQuery{Page: 1, PageSize: 20, Keyword: "missing_user_list_value"})
	if err != nil || result.List == nil || result.Total != 0 || result.Page != 1 || result.PageSize != 20 {
		t.Fatalf("empty List() = %#v,%v", result, err)
	}
	invalidEnabled := yesno.Value(2)
	invalidRoleID := int64(0)
	for _, query := range []user.ListQuery{
		{Page: 0, PageSize: 20},
		{Page: 1, PageSize: 0},
		{Page: 1, PageSize: 101},
		{Page: 1, PageSize: 20, IsEnabled: &invalidEnabled},
		{Page: 1, PageSize: 20, RoleID: &invalidRoleID},
		{Page: 1, PageSize: 20, Keyword: strings.Repeat("界", 255)},
	} {
		_, listErr := service.List(ctx, query)
		var appErr *apperror.Error
		if !errors.As(listErr, &appErr) || appErr.Code != apperror.CodeInvalidRequest {
			t.Errorf("List(%+v) error = %v", query, listErr)
		}
	}
}

func TestListServiceMapsInvalidStoredRelations(t *testing.T) {
	tx, ctx, _ := openUserTransaction(t)
	unique := fmt.Sprintf("service_invalid_%d", time.Now().UnixNano())
	if err := tx.WithContext(ctx).Create(&user.User{Username: unique, Email: unique + "@example.com", PasswordHash: "hash", IsEnabled: yesno.Yes}).Error; err != nil {
		t.Fatal(err)
	}
	_, err := newUserTestService(t, user.NewRepository(tx)).List(ctx, user.ListQuery{Page: 1, PageSize: 20, Keyword: unique})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != user.CodeUserDataInvalid {
		t.Fatalf("invalid relation error = %v", err)
	}
}

func TestRoleOptionsServiceReturnsRepositoryOptions(t *testing.T) {
	tx, ctx, _ := openUserTransaction(t)
	options, err := newUserTestService(t, user.NewRepository(tx)).RoleOptions(ctx)
	if err != nil || options == nil || len(options) == 0 {
		t.Fatalf("RoleOptions() = %#v,%v", options, err)
	}
}

func TestServiceUpdateUsernameAllowsSelfAndIsIdempotent(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created := createListedUser(t, tx, ctx, fmt.Sprintf("self%d", time.Now().UnixNano()), fmt.Sprintf("self%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC().Add(-time.Hour), defaultRole.ID)
	service := newUserTestService(t, user.NewRepository(tx))
	updated, err := service.Update(ctx, created.ID, created.ID, user.UpdateInput{Username: "  新用户名_01  "})
	if err != nil || updated.ID != created.ID || updated.Username != "新用户名_01" || !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Fatalf("Update() = %+v,%v", updated, err)
	}
	idempotent, err := service.Update(ctx, created.ID, created.ID, user.UpdateInput{Username: "新用户名_01"})
	if err != nil || !idempotent.UpdatedAt.Equal(updated.UpdatedAt) {
		t.Fatalf("idempotent Update() = %+v,%v", idempotent, err)
	}
}

func TestServiceUpdateUsernameValidatesTargetAndConflicts(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	actor := createListedUser(t, tx, ctx, fmt.Sprintf("actor%d", time.Now().UnixNano()), fmt.Sprintf("actor%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	target := createListedUser(t, tx, ctx, fmt.Sprintf("target%d", time.Now().UnixNano()), fmt.Sprintf("target%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	service := newUserTestService(t, user.NewRepository(tx))
	for _, name := range []string{"ab", "bad name", strings.Repeat("a", 65)} {
		if _, err := service.Update(ctx, actor.ID, target.ID, user.UpdateInput{Username: name}); appErrorCodeForUser(err) != apperror.CodeInvalidRequest {
			t.Errorf("Update(%q) error = %v", name, err)
		}
	}
	if _, err := service.Update(ctx, actor.ID, 9223372036854770000, user.UpdateInput{Username: "valid_name"}); appErrorCodeForUser(err) != user.CodeUserNotFound {
		t.Fatalf("unknown target error = %v", err)
	}
	if _, err := service.Update(ctx, actor.ID, target.ID, user.UpdateInput{Username: strings.ToUpper(actor.Username)}); appErrorCodeForUser(err) != user.CodeUserUsernameConflict {
		t.Fatalf("conflict error = %v", err)
	}
	if err := tx.WithContext(ctx).Delete(&target).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.Update(ctx, actor.ID, target.ID, user.UpdateInput{Username: "deleted_target"}); appErrorCodeForUser(err) != user.CodeUserNotFound {
		t.Fatalf("deleted target error = %v", err)
	}
	if _, err := service.Update(ctx, actor.ID, actor.ID, user.UpdateInput{Username: target.Username}); err != nil {
		t.Fatalf("soft-deleted username was not reusable: %v", err)
	}
}

func TestServiceUpdateUsernameProtectsSuperAdminTarget(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	superRole, err := roleRepository.FindByCode(ctx, "super_admin")
	if err != nil {
		t.Fatal(err)
	}
	ordinary := createListedUser(t, tx, ctx, fmt.Sprintf("ordinary%d", time.Now().UnixNano()), fmt.Sprintf("ordinary%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	superActor := createListedUser(t, tx, ctx, fmt.Sprintf("superactor%d", time.Now().UnixNano()), fmt.Sprintf("superactor%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), superRole.ID)
	superTarget := createListedUser(t, tx, ctx, fmt.Sprintf("supertarget%d", time.Now().UnixNano()), fmt.Sprintf("supertarget%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), superRole.ID)
	service := newUserTestService(t, user.NewRepository(tx))
	if _, err := service.Update(ctx, ordinary.ID, superTarget.ID, user.UpdateInput{Username: "blocked_super"}); appErrorCodeForUser(err) != user.CodeUserSuperAdminProtected {
		t.Fatalf("ordinary actor error = %v", err)
	}
	updated, err := service.Update(ctx, superActor.ID, superTarget.ID, user.UpdateInput{Username: "allowed_super"})
	if err != nil || updated.Username != "allowed_super" {
		t.Fatalf("super actor Update() = %+v,%v", updated, err)
	}
}

func TestServiceUpdateUsernameMapsRepositoryFailure(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	actor := createListedUser(t, tx, ctx, fmt.Sprintf("failure%d", time.Now().UnixNano()), fmt.Sprintf("failure%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = newUserTestService(t, user.NewRepository(tx)).Update(canceled, actor.ID, actor.ID, user.UpdateInput{Username: "failure_name"})
	if appErrorCodeForUser(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("repository failure error = %v", err)
	}
}

func TestServiceRolesReturnsCompleteOptionsAndStrictCurrentRelations(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	disabled := role.Role{Code: fmt.Sprintf("disabled_%d", time.Now().UnixNano()), Name: "Disabled", IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&disabled).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Model(&role.Role{}).Where("id = ?", disabled.ID).Update("is_enabled", yesno.No).Error; err != nil {
		t.Fatal(err)
	}
	target := createListedUser(t, tx, ctx, fmt.Sprintf("roles%d", time.Now().UnixNano()), fmt.Sprintf("roles%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), disabled.ID, defaultRole.ID)
	result, err := newUserTestService(t, user.NewRepository(tx)).Roles(ctx, target.ID)
	if err != nil || result.User.ID != target.ID || result.Roles == nil || !reflect.DeepEqual(result.RoleIDs, []int64{defaultRole.ID, disabled.ID}) {
		t.Fatalf("Roles() = %+v,%v", result, err)
	}
	foundDisabled := false
	for _, option := range result.Roles {
		if option.ID == disabled.ID && option.IsEnabled == yesno.No {
			foundDisabled = true
		}
	}
	if !foundDisabled {
		t.Fatal("disabled role was omitted from role query")
	}
	if _, err := newUserTestService(t, user.NewRepository(tx)).Roles(ctx, 9223372036854770000); appErrorCodeForUser(err) != user.CodeUserNotFound {
		t.Fatalf("unknown Roles() error = %v", err)
	}
	corrupt := user.User{Username: fmt.Sprintf("corrupt%d", time.Now().UnixNano()), Email: fmt.Sprintf("corrupt%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&corrupt).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := newUserTestService(t, user.NewRepository(tx)).Roles(ctx, corrupt.ID); appErrorCodeForUser(err) != user.CodeUserDataInvalid {
		t.Fatalf("corrupt Roles() error = %v", err)
	}
}

func TestServiceUpdateRolesNormalizesDiffAndPreservesHistory(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	disabled := role.Role{Code: fmt.Sprintf("assigned_%d", time.Now().UnixNano()), Name: "Assigned disabled", IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&disabled).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Model(&role.Role{}).Where("id = ?", disabled.ID).Update("is_enabled", yesno.No).Error; err != nil {
		t.Fatal(err)
	}
	actor := createListedUser(t, tx, ctx, fmt.Sprintf("roleactor%d", time.Now().UnixNano()), fmt.Sprintf("roleactor%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	target := createListedUser(t, tx, ctx, fmt.Sprintf("roletarget%d", time.Now().UnixNano()), fmt.Sprintf("roletarget%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC().Add(-time.Hour), defaultRole.ID)
	service := newUserTestService(t, user.NewRepository(tx))
	beforeVersion := readUserAccessVersion(t, tx, ctx, target.ID)
	count, err := service.UpdateRoles(ctx, actor.ID, target.ID, []int64{disabled.ID, defaultRole.ID, disabled.ID})
	if err != nil || count != 2 {
		t.Fatalf("UpdateRoles() = %d,%v", count, err)
	}
	var firstUpdated user.User
	if err := tx.WithContext(ctx).Take(&firstUpdated, target.ID).Error; err != nil {
		t.Fatal(err)
	}
	relations, err := user.NewRepository(tx).FindUserRoles(ctx, target.ID)
	if err != nil || len(relations) != 2 {
		t.Fatalf("relations = %+v,%v", relations, err)
	}
	if got := readUserAccessVersion(t, tx, ctx, target.ID); got != beforeVersion+1 {
		t.Fatalf("access version = %d, want %d", got, beforeVersion+1)
	}
	count, err = service.UpdateRoles(ctx, actor.ID, target.ID, []int64{defaultRole.ID, disabled.ID})
	if err != nil || count != 2 {
		t.Fatalf("idempotent UpdateRoles() = %d,%v", count, err)
	}
	var idempotent user.User
	if err := tx.WithContext(ctx).Take(&idempotent, target.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !idempotent.UpdatedAt.Equal(firstUpdated.UpdatedAt) {
		t.Fatal("idempotent role save changed updated_at")
	}
	if got := readUserAccessVersion(t, tx, ctx, target.ID); got != beforeVersion+1 {
		t.Fatalf("idempotent access version = %d, want %d", got, beforeVersion+1)
	}

	if _, err := service.UpdateRoles(ctx, actor.ID, target.ID, []int64{disabled.ID}); appErrorCodeForUser(err) != user.CodeUserInvalidRoles {
		t.Fatalf("disabled-only error = %v", err)
	}
	if _, err := service.UpdateRoles(ctx, actor.ID, target.ID, []int64{}); appErrorCodeForUser(err) != user.CodeUserInvalidRoles {
		t.Fatalf("empty error = %v", err)
	}
	if _, err := service.UpdateRoles(ctx, actor.ID, target.ID, []int64{defaultRole.ID, 9223372036854770000}); appErrorCodeForUser(err) != user.CodeUserRoleNotFound {
		t.Fatalf("missing role error = %v", err)
	}
	if _, err := service.UpdateRoles(ctx, target.ID, target.ID, []int64{defaultRole.ID}); appErrorCodeForUser(err) != user.CodeUserSelfOperation {
		t.Fatalf("self role error = %v", err)
	}
}

func TestServiceUpdateRolesProtectsSuperAdministratorAssignments(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	superRole, err := roleRepository.FindByCode(ctx, role.CodeSuperAdmin)
	if err != nil {
		t.Fatal(err)
	}
	ordinary := createListedUser(t, tx, ctx, fmt.Sprintf("grantordinary%d", time.Now().UnixNano()), fmt.Sprintf("grantordinary%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	superActor := createListedUser(t, tx, ctx, fmt.Sprintf("grantactor%d", time.Now().UnixNano()), fmt.Sprintf("grantactor%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), superRole.ID)
	superTarget := createListedUser(t, tx, ctx, fmt.Sprintf("granttarget%d", time.Now().UnixNano()), fmt.Sprintf("granttarget%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), superRole.ID, defaultRole.ID)
	normalTarget := createListedUser(t, tx, ctx, fmt.Sprintf("normalgrant%d", time.Now().UnixNano()), fmt.Sprintf("normalgrant%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	service := newUserTestService(t, user.NewRepository(tx))
	if _, err := service.UpdateRoles(ctx, ordinary.ID, superTarget.ID, []int64{defaultRole.ID}); appErrorCodeForUser(err) != user.CodeUserSuperAdminProtected {
		t.Fatalf("ordinary super-target error = %v", err)
	}
	if _, err := service.UpdateRoles(ctx, ordinary.ID, normalTarget.ID, []int64{defaultRole.ID, superRole.ID}); appErrorCodeForUser(err) != user.CodeUserSuperAdminProtected {
		t.Fatalf("ordinary super grant error = %v", err)
	}
	count, err := service.UpdateRoles(ctx, superActor.ID, normalTarget.ID, []int64{superRole.ID, defaultRole.ID})
	if err != nil || count != 2 {
		t.Fatalf("super grant = %d,%v", count, err)
	}
}

func TestServiceUpdateRolesRedisFailurePreventsPostgreSQLMutation(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	extraRole := role.Role{Code: fmt.Sprintf("redis_role_%d", time.Now().UnixNano()), Name: "Redis role", IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&extraRole).Error; err != nil {
		t.Fatal(err)
	}
	actor := createListedUser(t, tx, ctx, fmt.Sprintf("redisroleactor%d", time.Now().UnixNano()), fmt.Sprintf("redisroleactor%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	target := createListedUser(t, tx, ctx, fmt.Sprintf("redisroletarget%d", time.Now().UnixNano()), fmt.Sprintf("redisroletarget%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	service, _, _, redisClient := newUserMutationTestService(t, user.NewRepository(tx))
	if err := redisClient.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateRoles(ctx, actor.ID, target.ID, []int64{defaultRole.ID, extraRole.ID}); appErrorCodeForUser(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("UpdateRoles() error = %v", err)
	}
	relations, err := user.NewRepository(tx).FindUserRoles(ctx, target.ID)
	if err != nil || len(relations) != 1 || relations[0].RoleID != defaultRole.ID {
		t.Fatalf("roles mutated without Redis coordination: %+v,%v", relations, err)
	}
	if got := readUserAccessVersion(t, tx, ctx, target.ID); got != 1 {
		t.Fatalf("access version mutated without Redis coordination: %d", got)
	}
}

func TestServiceConcurrentSuperAdminRemovalsCannotBothCommit(t *testing.T) {
	db, ctx, roleRepository := openUserDatabase(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	superRole, err := roleRepository.FindByCode(ctx, role.CodeSuperAdmin)
	if err != nil {
		t.Fatal(err)
	}
	first := createListedUser(t, db, ctx, fmt.Sprintf("concurrentsupera%d", time.Now().UnixNano()), fmt.Sprintf("concurrentsupera%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), superRole.ID, defaultRole.ID)
	second := createListedUser(t, db, ctx, fmt.Sprintf("concurrentsuperb%d", time.Now().UnixNano()), fmt.Sprintf("concurrentsuperb%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), superRole.ID, defaultRole.ID)
	t.Cleanup(func() {
		_ = db.Unscoped().Where("user_id IN ?", []int64{first.ID, second.ID}).Delete(&role.UserRole{}).Error
		_ = db.Unscoped().Where("id IN ?", []int64{first.ID, second.ID}).Delete(&user.User{}).Error
	})
	service := newUserTestService(t, user.NewRepository(db))
	start := make(chan struct{})
	errorsByOperation := make(chan error, 2)
	var wait sync.WaitGroup
	for _, operation := range []struct{ actorID, targetID int64 }{
		{actorID: first.ID, targetID: second.ID},
		{actorID: second.ID, targetID: first.ID},
	} {
		wait.Add(1)
		go func(operation struct{ actorID, targetID int64 }) {
			defer wait.Done()
			<-start
			_, updateErr := service.UpdateRoles(ctx, operation.actorID, operation.targetID, []int64{defaultRole.ID})
			errorsByOperation <- updateErr
		}(operation)
	}
	close(start)
	wait.Wait()
	close(errorsByOperation)
	successes := 0
	failures := 0
	for updateErr := range errorsByOperation {
		if updateErr == nil {
			successes++
			continue
		}
		if appErrorCodeForUser(updateErr) == user.CodeUserSuperAdminProtected {
			failures++
			continue
		}
		t.Fatalf("unexpected concurrent error = %v", updateErr)
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("concurrent results successes=%d failures=%d", successes, failures)
	}
}

func TestServiceUpdateStatusDisablesRevokesAndPublishesFreshStates(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	actor := createListedUser(t, tx, ctx, fmt.Sprintf("statusactor%d", time.Now().UnixNano()), fmt.Sprintf("statusactor%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	target := createListedUser(t, tx, ctx, fmt.Sprintf("statustarget%d", time.Now().UnixNano()), fmt.Sprintf("statustarget%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	session := createUserSession(t, tx, ctx, target.ID)
	appSession := createUserSessionForPlatform(t, tx, ctx, target.ID, "app", "550e8400-e29b-41d4-a716-446655440001")
	service, authStates, accessStates, _ := newUserMutationTestService(t, user.NewRepository(tx))
	beforeVersion := readUserAccessVersion(t, tx, ctx, target.ID)
	if err := service.UpdateStatus(ctx, actor.ID, target.ID, yesno.No); err != nil {
		t.Fatal(err)
	}
	assertDisabledUserAndRevokedSessions(t, tx, ctx, target.ID, session.ID, appSession.ID)
	assertReadyUserState(t, authStates, target.ID, false, false)
	assertReadySessionsState(t, authStates, "admin", target.ID)
	assertReadySessionsState(t, authStates, "app", target.ID)
	assertReadyAccessState(t, accessStates, target.ID, beforeVersion+1)
	if got := readUserAccessVersion(t, tx, ctx, target.ID); got != beforeVersion+1 {
		t.Fatalf("disabled access version = %d, want %d", got, beforeVersion+1)
	}
	if err := service.UpdateStatus(ctx, actor.ID, target.ID, yesno.No); err != nil {
		t.Fatalf("repeat disable error = %v", err)
	}
	if got := readUserAccessVersion(t, tx, ctx, target.ID); got != beforeVersion+1 {
		t.Fatalf("repeat disable access version = %d, want %d", got, beforeVersion+1)
	}
	if err := service.UpdateStatus(ctx, actor.ID, target.ID, yesno.Yes); err != nil {
		t.Fatal(err)
	}
	assertReadyUserState(t, authStates, target.ID, true, false)
	assertReadyAccessState(t, accessStates, target.ID, beforeVersion+2)
	if got := readUserAccessVersion(t, tx, ctx, target.ID); got != beforeVersion+2 {
		t.Fatalf("enabled access version = %d, want %d", got, beforeVersion+2)
	}
	if err := service.UpdateStatus(ctx, actor.ID, actor.ID, yesno.No); appErrorCodeForUser(err) != user.CodeUserSelfOperation {
		t.Fatalf("self disable error = %v", err)
	}
}

func TestServiceUpdateStatusRedisFailurePreventsPostgreSQLMutation(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	actor := createListedUser(t, tx, ctx, fmt.Sprintf("redisactor%d", time.Now().UnixNano()), fmt.Sprintf("redisactor%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	target := createListedUser(t, tx, ctx, fmt.Sprintf("redistarget%d", time.Now().UnixNano()), fmt.Sprintf("redistarget%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	session := createUserSession(t, tx, ctx, target.ID)
	service, _, _, redisClient := newUserMutationTestService(t, user.NewRepository(tx))
	if err := redisClient.Close(); err != nil {
		t.Fatal(err)
	}
	err = service.UpdateStatus(ctx, actor.ID, target.ID, yesno.No)
	if appErrorCodeForUser(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Redis failure error = %v", err)
	}
	var stored user.User
	if err := tx.WithContext(ctx).Take(&stored, target.ID).Error; err != nil {
		t.Fatal(err)
	}
	var storedSession auth.Session
	if err := tx.WithContext(ctx).Take(&storedSession, session.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.IsEnabled != yesno.Yes || storedSession.RevokedAt != nil {
		t.Fatalf("PostgreSQL mutated without Redis coordination: user=%+v session=%+v", stored, storedSession)
	}
}

func TestServiceUpdateStatusProtectsSuperAdminTarget(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	superRole, err := roleRepository.FindByCode(ctx, role.CodeSuperAdmin)
	if err != nil {
		t.Fatal(err)
	}
	ordinary := createListedUser(t, tx, ctx, fmt.Sprintf("statusordinary%d", time.Now().UnixNano()), fmt.Sprintf("statusordinary%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	superTarget := createListedUser(t, tx, ctx, fmt.Sprintf("statussuper%d", time.Now().UnixNano()), fmt.Sprintf("statussuper%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), superRole.ID)
	if err := newUserTestService(t, user.NewRepository(tx)).UpdateStatus(ctx, ordinary.ID, superTarget.ID, yesno.No); appErrorCodeForUser(err) != user.CodeUserSuperAdminProtected {
		t.Fatalf("ordinary actor status error = %v", err)
	}
}

func TestServiceDeleteSoftDeletesRelationsRevokesSessionsAndPublishesStates(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	actor := createListedUser(t, tx, ctx, fmt.Sprintf("deleteactor%d", time.Now().UnixNano()), fmt.Sprintf("deleteactor%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	target := createListedUser(t, tx, ctx, fmt.Sprintf("deletetarget%d", time.Now().UnixNano()), fmt.Sprintf("deletetarget%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	session := createUserSession(t, tx, ctx, target.ID)
	service, authStates, accessStates, _ := newUserMutationTestService(t, user.NewRepository(tx))
	beforeVersion := readUserAccessVersion(t, tx, ctx, target.ID)
	if err := service.Delete(ctx, actor.ID, target.ID); err != nil {
		t.Fatal(err)
	}
	var stored user.User
	if err := tx.WithContext(ctx).Unscoped().Take(&stored, target.ID).Error; err != nil {
		t.Fatal(err)
	}
	var relationCount int64
	if err := tx.WithContext(ctx).Model(&role.UserRole{}).Where("user_id = ?", target.ID).Count(&relationCount).Error; err != nil {
		t.Fatal(err)
	}
	var storedSession auth.Session
	if err := tx.WithContext(ctx).Take(&storedSession, session.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !stored.DeletedAt.Valid || relationCount != 0 || storedSession.RevokedAt == nil {
		t.Fatalf("delete state user=%+v relationCount=%d session=%+v", stored, relationCount, storedSession)
	}
	assertReadyUserState(t, authStates, target.ID, false, true)
	assertReadySessionsState(t, authStates, "admin", target.ID)
	assertReadyAccessState(t, accessStates, target.ID, beforeVersion+1)
	if err := service.Delete(ctx, actor.ID, target.ID); err != nil {
		t.Fatalf("idempotent delete = %v", err)
	}
	if got := readUserAccessVersion(t, tx, ctx, target.ID); got != beforeVersion+1 {
		t.Fatalf("idempotent delete access version = %d, want %d", got, beforeVersion+1)
	}
	if err := service.Delete(ctx, actor.ID, 9223372036854770000); appErrorCodeForUser(err) != user.CodeUserNotFound {
		t.Fatalf("unknown delete error = %v", err)
	}
	if err := service.Delete(ctx, actor.ID, actor.ID); appErrorCodeForUser(err) != user.CodeUserSelfOperation {
		t.Fatalf("self delete error = %v", err)
	}
}

func TestServiceDeleteRedisFailurePreventsPostgreSQLMutation(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	actor := createListedUser(t, tx, ctx, fmt.Sprintf("retryactor%d", time.Now().UnixNano()), fmt.Sprintf("retryactor%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	target := createListedUser(t, tx, ctx, fmt.Sprintf("retrytarget%d", time.Now().UnixNano()), fmt.Sprintf("retrytarget%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	service, _, _, redisClient := newUserMutationTestService(t, user.NewRepository(tx))
	if err := redisClient.Close(); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, actor.ID, target.ID); appErrorCodeForUser(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("delete error = %v", err)
	}
	var stored user.User
	if err := tx.WithContext(ctx).Take(&stored, target.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.DeletedAt.Valid {
		t.Fatal("PostgreSQL delete ran without Redis coordination")
	}
}

func createUserSession(t *testing.T, tx *gorm.DB, ctx context.Context, userID int64) auth.Session {
	t.Helper()
	return createUserSessionForPlatform(t, tx, ctx, userID, "admin", "550e8400-e29b-41d4-a716-446655440000")
}

func createUserSessionForPlatform(t *testing.T, tx *gorm.DB, ctx context.Context, userID int64, platform, deviceID string) auth.Session {
	t.Helper()
	session := auth.Session{
		UserID: userID, Platform: platform, DeviceID: deviceID,
		RefreshTokenHash: fmt.Sprintf("%064d", time.Now().UnixNano()), Version: 1,
		ClientIP: "127.0.0.1", UserAgent: "test", RefreshExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	if err := tx.WithContext(ctx).Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	return session
}

func newUserTestService(t *testing.T, repository *user.Repository) *user.Service {
	t.Helper()
	service, _, _, _ := newUserMutationTestService(t, repository)
	return service
}

func newUserMutationTestService(
	t *testing.T,
	repository *user.Repository,
) (*user.Service, *authstate.Store, *accessstate.Store, *projectredis.Client) {
	t.Helper()
	redisClient := openUserTestRedis(t)
	authStates := authstate.NewStore(redisClient)
	accessStates := accessstate.NewStore(redisClient)
	return user.NewService(
		repository,
		authStates,
		authstate.NewInvalidator(authStates),
		accessStates,
		accessstate.NewInvalidator(accessStates),
	), authStates, accessStates, redisClient
}

func openUserTestRedis(t *testing.T) *projectredis.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("Redis integration test")
	}
	if err := godotenv.Load("../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("load server .env: %v", err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatalf("load worker config: %v", err)
	}
	redisURL, err := url.Parse(settings.RedisURL)
	if err != nil {
		t.Fatalf("parse Redis URL: %v", err)
	}
	redisURL.Path = "/14"
	redisURL.RawPath = ""
	client, err := projectredis.Open(context.Background(), redisURL.String())
	if err != nil {
		t.Fatalf("open test Redis database 14: %v", err)
	}
	for _, pattern := range []string{"auth:user-state:*", "auth:sessions-state:*", "authz:access-state:*"} {
		if err := client.ScanDelete(context.Background(), pattern); err != nil {
			_ = client.Close()
			t.Fatalf("clean test Redis database 14: %v", err)
		}
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func readUserAccessVersion(t *testing.T, tx *gorm.DB, ctx context.Context, userID int64) int64 {
	t.Helper()
	var version int64
	result := tx.WithContext(ctx).Raw("SELECT version FROM sys_access_version WHERE user_id = ?", userID).Scan(&version)
	if result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("read access version: rows=%d error=%v", result.RowsAffected, result.Error)
	}
	return version
}

func assertDisabledUserAndRevokedSessions(t *testing.T, tx *gorm.DB, ctx context.Context, userID int64, sessionIDs ...int64) {
	t.Helper()
	var stored user.User
	if err := tx.WithContext(ctx).Take(&stored, userID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.IsEnabled != yesno.No {
		t.Fatalf("status mutation was incomplete: user=%+v", stored)
	}
	for _, sessionID := range sessionIDs {
		var storedSession auth.Session
		if err := tx.WithContext(ctx).Take(&storedSession, sessionID).Error; err != nil {
			t.Fatal(err)
		}
		if storedSession.RevokedAt == nil {
			t.Fatalf("session %d was not revoked: %+v", sessionID, storedSession)
		}
	}
}

func assertReadyUserState(t *testing.T, store *authstate.Store, userID int64, enabled, deleted bool) {
	t.Helper()
	state, found, err := store.ReadUser(context.Background(), userID)
	if err != nil || !found || state.State != authstate.StateReady || state.IsEnabled != enabled || state.Deleted != deleted {
		t.Fatalf("user state = %+v found=%v error=%v", state, found, err)
	}
}

func assertReadySessionsState(t *testing.T, store *authstate.Store, platform string, userID int64) {
	t.Helper()
	state, found, err := store.ReadSessions(context.Background(), platform, userID)
	if err != nil || !found || state.State != authstate.StateReady || state.Generation == "" {
		t.Fatalf("sessions state = %+v found=%v error=%v", state, found, err)
	}
}

func assertReadyAccessState(t *testing.T, store *accessstate.Store, userID, version int64) {
	t.Helper()
	state, found, err := store.Read(context.Background(), userID)
	if err != nil || !found || state.State != accessstate.StateReady || state.Version != version {
		t.Fatalf("access state = %+v found=%v error=%v", state, found, err)
	}
}

func appErrorCodeForUser(err error) int {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return 0
}
