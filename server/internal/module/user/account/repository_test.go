package account_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/auth/login"
	authplatform "admin/server/internal/module/auth/platform"
	"admin/server/internal/module/rbac/access"
	"admin/server/internal/module/rbac/role"
	"admin/server/internal/module/user/account"
	"admin/server/internal/module/user/profile"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestCreateWithRolePersistsUserAndRoleAtomically(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repository := account.NewRepository(tx)
	input := newCreateInput("persist", defaultRole.ID)

	created, err := repository.CreateWithRole(ctx, input)
	if err != nil {
		t.Fatalf("CreateWithRole() error = %v", err)
	}
	if created.ID <= 0 || created.Username != input.Username || created.Email != input.Email || created.IsEnabled != yesno.Yes {
		t.Fatalf("created user = %+v", created)
	}
	var relation role.UserRole
	if err := tx.WithContext(ctx).Where("user_id = ? AND role_id = ?", created.ID, defaultRole.ID).Take(&relation).Error; err != nil {
		t.Fatalf("find user role: %v", err)
	}
	var version access.Version
	if err := tx.WithContext(ctx).Take(&version, "user_id = ?", created.ID).Error; err != nil {
		t.Fatalf("find access version: %v", err)
	}
	if version.Version != 1 || version.CreatedAt.IsZero() || version.UpdatedAt.IsZero() || !version.CreatedAt.Equal(created.CreatedAt) || !version.UpdatedAt.Equal(created.UpdatedAt) {
		t.Fatalf("access version = %+v user=%+v", version, created)
	}
}

