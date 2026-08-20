package user_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"admin/server/internal/module/auth"
	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

func TestListServiceValidatesQueryAndReturnsNonNilEmptyPage(t *testing.T) {
	tx, ctx, _ := openUserTransaction(t)
	service := user.NewService(user.NewRepository(tx), nil)
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
	_, err := user.NewService(user.NewRepository(tx), nil).List(ctx, user.ListQuery{Page: 1, PageSize: 20, Keyword: unique})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != user.CodeUserDataInvalid {
		t.Fatalf("invalid relation error = %v", err)
	}
}

func TestRoleOptionsServiceReturnsRepositoryOptions(t *testing.T) {
	tx, ctx, _ := openUserTransaction(t)
	options, err := user.NewService(user.NewRepository(tx), nil).RoleOptions(ctx)
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
	service := user.NewService(user.NewRepository(tx), nil)
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
	service := user.NewService(user.NewRepository(tx), nil)
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
	service := user.NewService(user.NewRepository(tx), nil)
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
	_, err = user.NewService(user.NewRepository(tx), nil).Update(canceled, actor.ID, actor.ID, user.UpdateInput{Username: "failure_name"})
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
	result, err := user.NewService(user.NewRepository(tx), nil).Roles(ctx, target.ID)
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
	if _, err := user.NewService(user.NewRepository(tx), nil).Roles(ctx, 9223372036854770000); appErrorCodeForUser(err) != user.CodeUserNotFound {
		t.Fatalf("unknown Roles() error = %v", err)
	}
	corrupt := user.User{Username: fmt.Sprintf("corrupt%d", time.Now().UnixNano()), Email: fmt.Sprintf("corrupt%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&corrupt).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := user.NewService(user.NewRepository(tx), nil).Roles(ctx, corrupt.ID); appErrorCodeForUser(err) != user.CodeUserDataInvalid {
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
	pointers := &recordingSessionPointers{}
	service := user.NewService(user.NewRepository(tx), pointers)
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
	if len(pointers.deletedKeys) != 0 {
		t.Fatalf("role save touched Redis: %v", pointers.deletedKeys)
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
	service := user.NewService(user.NewRepository(tx), nil)
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
	service := user.NewService(user.NewRepository(db), nil)
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

func TestServiceUpdateStatusDisablesRevokesAndCleansPointerAfterCommit(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	actor := createListedUser(t, tx, ctx, fmt.Sprintf("statusactor%d", time.Now().UnixNano()), fmt.Sprintf("statusactor%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	target := createListedUser(t, tx, ctx, fmt.Sprintf("statustarget%d", time.Now().UnixNano()), fmt.Sprintf("statustarget%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	session := createUserSession(t, tx, ctx, target.ID)
	pointers := &recordingSessionPointers{deleteFn: func(key string) error {
		var stored user.User
		if err := tx.WithContext(ctx).Take(&stored, target.ID).Error; err != nil {
			t.Fatal(err)
		}
		var storedSession auth.Session
		if err := tx.WithContext(ctx).Take(&storedSession, session.ID).Error; err != nil {
			t.Fatal(err)
		}
		if stored.IsEnabled != yesno.No || storedSession.RevokedAt == nil {
			t.Fatal("Redis cleanup ran before PostgreSQL reached the safe state")
		}
		return nil
	}}
	service := user.NewService(user.NewRepository(tx), pointers)
	if err := service.UpdateStatus(ctx, actor.ID, target.ID, yesno.No); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pointers.deletedKeys, []string{user.CurrentSessionPointerKey(target.ID)}) {
		t.Fatalf("deleted keys = %v", pointers.deletedKeys)
	}
	if err := service.UpdateStatus(ctx, actor.ID, target.ID, yesno.No); err != nil {
		t.Fatalf("repeat disable error = %v", err)
	}
	if len(pointers.deletedKeys) != 2 {
		t.Fatal("repeat disable did not retry pointer cleanup")
	}
	if err := service.UpdateStatus(ctx, actor.ID, target.ID, yesno.Yes); err != nil {
		t.Fatal(err)
	}
	if len(pointers.deletedKeys) != 2 {
		t.Fatal("enable wrote Redis")
	}
	if err := service.UpdateStatus(ctx, actor.ID, actor.ID, yesno.No); appErrorCodeForUser(err) != user.CodeUserSelfOperation {
		t.Fatalf("self disable error = %v", err)
	}
}

func TestServiceUpdateStatusRedisFailureKeepsPostgreSQLSafe(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	actor := createListedUser(t, tx, ctx, fmt.Sprintf("redisactor%d", time.Now().UnixNano()), fmt.Sprintf("redisactor%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	target := createListedUser(t, tx, ctx, fmt.Sprintf("redistarget%d", time.Now().UnixNano()), fmt.Sprintf("redistarget%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	session := createUserSession(t, tx, ctx, target.ID)
	pointers := &recordingSessionPointers{deleteErr: errors.New("redis unavailable")}
	err = user.NewService(user.NewRepository(tx), pointers).UpdateStatus(ctx, actor.ID, target.ID, yesno.No)
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
	if stored.IsEnabled != yesno.No || storedSession.RevokedAt == nil {
		t.Fatalf("unsafe PostgreSQL state: user=%+v session=%+v", stored, storedSession)
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
	if err := user.NewService(user.NewRepository(tx), &recordingSessionPointers{}).UpdateStatus(ctx, ordinary.ID, superTarget.ID, yesno.No); appErrorCodeForUser(err) != user.CodeUserSuperAdminProtected {
		t.Fatalf("ordinary actor status error = %v", err)
	}
}

func TestServiceDeleteSoftDeletesRelationsRevokesSessionsAndRetriesRedis(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	actor := createListedUser(t, tx, ctx, fmt.Sprintf("deleteactor%d", time.Now().UnixNano()), fmt.Sprintf("deleteactor%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	target := createListedUser(t, tx, ctx, fmt.Sprintf("deletetarget%d", time.Now().UnixNano()), fmt.Sprintf("deletetarget%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	session := createUserSession(t, tx, ctx, target.ID)
	pointers := &recordingSessionPointers{}
	service := user.NewService(user.NewRepository(tx), pointers)
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
	if !stored.DeletedAt.Valid || relationCount != 0 || storedSession.RevokedAt == nil || len(pointers.deletedKeys) != 1 {
		t.Fatalf("delete state user=%+v relationCount=%d session=%+v keys=%v", stored, relationCount, storedSession, pointers.deletedKeys)
	}
	if err := service.Delete(ctx, actor.ID, target.ID); err != nil || len(pointers.deletedKeys) != 2 {
		t.Fatalf("idempotent delete = %v keys=%v", err, pointers.deletedKeys)
	}
	if err := service.Delete(ctx, actor.ID, 9223372036854770000); appErrorCodeForUser(err) != user.CodeUserNotFound {
		t.Fatalf("unknown delete error = %v", err)
	}
	if err := service.Delete(ctx, actor.ID, actor.ID); appErrorCodeForUser(err) != user.CodeUserSelfOperation {
		t.Fatalf("self delete error = %v", err)
	}
}

func TestServiceDeleteRedisFailureIsRetryable(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	actor := createListedUser(t, tx, ctx, fmt.Sprintf("retryactor%d", time.Now().UnixNano()), fmt.Sprintf("retryactor%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	target := createListedUser(t, tx, ctx, fmt.Sprintf("retrytarget%d", time.Now().UnixNano()), fmt.Sprintf("retrytarget%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	pointers := &recordingSessionPointers{deleteErr: errors.New("redis unavailable")}
	service := user.NewService(user.NewRepository(tx), pointers)
	if err := service.Delete(ctx, actor.ID, target.ID); appErrorCodeForUser(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("first delete error = %v", err)
	}
	pointers.deleteErr = nil
	if err := service.Delete(ctx, actor.ID, target.ID); err != nil {
		t.Fatalf("retry delete error = %v", err)
	}
}

func createUserSession(t *testing.T, tx *gorm.DB, ctx context.Context, userID int64) auth.Session {
	t.Helper()
	session := auth.Session{UserID: userID, RefreshTokenHash: fmt.Sprintf("%064d", time.Now().UnixNano()), Version: 1, ClientIP: "127.0.0.1", UserAgent: "test", RefreshExpiresAt: time.Now().UTC().Add(time.Hour)}
	if err := tx.WithContext(ctx).Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	return session
}

type recordingSessionPointers struct {
	deletedKeys []string
	deleteErr   error
	deleteFn    func(string) error
}

func (r *recordingSessionPointers) Delete(_ context.Context, key string) error {
	r.deletedKeys = append(r.deletedKeys, key)
	if r.deleteFn != nil {
		return r.deleteFn(key)
	}
	return r.deleteErr
}

func appErrorCodeForUser(err error) int {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return 0
}
