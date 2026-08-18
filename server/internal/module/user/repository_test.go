package user_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/auth"
	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestCreateWithRolePersistsUserAndRoleAtomically(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repository := user.NewRepository(tx)
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
}

func TestCreateWithRoleMapsUsernameConstraint(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repository := user.NewRepository(tx)
	first := newCreateInput("username", defaultRole.ID)
	if _, err := repository.CreateWithRole(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := newCreateInput("username-second", defaultRole.ID)
	second.Username = strings.ToUpper(first.Username)

	if _, err := repository.CreateWithRole(ctx, second); !errors.Is(err, user.ErrUsernameConflict) {
		t.Fatalf("CreateWithRole() error = %v", err)
	}
}

func TestCreateWithRoleMapsEmailConstraint(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repository := user.NewRepository(tx)
	first := newCreateInput("email", defaultRole.ID)
	if _, err := repository.CreateWithRole(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := newCreateInput("email-second", defaultRole.ID)
	second.Email = first.Email

	if _, err := repository.CreateWithRole(ctx, second); !errors.Is(err, user.ErrEmailConflict) {
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
	repository := user.NewRepository(tx)
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
		BEFORE INSERT ON sys_user_role
		FOR EACH ROW EXECUTE FUNCTION pg_temp.reject_user_role_insert();`).Error; err != nil {
		t.Fatalf("create rejection trigger: %v", err)
	}
	repository := user.NewRepository(tx)
	input := newCreateInput("relationship-rollback", defaultRole.ID)

	if _, err := repository.CreateWithRole(ctx, input); err == nil {
		t.Fatal("forced relationship failure was ignored")
	}
	assertUserCount(t, tx, ctx, input.Username, 0)
}

func TestFindCredentialUsesCaseInsensitiveUsername(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repository := user.NewRepository(tx)
	input := newCreateInput("credential", defaultRole.ID)
	created, err := repository.CreateWithRole(ctx, input)
	if err != nil {
		t.Fatal(err)
	}

	credential, err := repository.FindCredentialByUsername(ctx, strings.ToUpper(input.Username))
	if err != nil {
		t.Fatal(err)
	}
	if credential.ID != created.ID || credential.PasswordHash != input.PasswordHash || credential.Email != input.Email {
		t.Fatalf("credential = %+v", credential)
	}
}

func TestFindCredentialReturnsDisabledStateAndExcludesDeletedUsers(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repository := user.NewRepository(tx)
	input := newCreateInput("credential-state", defaultRole.ID)
	created, err := repository.CreateWithRole(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.WithContext(ctx).Model(&user.User{}).Where("id = ?", created.ID).Update("is_enabled", yesno.No).Error; err != nil {
		t.Fatal(err)
	}
	credential, err := repository.FindCredentialByUsername(ctx, input.Username)
	if err != nil {
		t.Fatal(err)
	}
	if credential.IsEnabled != yesno.No {
		t.Fatalf("disabled credential state = %d", credential.IsEnabled)
	}
	if err := tx.WithContext(ctx).Delete(&user.User{}, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.FindCredentialByUsername(ctx, input.Username); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("deleted credential error = %v", err)
	}
}

func TestFindCurrentUserRequiresAnEnabledRole(t *testing.T) {
	tx, ctx, roleRepository := openUserTransaction(t)
	defaultRole, err := roleRepository.FindDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repository := user.NewRepository(tx)
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

func openUserTransaction(t *testing.T) (*gorm.DB, context.Context, *role.Repository) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration test")
	}
	if err := godotenv.Load("../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("load server .env: %v", err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatalf("load worker config: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	connection, err := database.Open(ctx, settings.PostgresDSN)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	if err := database.AutoMigrate(ctx, connection.GORM, &user.User{}, &role.Role{}, &role.UserRole{}, &auth.Session{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := auth.EnsureSchema(ctx, connection.GORM); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	tx := connection.GORM.WithContext(ctx).Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	roleRepository := role.NewRepository(tx)
	if err := roleRepository.EnsureSystemRoles(ctx); err != nil {
		t.Fatalf("EnsureSystemRoles: %v", err)
	}
	return tx, ctx, roleRepository
}

func newCreateInput(prefix string, roleID int64) user.CreateInput {
	unique := time.Now().UnixNano()
	return user.CreateInput{
		Username:     fmt.Sprintf("%s-%d", prefix, unique),
		Email:        fmt.Sprintf("%s-%d@example.com", prefix, unique),
		PasswordHash: "$2a$10$placeholder",
		RoleID:       roleID,
	}
}

func assertUserCount(t *testing.T, tx *gorm.DB, ctx context.Context, username string, want int64) {
	t.Helper()
	var count int64
	if err := tx.WithContext(ctx).Model(&user.User{}).Where("username = ?", username).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("user count = %d, want %d", count, want)
	}
}
