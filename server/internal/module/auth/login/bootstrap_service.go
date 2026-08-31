package auth

import (
	"context"

	"admin/server/internal/module/permission/role"
	user "admin/server/internal/module/user/account"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
)

type BootstrapAdminInput struct {
	Username string
	Email    string
	Password string
}

type bootstrapUserStore interface {
	CreateWithRole(context.Context, user.CreateInput) (user.User, error)
}

type bootstrapRoleStore interface {
	FindByCode(context.Context, string) (role.Role, error)
	HasActiveUserWithRole(context.Context, int64) (bool, error)
}

type BootstrapService struct {
	users bootstrapUserStore
	roles bootstrapRoleStore
}

func NewBootstrapService(users bootstrapUserStore, roles bootstrapRoleStore) *BootstrapService {
	return &BootstrapService{users: users, roles: roles}
}

func (s *BootstrapService) Create(ctx context.Context, input BootstrapAdminInput) (Registered, error) {
	adminRole, err := s.roles.FindByCode(ctx, role.CodeSuperAdmin)
	if err != nil {
		return Registered{}, apperror.Internal(err)
	}
	exists, err := s.roles.HasActiveUserWithRole(ctx, adminRole.ID)
	if err != nil {
		return Registered{}, apperror.Internal(err)
	}
	if exists {
		return Registered{}, apperror.Conflict(i18n.KeySuperAdminExists, nil, nil)
	}
	normalized, err := validateAccountInput(input.Username, input.Email, input.Password, input.Password)
	if err != nil {
		return Registered{}, err
	}
	passwordHash, err := HashPassword(normalized.Password)
	if err != nil {
		return Registered{}, apperror.Internal(err)
	}
	created, err := s.users.CreateWithRole(ctx, user.CreateInput{
		Username:     normalized.Username,
		Email:        normalized.Email,
		PasswordHash: passwordHash,
		RoleID:       adminRole.ID,
	})
	if err != nil {
		return Registered{}, mapUserCreateError(err)
	}
	return Registered{UserID: created.ID, Username: created.Username, Email: created.Email}, nil
}
