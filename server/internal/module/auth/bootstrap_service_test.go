package auth

import (
	"context"
	"errors"
	"testing"

	"admin/server/internal/module/role"
	"admin/server/internal/module/user"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
)

func TestBootstrapAdminCreatesUserWithSuperAdminRole(t *testing.T) {
	var stored user.CreateInput
	users := &fakeBootstrapUserStore{createFn: func(_ context.Context, input user.CreateInput) (user.User, error) {
		stored = input
		return user.User{ID: 10, Username: input.Username, Email: input.Email}, nil
	}}
	roles := &fakeBootstrapRoleStore{found: role.Role{ID: 8, Code: role.CodeSuperAdmin, IsEnabled: yesno.Yes}}
	service := NewBootstrapService(users, roles)

	created, err := service.Create(context.Background(), BootstrapAdminInput{Username: "admin", Email: "ADMIN@example.com", Password: "password"})
	if err != nil {
		t.Fatal(err)
	}
	if stored.RoleID != 8 || stored.Email != "admin@example.com" || stored.PasswordHash == "" {
		t.Fatalf("CreateWithRole input = %+v", stored)
	}
	if err := VerifyPassword(stored.PasswordHash, "password"); err != nil {
		t.Fatalf("stored password hash does not verify: %v", err)
	}
	if created.UserID != 10 || created.Username != "admin" {
		t.Fatalf("created = %+v", created)
	}
}

func TestBootstrapAdminRejectsExistingActiveSuperAdmin(t *testing.T) {
	roles := &fakeBootstrapRoleStore{found: role.Role{ID: 8}, hasActive: true}
	service := NewBootstrapService(&fakeBootstrapUserStore{}, roles)
	_, err := service.Create(context.Background(), BootstrapAdminInput{Username: "admin", Email: "admin@example.com", Password: "password"})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeConflict || appErr.MessageKey != i18n.KeySuperAdminExists {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestBootstrapAdminRejectsMissingOrDisabledSystemRole(t *testing.T) {
	roles := &fakeBootstrapRoleStore{findErr: errors.New("super_admin role missing")}
	service := NewBootstrapService(&fakeBootstrapUserStore{}, roles)
	_, err := service.Create(context.Background(), BootstrapAdminInput{Username: "admin", Email: "admin@example.com", Password: "password"})
	if appErrorCode(err) != apperror.CodeInternal {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestBootstrapAdminUsesRegistrationValidationAndConflictMapping(t *testing.T) {
	roles := &fakeBootstrapRoleStore{found: role.Role{ID: 8}}
	invalidService := NewBootstrapService(&fakeBootstrapUserStore{}, roles)
	if _, err := invalidService.Create(context.Background(), BootstrapAdminInput{Username: "bad name", Email: "invalid", Password: "short"}); appErrorCode(err) != apperror.CodeInvalidRequest {
		t.Fatalf("invalid input error = %v", err)
	}

	for _, conflict := range []error{user.ErrUsernameConflict, user.ErrEmailConflict} {
		users := &fakeBootstrapUserStore{createFn: func(context.Context, user.CreateInput) (user.User, error) {
			return user.User{}, conflict
		}}
		service := NewBootstrapService(users, roles)
		if _, err := service.Create(context.Background(), BootstrapAdminInput{Username: "admin", Email: "admin@example.com", Password: "password"}); appErrorCode(err) != apperror.CodeConflict {
			t.Fatalf("conflict %v mapped to %v", conflict, err)
		}
	}
}

type fakeBootstrapUserStore struct {
	createFn func(context.Context, user.CreateInput) (user.User, error)
}

func (f *fakeBootstrapUserStore) CreateWithRole(ctx context.Context, input user.CreateInput) (user.User, error) {
	if f.createFn == nil {
		return user.User{}, errors.New("unexpected CreateWithRole call")
	}
	return f.createFn(ctx, input)
}

type fakeBootstrapRoleStore struct {
	found     role.Role
	findErr   error
	hasActive bool
	hasErr    error
}

func (f *fakeBootstrapRoleStore) FindByCode(context.Context, string) (role.Role, error) {
	return f.found, f.findErr
}

func (f *fakeBootstrapRoleStore) HasActiveUserWithRole(context.Context, int64) (bool, error) {
	return f.hasActive, f.hasErr
}