func TestCreateWithRoleMapsUsernameConstraint(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repository := account.NewRepository(tx)
	first := newCreateInput("username", defaultRole.ID)
	if _, err := repository.CreateWithRole(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := newCreateInput("username-second", defaultRole.ID)
	second.Username = strings.ToUpper(first.Username)

	if _, err := repository.CreateWithRole(ctx, second); !errors.Is(err, account.ErrUsernameConflict) {
		t.Fatalf("CreateWithRole() error = %v", err)
	}
}

func TestCreateWithRoleMapsEmailConstraint(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repository := account.NewRepository(tx)
	first := newCreateInput("email", defaultRole.ID)
	if _, err := repository.CreateWithRole(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := newCreateInput("email-second", defaultRole.ID)
	second.Email = first.Email

	if _, err := repository.CreateWithRole(ctx, second); !errors.Is(err, account.ErrEmailConflict) {
		t.Fatalf("CreateWithRole() error = %v", err)
	}
}

func TestCreateWithRoleRejectsInactiveRole(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Model(&role.Role{}).Where("id = ?", defaultRole.ID).Update("is_enabled", yesno.No).Error; err != nil {
		t.Fatal(err)
	}
	repository := account.NewRepository(tx)
	input := newCreateInput("inactive-role", defaultRole.ID)

	if _, err := repository.CreateWithRole(ctx, input); err == nil {
		t.Fatal("inactive role was accepted")
	}
	assertUserCount(t, tx, ctx, input.Username, 0)
}

func TestCreateWithRoleRollsBackAfterRelationshipFailure(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Exec(`
		CREATE FUNCTION pg_temp.reject_user_role_insert() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'forced relationship failure';
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER test_reject_user_role_insert
		BEFORE INSERT ON rbac_user_role
		FOR EACH ROW EXECUTE FUNCTION pg_temp.reject_user_role_insert();`).Error; err != nil {
		t.Fatalf("create rejection trigger: %v", err)
	}
	repository := account.NewRepository(tx)
	input := newCreateInput("relationship-rollback", defaultRole.ID)

	if _, err := repository.CreateWithRole(ctx, input); err == nil {
		t.Fatal("forced relationship failure was ignored")
	}
	assertUserCount(t, tx, ctx, input.Username, 0)
	var orphanVersions int64
	if err := tx.WithContext(ctx).Raw(`
		SELECT count(*)
		FROM rbac_access_version AS access_version
		LEFT JOIN user_account AS app_user ON app_user.id = access_version.user_id
		WHERE app_user.id IS NULL`).Scan(&orphanVersions).Error; err != nil {
		t.Fatal(err)
	}
	if orphanVersions != 0 {
		t.Fatalf("orphan access versions = %d", orphanVersions)
	}
}

func TestFindCredentialUsesExactNormalizedEmail(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repository := account.NewRepository(tx)
	input := newCreateInput("credential", defaultRole.ID)
	created, err := repository.CreateWithRole(ctx, input)
	if err != nil {
		t.Fatal(err)
	}

	credential, err := repository.FindCredentialByEmail(ctx, input.Email)
	if err != nil {
		t.Fatal(err)
	}
	if credential.ID != created.ID || credential.PasswordHash != input.PasswordHash || credential.Email != input.Email {
		t.Fatalf("credential = %+v", credential)
	}
	if _, err := repository.FindCredentialByEmail(ctx, input.Username); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("username unexpectedly resolved as a credential: %v", err)
	}
}

func TestFindCredentialReturnsDisabledStateAndExcludesDeletedUsers(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repository := account.NewRepository(tx)
	input := newCreateInput("credential-state", defaultRole.ID)
	created, err := repository.CreateWithRole(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Model(&account.User{}).Where("id = ?", created.ID).Update("is_enabled", yesno.No).Error; err != nil {
		t.Fatal(err)
	}
	credential, err := repository.FindCredentialByEmail(ctx, input.Email)
	if err != nil {
		t.Fatal(err)
	}
	if credential.IsEnabled != yesno.No {
		t.Fatalf("disabled credential state = %d", credential.IsEnabled)
	}
	if err := tx.WithContext(ctx).Delete(&account.User{}, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.FindCredentialByEmail(ctx, input.Email); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("deleted credential error = %v", err)
	}
}

func TestFindCurrentUserRequiresAnEnabledRole(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repository := account.NewRepository(tx)
	input := newCreateInput("current", defaultRole.ID)
	created, err := repository.CreateWithRole(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	current, err := repository.FindCurrent(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.ID != created.ID || current.Username != input.Username || current.Email != input.Email {
		t.Fatalf("current user = %+v", current)
	}
	if err := tx.WithContext(ctx).Model(&role.Role{}).Where("id = ?", defaultRole.ID).Update("is_enabled", yesno.No).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.FindCurrent(ctx, created.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("disabled-role current user error = %v", err)
	}
}

func TestRepositoryProfilePhoneRoundTripAndKeywordSearch(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repository := account.NewRepository(tx)
	input := newCreateInput("phone", defaultRole.ID)
	created, err := repository.CreateWithRole(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if created.Phone != nil {
		t.Fatalf("new user phone = %v, want nil", created.Phone)
	}
	phone := "+86 138-0000-0000"
	updatedAt := time.Date(2026, 8, 27, 1, 2, 3, 0, time.UTC)
	if err := repository.UpdateProfile(ctx, created.ID, created.Username+"x", &phone, updatedAt); err != nil {
		t.Fatal(err)
	}
	stored, err := repository.FindUser(ctx, created.ID)
	if err != nil || stored.Username != created.Username+"x" || stored.Phone == nil || *stored.Phone != phone || !stored.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("stored=%+v err=%v", stored, err)
	}
	current, err := repository.FindCurrent(ctx, created.ID)
	if err != nil || current.Phone == nil || *current.Phone != phone {
		t.Fatalf("current=%+v err=%v", current, err)
	}
	rows, err := repository.List(ctx, account.ListQuery{Page: 1, PageSize: 20, Keyword: "138-0000"})
	if err != nil || len(rows) != 1 || rows[0].ID != created.ID || rows[0].Phone == nil || *rows[0].Phone != phone {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
}

func TestUpdateProfileMapsPhoneConstraint(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	first := createListedUser(t, tx, ctx, fmt.Sprintf("phone-first%d", time.Now().UnixNano()), fmt.Sprintf("phone-first%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	second := createListedUser(t, tx, ctx, fmt.Sprintf("phone-second%d", time.Now().UnixNano()), fmt.Sprintf("phone-second%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	phone := "+86 138-0000-0000"
	repository := account.NewRepository(tx)
	if err := repository.UpdateProfile(ctx, first.ID, first.Username, &phone, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdateProfile(ctx, second.ID, second.Username, &phone, time.Now().UTC()); !errors.Is(err, account.ErrPhoneConflict) {
		t.Fatalf("UpdateProfile() error = %v", err)
	}
}

func TestCountAndListUsersWithStableFiltersAndRoles(t *testing.T) {
	tx, ctx, _ := openUserTransaction(t)
	repository := account.NewRepository(tx)
	unique := fmt.Sprintf("list%d", time.Now().UnixNano())
	roles := []role.Role{
		{Code: unique + "_b", Name: "B role", IsEnabled: yesno.No},
		{Code: unique + "_a", Name: "A role", IsEnabled: yesno.Yes},
	}
	if err := tx.WithContext(ctx).Create(&roles).Error; err != nil {
		t.Fatal(err)
	}
	roleB, roleA := roles[0], roles[1]
	if err := tx.WithContext(ctx).Model(&role.Role{}).Where("id = ?", roleB.ID).Update("is_enabled", yesno.No).Error; err != nil {
		t.Fatal(err)
	}
	roleB.IsEnabled = yesno.No
	first := createListedUser(t, tx, ctx, unique+"-first", unique+"-FIRST@example.com", yesno.Yes, time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC), roleB.ID, roleA.ID)
	second := createListedUser(t, tx, ctx, unique+"-second", "literal%_\\"+unique+"@example.com", yesno.No, time.Date(2026, 8, 20, 2, 0, 0, 0, time.UTC), roleA.ID)

	query := account.ListQuery{Page: 1, PageSize: 20, Keyword: unique}
	total, err := repository.Count(ctx, query)
	if err != nil || total != 2 {
		t.Fatalf("Count() = %d,%v", total, err)
	}
	items, err := repository.List(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != first.ID || items[1].ID != second.ID {
		t.Fatalf("stable list = %+v", items)
	}
	if got := []string{items[0].Roles[0].Code, items[0].Roles[1].Code}; !reflect.DeepEqual(got, []string{roleA.Code, roleB.Code}) {
		t.Fatalf("role order = %v", got)
	}
	if items[0].Roles[1].IsEnabled != yesno.No {
		t.Fatal("disabled role was hidden or changed")
	}

	for _, keyword := range []string{strings.ToUpper(unique + "-first"), "%", "_", "\\"} {
		found, listErr := repository.List(ctx, account.ListQuery{Page: 1, PageSize: 20, Keyword: keyword})
		if listErr != nil || len(found) != 1 {
			t.Fatalf("List(keyword=%q) = %+v,%v", keyword, found, listErr)
		}
	}

	enabled := yesno.Yes
	items, err = repository.List(ctx, account.ListQuery{Page: 1, PageSize: 20, Keyword: unique, IsEnabled: &enabled})
	if err != nil || len(items) != 1 || items[0].ID != first.ID {
		t.Fatalf("enabled list = %+v,%v", items, err)
	}
	items, err = repository.List(ctx, account.ListQuery{Page: 1, PageSize: 20, Keyword: unique, RoleID: &roleB.ID})
	if err != nil || len(items) != 1 || items[0].ID != first.ID {
		t.Fatalf("role list = %+v,%v", items, err)
	}
	missingRoleID := int64(9223372036854770000)
	items, err = repository.List(ctx, account.ListQuery{Page: 1, PageSize: 20, Keyword: unique, RoleID: &missingRoleID})
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("unknown role list = %#v,%v", items, err)
	}
}

func TestListDetectsInvalidUserRoleData(t *testing.T) {
	tx, ctx, _ := openUserTransaction(t)
	repository := account.NewRepository(tx)
	unique := fmt.Sprintf("invalid_%d", time.Now().UnixNano())
	withoutRole := account.User{Username: unique + "_none", Email: unique + "_none@example.com", PasswordHash: "hash", IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&withoutRole).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.List(ctx, account.ListQuery{Page: 1, PageSize: 20, Keyword: withoutRole.Username}); !errors.Is(err, account.ErrUserDataInvalid) {
		t.Fatalf("missing relation error = %v", err)
	}

	deletedRole := role.Role{Code: unique + "_deleted", Name: "Deleted", IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&deletedRole).Error; err != nil {
		t.Fatal(err)
	}
	withDeletedRole := createListedUser(t, tx, ctx, unique+"_deleted_user", unique+"_deleted@example.com", yesno.Yes, time.Now().UTC(), deletedRole.ID)
	if err := tx.WithContext(ctx).Delete(&deletedRole).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.List(ctx, account.ListQuery{Page: 1, PageSize: 20, Keyword: withDeletedRole.Username}); !errors.Is(err, account.ErrUserDataInvalid) {
		t.Fatalf("deleted role relation error = %v", err)
	}
}

func TestFindRoleOptionsIncludesDisabledRolesInStableOrder(t *testing.T) {
	tx, ctx, _ := openUserTransaction(t)
	repository := account.NewRepository(tx)
	unique := fmt.Sprintf("option_%d", time.Now().UnixNano())
	disabled := role.Role{Code: unique, Name: "Disabled option", IsEnabled: yesno.No}
	if err := tx.WithContext(ctx).Create(&disabled).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Model(&role.Role{}).Where("id = ?", disabled.ID).Update("is_enabled", yesno.No).Error; err != nil {
		t.Fatal(err)
	}
	options, err := repository.FindRoleOptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for index, option := range options {
		if index > 0 && (options[index-1].Code > option.Code || options[index-1].Code == option.Code && options[index-1].ID > option.ID) {
			t.Fatalf("role options are not sorted: %+v", options)
		}
		if option.ID == disabled.ID && option.IsEnabled == yesno.No {
			found = true
		}
	}
	if !found {
		t.Fatal("disabled role option was omitted")
	}
}

func TestRepositoryTransactionRollsBackPriorWrites(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created := createListedUser(t, tx, ctx, fmt.Sprintf("rollback%d", time.Now().UnixNano()), fmt.Sprintf("rollback%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	repository := account.NewRepository(tx)
	forced := errors.New("forced rollback")
	err = repository.Transaction(ctx, func(scoped *account.Repository) error {
		if err := scoped.UpdateStatus(ctx, created.ID, yesno.No, time.Now().UTC()); err != nil {
			return err
		}
		return forced
	})
	if !errors.Is(err, forced) {
		t.Fatalf("Transaction() error = %v", err)
	}
	var stored account.User
	if err := tx.WithContext(ctx).Take(&stored, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.IsEnabled != yesno.Yes {
		t.Fatal("transaction did not roll back status update")
	}
}

func TestRepositoryLockAndCountEffectiveSuperAdmins(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	superRole, err := roleRepository.FindByCode(ctx, role.CodeSuperAdmin)
	if err != nil {
		t.Fatal(err)
	}
	created := createListedUser(t, tx, ctx, fmt.Sprintf("super%d", time.Now().UnixNano()), fmt.Sprintf("super%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), superRole.ID)
	repository := account.NewRepository(tx)
	lockedRole, err := repository.LockSuperAdminRole(ctx)
	if err != nil || lockedRole.ID != superRole.ID {
		t.Fatalf("LockSuperAdminRole() = %+v,%v", lockedRole, err)
	}
	lockedUser, err := repository.LockUser(ctx, created.ID)
	if err != nil || lockedUser.ID != created.ID {
		t.Fatalf("LockUser() = %+v,%v", lockedUser, err)
	}
	effective, err := repository.IsEffectiveSuperAdmin(ctx, created.ID, superRole.ID)
	if err != nil || !effective {
		t.Fatalf("IsEffectiveSuperAdmin() = %v,%v", effective, err)
	}
	activeBinding, err := repository.HasActiveRole(ctx, created.ID, superRole.ID)
	if err != nil || !activeBinding {
		t.Fatalf("HasActiveRole() = %v,%v", activeBinding, err)
	}
	count, err := repository.CountEffectiveSuperAdmins(ctx, superRole.ID)
	if err != nil || count < 1 {
		t.Fatalf("CountEffectiveSuperAdmins() = %d,%v", count, err)
	}
	if err := repository.UpdateStatus(ctx, created.ID, yesno.No, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	effective, err = repository.IsEffectiveSuperAdmin(ctx, created.ID, superRole.ID)
	if err != nil || effective {
		t.Fatalf("disabled IsEffectiveSuperAdmin() = %v,%v", effective, err)
	}
	activeBinding, err = repository.HasActiveRole(ctx, created.ID, superRole.ID)
	if err != nil || !activeBinding {
		t.Fatalf("disabled HasActiveRole() = %v,%v", activeBinding, err)
	}
	roles, err := repository.FindUserRoles(ctx, created.ID)
	if err != nil || len(roles) != 1 || roles[0].RoleID != superRole.ID {
		t.Fatalf("FindUserRoles() = %+v,%v", roles, err)
	}
	allRoles, err := repository.LockRoles(ctx)
	if err != nil || len(allRoles) < 2 {
		t.Fatalf("LockRoles() = %+v,%v", allRoles, err)
	}
}

func TestRepositoryUpdateSoftDeleteCreateUserRolesAndRevoke(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	extraRole := role.Role{Code: fmt.Sprintf("extra_%d", time.Now().UnixNano()), Name: "Extra", IsEnabled: yesno.Yes}
	if err := tx.WithContext(ctx).Create(&extraRole).Error; err != nil {
		t.Fatal(err)
	}
	created := createListedUser(t, tx, ctx, fmt.Sprintf("writes%d", time.Now().UnixNano()), fmt.Sprintf("writes%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	repository := account.NewRepository(tx)
	operationTime := time.Date(2026, 8, 20, 6, 7, 8, 0, time.UTC)
	if err := repository.UpdateProfile(ctx, created.ID, created.Username+"x", nil, operationTime); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdateStatus(ctx, created.ID, yesno.No, operationTime); err != nil {
		t.Fatal(err)
	}
	if err := repository.TouchUser(ctx, created.ID, operationTime.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	lockedRelations, err := repository.LockUserRoles(ctx, created.ID)
	if err != nil || len(lockedRelations) != 1 {
		t.Fatalf("LockUserRoles() = %+v,%v", lockedRelations, err)
	}
	if err := repository.SoftDeleteUserRoleIDs(ctx, []int64{lockedRelations[0].ID}, operationTime); err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateUserRoles(ctx, []role.UserRole{{UserID: created.ID, RoleID: defaultRole.ID}, {UserID: created.ID, RoleID: extraRole.ID}}); err != nil {
		t.Fatal(err)
	}
	activeRelations, err := repository.FindUserRoles(ctx, created.ID)
	if err != nil || len(activeRelations) != 2 {
		t.Fatalf("active relationships = %+v,%v", activeRelations, err)
	}
	session := auth.Session{
		UserID: created.ID, PlatformID: testPlatformID(t, tx, ctx, "admin"), Platform: "admin", DeviceID: "550e8400-e29b-41d4-a716-446655440000",
		RefreshTokenHash: fmt.Sprintf("%064d", created.ID), Version: 1, ClientIP: "127.0.0.1",
		UserAgent: "test", RefreshExpiresAt: operationTime.Add(time.Hour),
	}
	if err := tx.WithContext(ctx).Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.RevokeActiveSessions(ctx, created.ID, operationTime); err != nil {
		t.Fatal(err)
	}
	if err := repository.SoftDeleteUser(ctx, created.ID, operationTime); err != nil {
		t.Fatal(err)
	}
	unscoped, err := repository.LockUserUnscoped(ctx, created.ID)
	if err != nil || !unscoped.DeletedAt.Valid || !unscoped.DeletedAt.Time.Equal(operationTime) || !unscoped.UpdatedAt.Equal(operationTime) {
		t.Fatalf("deleted user = %+v,%v", unscoped, err)
	}
	var storedSession auth.Session
	if err := tx.WithContext(ctx).Take(&storedSession, session.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedSession.RevokedAt == nil || !storedSession.RevokedAt.Equal(operationTime) || !storedSession.UpdatedAt.Equal(operationTime) {
		t.Fatalf("revoked session = %+v", storedSession)
	}
}

func TestRepositoryFindActiveSessionPlatformsReturnsSortedDistinctValues(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created := createListedUser(
		t, tx, ctx, fmt.Sprintf("platforms%d", time.Now().UnixNano()),
		fmt.Sprintf("platforms%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID,
	)
	now := time.Now().UTC().Truncate(time.Microsecond)
	sessions := []auth.Session{
		{UserID: created.ID, Platform: "app", DeviceID: "550e8400-e29b-41d4-a716-446655440001", RefreshTokenHash: fmt.Sprintf("%064d", now.UnixNano()+1), Version: 1, ClientIP: "127.0.0.1", UserAgent: "app", RefreshExpiresAt: now.Add(time.Hour)},
		{UserID: created.ID, Platform: "admin", DeviceID: "550e8400-e29b-41d4-a716-446655440002", RefreshTokenHash: fmt.Sprintf("%064d", now.UnixNano()+2), Version: 1, ClientIP: "127.0.0.1", UserAgent: "admin-a", RefreshExpiresAt: now.Add(time.Hour)},
		{UserID: created.ID, Platform: "admin", DeviceID: "550e8400-e29b-41d4-a716-446655440003", RefreshTokenHash: fmt.Sprintf("%064d", now.UnixNano()+3), Version: 1, ClientIP: "127.0.0.1", UserAgent: "admin-b", RefreshExpiresAt: now.Add(time.Hour)},
		{UserID: created.ID, Platform: "legacy", DeviceID: "550e8400-e29b-41d4-a716-446655440004", RefreshTokenHash: fmt.Sprintf("%064d", now.UnixNano()+4), Version: 1, ClientIP: "127.0.0.1", UserAgent: "revoked", RefreshExpiresAt: now.Add(time.Hour), RevokedAt: &now},
	}
	for index := range sessions {
		sessions[index].PlatformID = testPlatformID(t, tx, ctx, sessions[index].Platform)
	}
	if err := tx.WithContext(ctx).Create(&sessions).Error; err != nil {
		t.Fatal(err)
	}
	platforms, err := account.NewRepository(tx).FindActiveSessionPlatforms(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(platforms, []string{"admin", "app"}) {
		t.Fatalf("active platforms = %v", platforms)
	}
}

func TestRepositoryAccessVersionOperationsAdvanceOnlyTarget(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	first := createListedUser(t, tx, ctx, fmt.Sprintf("versiona%d", time.Now().UnixNano()), fmt.Sprintf("versiona%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	second := createListedUser(t, tx, ctx, fmt.Sprintf("versionb%d", time.Now().UnixNano()), fmt.Sprintf("versionb%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	repository := account.NewRepository(tx)
	candidate, err := repository.FindAccessVersion(ctx, first.ID)
	if err != nil || candidate.UserID != first.ID || candidate.Version != 1 {
		t.Fatalf("FindAccessVersion() = %+v,%v", candidate, err)
	}
	err = repository.Transaction(ctx, func(scoped *account.Repository) error {
		locked, lockErr := scoped.LockAccessVersion(ctx, first.ID)
		if lockErr != nil || locked != candidate.Version {
			return fmt.Errorf("LockAccessVersion() = %d,%v", locked, lockErr)
		}
		advanced, incrementErr := scoped.IncrementAccessVersion(ctx, first.ID, time.Now().UTC().Truncate(time.Microsecond))
		if incrementErr != nil || advanced != 2 {
			return fmt.Errorf("IncrementAccessVersion() = %d,%v", advanced, incrementErr)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := readUserAccessVersion(t, tx, ctx, first.ID); got != 2 {
		t.Fatalf("first access version = %d", got)
	}
	if got := readUserAccessVersion(t, tx, ctx, second.ID); got != 1 {
		t.Fatalf("second access version = %d", got)
	}
}

func TestRepositoryUpdateProfileMapsActiveUsernameConstraint(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	first := createListedUser(t, tx, ctx, fmt.Sprintf("conflict%d", time.Now().UnixNano()), fmt.Sprintf("conflict%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	second := createListedUser(t, tx, ctx, fmt.Sprintf("other%d", time.Now().UnixNano()), fmt.Sprintf("other%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	if err := account.NewRepository(tx).UpdateProfile(ctx, second.ID, strings.ToUpper(first.Username), nil, time.Now().UTC()); !errors.Is(err, account.ErrUsernameConflict) {
		t.Fatalf("UpdateProfile() error = %v", err)
	}
}

func TestRepositoryTransactionRollsBackStatusRolesAndSessionsAfterForcedFailure(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created := createListedUser(t, tx, ctx, fmt.Sprintf("forced%d", time.Now().UnixNano()), fmt.Sprintf("forced%d@example.com", time.Now().UnixNano()), yesno.Yes, time.Now().UTC(), defaultRole.ID)
	var relation role.UserRole
	if err := tx.WithContext(ctx).Where("user_id = ? AND role_id = ?", created.ID, defaultRole.ID).Take(&relation).Error; err != nil {
		t.Fatal(err)
	}
	session := auth.Session{
		UserID: created.ID, PlatformID: testPlatformID(t, tx, ctx, "admin"), Platform: "admin", DeviceID: "550e8400-e29b-41d4-a716-446655440000",
		RefreshTokenHash: fmt.Sprintf("%064d", created.ID+10000), Version: 1, ClientIP: "127.0.0.1",
		UserAgent: "rollback", RefreshExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	if err := tx.WithContext(ctx).Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Exec(`
		CREATE FUNCTION pg_temp.reject_user_session_revoke() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'forced session revoke failure';
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER test_reject_user_session_revoke
		BEFORE UPDATE ON user_session
		FOR EACH ROW EXECUTE FUNCTION pg_temp.reject_user_session_revoke();`).Error; err != nil {
		t.Fatal(err)
	}
	repository := account.NewRepository(tx)
	operationTime := time.Now().UTC().Truncate(time.Microsecond)
	err = repository.Transaction(ctx, func(scoped *account.Repository) error {
		if err := scoped.UpdateStatus(ctx, created.ID, yesno.No, operationTime); err != nil {
			return err
		}
		if err := scoped.SoftDeleteUserRoleIDs(ctx, []int64{relation.ID}, operationTime); err != nil {
			return err
		}
		if _, err := scoped.LockAccessVersion(ctx, created.ID); err != nil {
			return err
		}
		if _, err := scoped.IncrementAccessVersion(ctx, created.ID, operationTime); err != nil {
			return err
		}
		return scoped.RevokeActiveSessions(ctx, created.ID, operationTime)
	})
	if err == nil {
		t.Fatal("forced session failure was ignored")
	}
	var storedUser account.User
	if err := tx.WithContext(ctx).Take(&storedUser, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	var storedRelation role.UserRole
	if err := tx.WithContext(ctx).Take(&storedRelation, relation.ID).Error; err != nil {
		t.Fatal(err)
	}
	var storedSession auth.Session
	if err := tx.WithContext(ctx).Take(&storedSession, session.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedUser.IsEnabled != yesno.Yes || storedRelation.DeletedAt.Valid || storedSession.RevokedAt != nil {
		t.Fatalf("partial write survived rollback: user=%+v relation=%+v session=%+v", storedUser, storedRelation, storedSession)
	}
	if got := readUserAccessVersion(t, tx, ctx, created.ID); got != 1 {
		t.Fatalf("access version survived rollback: %d", got)
	}
}

func TestRepositoryLockSuperAdminRoleSerializesMutationEntry(t *testing.T) {
	db, ctx, _ := openUserDatabase(t)
	firstTx := db.WithContext(ctx).Begin()
	if firstTx.Error != nil {
		t.Fatal(firstTx.Error)
	}
	t.Cleanup(func() { _ = firstTx.Rollback().Error })
	if _, err := account.NewRepository(firstTx).LockSuperAdminRole(ctx); err != nil {
		t.Fatal(err)
	}

	secondTx := db.WithContext(ctx).Begin()
	if secondTx.Error != nil {
		t.Fatal(secondTx.Error)
	}
	t.Cleanup(func() { _ = secondTx.Rollback().Error })
	waitCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	if _, err := account.NewRepository(secondTx).LockSuperAdminRole(waitCtx); err == nil {
		t.Fatal("second mutation acquired the common first lock before the first transaction completed")
	}
}

func createListedUser(t *testing.T, tx *gorm.DB, ctx context.Context, username, email string, enabled yesno.Value, createdAt time.Time, roleIDs ...int64) account.User {
	t.Helper()
	created := account.User{Username: username, Email: email, PasswordHash: "hash", IsEnabled: enabled, CreatedAt: createdAt, UpdatedAt: createdAt}
	if err := tx.WithContext(ctx).Create(&created).Error; err != nil {
		t.Fatal(err)
	}
	if enabled == yesno.No {
		if err := tx.WithContext(ctx).Model(&account.User{}).Where("id = ?", created.ID).Update("is_enabled", yesno.No).Error; err != nil {
			t.Fatal(err)
		}
		created.IsEnabled = yesno.No
	}
	if err := tx.WithContext(ctx).Create(&access.Version{
		UserID: created.ID, Version: 1, CreatedAt: created.CreatedAt, UpdatedAt: created.UpdatedAt,
	}).Error; err != nil {
		t.Fatal(err)
	}
	for _, roleID := range roleIDs {
		if err := tx.WithContext(ctx).Create(&role.UserRole{UserID: created.ID, RoleID: roleID}).Error; err != nil {
			t.Fatal(err)
		}
	}
	return created
}

func openUserTransaction(t *testing.T) (*gorm.DB, context.Context, *role.Repository) {
	t.Helper()
	db, ctx, _ := openUserDatabase(t)
	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	return tx, ctx, role.NewRepository(tx)
}

func openUserDatabase(t *testing.T) (*gorm.DB, context.Context, *role.Repository) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration test")
	}
	if err := godotenv.Load("../../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("load server .env: %v", err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatalf("load worker config: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	root, err := database.Open(ctx, settings.PostgresDSN)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	schema := fmt.Sprintf("test_user_%d", time.Now().UnixNano())
	if err := root.GORM.WithContext(ctx).Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	pgxConfig, err := pgx.ParseConfig(settings.PostgresDSN)
	if err != nil {
		t.Fatal(err)
	}
	pgxConfig.RuntimeParams["search_path"] = schema
	sqlDB := stdlib.OpenDB(*pgxConfig)
	if err := sqlDB.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = root.GORM.WithContext(cleanupCtx).Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error
		_ = root.Close()
	})
	if err := database.AutoMigrate(ctx, db, &account.User{}, &profile.Profile{}, &role.Role{}, &role.UserRole{}, &authplatform.Platform{}, &auth.Session{}, &access.Version{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := authplatform.EnsureSchema(ctx, db); err != nil {
		t.Fatalf("Ensure platform schema: %v", err)
	}
	for _, statement := range []string{
		`CREATE UNIQUE INDEX ux_user_account_username_active ON user_account (lower(username)) WHERE deleted_at IS NULL`,
		`CREATE UNIQUE INDEX ux_user_account_email_active ON user_account (lower(email)) WHERE deleted_at IS NULL`,
		`CREATE UNIQUE INDEX ux_user_account_phone_active ON user_account (phone) WHERE phone IS NOT NULL AND deleted_at IS NULL`,
	} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("create account test index: %v", err)
		}
	}
	for _, code := range []string{"app", "canvas", "legacy"} {
		value := authplatform.Platform{Code: code, Name: code, PolicyVersion: 1, AccessTTLSeconds: 900, RefreshTTLSeconds: 1209600, SessionCacheTTLSeconds: 1800, AccessCacheTTLSeconds: 1800, BindDevice: yesno.No, BindIP: yesno.No, MaxSessions: 0, AllowRegister: yesno.Yes, IsEnabled: yesno.Yes, IsBuiltin: yesno.No}
		if err := db.WithContext(ctx).Create(&value).Error; err != nil {
			t.Fatalf("create test platform %s: %v", code, err)
		}
	}
	if err := role.EnsureSchema(ctx, db); err != nil {
		t.Fatalf("Ensure role schema: %v", err)
	}
	if err := access.EnsureSchema(ctx, db); err != nil {
		t.Fatalf("Ensure access schema: %v", err)
	}
	roleRepository := role.NewRepository(db)
	if err := role.NewService(roleRepository, nil).EnsureSystemRoles(ctx); err != nil {
		t.Fatalf("EnsureSystemRoles: %v", err)
	}
	return db, ctx, roleRepository
}

func newCreateInput(prefix string, roleID int64) account.CreateInput {
	unique := time.Now().UnixNano()
	return account.CreateInput{
		Username:     fmt.Sprintf("%s-%d", prefix, unique),
		Email:        fmt.Sprintf("%s-%d@example.com", prefix, unique),
		PasswordHash: "$2a$10$placeholder",
		RoleID:       roleID,
	}
}

func testPlatformID(t *testing.T, db *gorm.DB, ctx context.Context, code string) int64 {
	t.Helper()
	var value authplatform.Platform
	if err := db.WithContext(ctx).Where("code = ?", code).Take(&value).Error; err != nil {
		t.Fatalf("find test platform %s: %v", code, err)
	}
	return value.ID
}

func TestPersonalProfileRepositoryPersistsAndReadsBirthdayAndGender(t *testing.T) {
	db, ctx, roleRepository := openUserDatabase(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	accountRepository := account.NewRepository(db)
	created, err := accountRepository.CreateWithRole(ctx, newCreateInput("profile", defaultRole.ID))
	if err != nil {
		t.Fatal(err)
	}
	birthday := time.Date(2000, 1, 2, 0, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 8, 28, 1, 2, 3, 0, time.UTC)
	profileRepository := profile.NewRepository(db)
	updated, err := profileRepository.Update(ctx, created.ID, "profile-user", nil, &birthday, 2, updatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Username != "profile-user" || updated.Gender != 2 || updated.Birthday == nil || !updated.Birthday.Equal(birthday) {
		t.Fatalf("updated profile=%+v", updated)
	}
	read, err := profileRepository.Find(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Email != created.Email || read.Gender != 2 || read.Birthday == nil || read.Birthday.Format("2006-01-02") != "2000-01-02" {
		t.Fatalf("read profile=%+v", read)
	}
}

func TestChangePasswordAndRevokeSessionsUpdatesHashAndAllPlatforms(t *testing.T) {
	db, ctx, roleRepository := openUserDatabase(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repository := account.NewRepository(db)
	created, err := repository.CreateWithRole(ctx, newCreateInput("password", defaultRole.ID))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 28, 2, 3, 4, 0, time.UTC)
	for _, platform := range []string{"admin", "canvas"} {
		if err := db.WithContext(ctx).Create(&auth.Session{UserID: created.ID, PlatformID: testPlatformID(t, db, ctx, platform), Platform: platform, DeviceID: "device-" + platform, RefreshTokenHash: fmt.Sprintf("%064d", len(platform)), Version: 1, ClientIP: "127.0.0.1", UserAgent: "test", RefreshExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	revoked, err := repository.ChangePasswordAndRevokeSessions(ctx, created.ID, "new-hash", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(revoked) != 2 {
		t.Fatalf("revoked sessions=%+v", revoked)
	}
	credential, err := repository.FindCredentialByID(ctx, created.ID)
	if err != nil || credential.PasswordHash != "new-hash" {
		t.Fatalf("credential=%+v err=%v", credential, err)
	}
	var active int64
	if err := db.WithContext(ctx).Table("user_session").Where("user_id = ? AND revoked_at IS NULL", created.ID).Count(&active).Error; err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatalf("active sessions=%d", active)
	}
}

func assertUserCount(t *testing.T, tx *gorm.DB, ctx context.Context, username string, want int64) {
	t.Helper()
	var count int64
	if err := tx.WithContext(ctx).Model(&account.User{}).Where("username = ?", username).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("user count = %d, want %d", count, want)
	}
}
