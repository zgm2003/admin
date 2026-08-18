package user

import (
	"context"
	"errors"
	"fmt"

	"admin/server/internal/module/role"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrUsernameConflict = errors.New("active username already exists")
	ErrEmailConflict    = errors.New("active email already exists")
)

type CreateInput struct {
	Username     string
	Email        string
	PasswordHash string
	RoleID       int64
}

type Credential struct {
	ID           int64
	Username     string
	Email        string
	PasswordHash string
	IsEnabled    yesno.Value
}

type Current struct {
	ID       int64
	Username string
	Email    string
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateWithRole(ctx context.Context, input CreateInput) (User, error) {
	var created User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var effectiveRole role.Role
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND is_enabled = ?", input.RoleID, yesno.Yes).
			Take(&effectiveRole).Error; err != nil {
			return fmt.Errorf("lock enabled role: %w", err)
		}

		created = User{
			Username:     input.Username,
			Email:        input.Email,
			PasswordHash: input.PasswordHash,
			IsEnabled:    yesno.Yes,
		}
		if err := tx.Create(&created).Error; err != nil {
			return mapCreateError(err)
		}
		userRole := role.UserRole{UserID: created.ID, RoleID: effectiveRole.ID}
		if err := tx.Create(&userRole).Error; err != nil {
			return fmt.Errorf("create user role relationship: %w", err)
		}
		return nil
	})
	if err != nil {
		return User{}, err
	}
	return created, nil
}

func (r *Repository) FindCredentialByUsername(ctx context.Context, username string) (Credential, error) {
	var credential Credential
	if err := r.db.WithContext(ctx).
		Model(&User{}).
		Select("id", "username", "email", "password_hash", "is_enabled").
		Where("lower(username) = lower(?)", username).
		Take(&credential).Error; err != nil {
		return Credential{}, fmt.Errorf("find user credential: %w", err)
	}
	return credential, nil
}

func (r *Repository) FindCurrent(ctx context.Context, userID int64) (Current, error) {
	var current Current
	result := r.db.WithContext(ctx).Raw(`
		SELECT app_user.id, app_user.username, app_user.email
		FROM sys_user AS app_user
		WHERE app_user.id = ?
		  AND app_user.deleted_at IS NULL
		  AND app_user.is_enabled = ?
		  AND EXISTS (
			SELECT 1
			FROM sys_user_role AS user_role
			JOIN sys_role AS app_role
			  ON app_role.id = user_role.role_id
			 AND app_role.deleted_at IS NULL
			 AND app_role.is_enabled = ?
			WHERE user_role.user_id = app_user.id
			  AND user_role.deleted_at IS NULL
		  )`, userID, yesno.Yes, yesno.Yes).Scan(&current)
	if result.Error != nil {
		return Current{}, fmt.Errorf("find current user: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return Current{}, fmt.Errorf("find current user: %w", gorm.ErrRecordNotFound)
	}
	return current, nil
}

func mapCreateError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.ConstraintName {
		case "ux_sys_user_username_active":
			return ErrUsernameConflict
		case "ux_sys_user_email_active":
			return ErrEmailConflict
		}
	}
	return fmt.Errorf("create user: %w", err)
}
